package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/microsoft/TypeScript/tsc/internal/contentmapper/asp"
)

type projectConfig struct {
	Version         int      `json:"version"`
	Root            string   `json:"root"`
	Entry           string   `json:"entry"`
	Scan            string   `json:"scan"`
	Types           []string `json:"types"`
	DependencyRules string   `json:"dependencyRules"`
	SourceTypes     string   `json:"sourceTypes"`
	LooseVariables  bool     `json:"looseVariables"`
}

func readProject(file string) (projectConfig, error) {
	var p projectConfig
	data, err := os.ReadFile(file)
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	if p.Version != 1 || p.Root == "" {
		return p, fmt.Errorf("project requires version 1 and root")
	}
	base, err := filepath.Abs(filepath.Dir(file))
	if err != nil {
		return p, err
	}
	resolve := func(value string) string {
		if value == "" {
			return ""
		}
		if filepath.IsAbs(value) {
			return filepath.Clean(value)
		}
		return filepath.Join(base, value)
	}
	p.Root, p.Entry, p.DependencyRules, p.SourceTypes = resolve(p.Root), resolve(p.Entry), resolve(p.DependencyRules), resolve(p.SourceTypes)
	p.Scan = resolve(p.Scan)
	for i := range p.Types {
		p.Types[i] = resolve(p.Types[i])
	}
	return p, nil
}

func discoverProject(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		file := filepath.Join(dir, "asp-check.json")
		if info, err := os.Stat(file); err == nil {
			if info.IsDir() {
				return "", fmt.Errorf("project config is a directory: %s", file)
			}
			return file, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

type batchDiagnostic struct {
	asp.Diagnostic
	Entries []string `json:"entries"`
}
type batchResult struct {
	Version     int               `json:"version"`
	Pages       []asp.Report      `json:"pages"`
	Diagnostics []batchDiagnostic `json:"diagnostics"`
	Clean       int               `json:"clean"`
	WithErrors  int               `json:"withErrors"`
	Failed      int               `json:"failed"`
	Errors      int               `json:"errors"`
	Warnings    int               `json:"warnings"`
	Seconds     float64           `json:"seconds"`
}

func runScan(out io.Writer, directory, root string, types []string, rules, sourceTypes string, loose bool, jobs int, jsonOutput, errorsOnly bool, warnShared bool) int {
	started := time.Now()
	absolute, err := filepath.Abs(directory)
	if err != nil {
		fmt.Fprintf(out, "ASP: %v\n", err)
		return 2
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		fmt.Fprintln(out, "ASP: --scan must name an existing directory")
		return 2
	}
	var files []string
	err = filepath.WalkDir(absolute, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(file), ".asp") {
			files = append(files, file)
		}
		return nil
	})
	if err != nil || len(files) == 0 {
		fmt.Fprintf(out, "ASP: scan failed or no ASP pages found: %v\n", err)
		return 2
	}
	sort.Strings(files)
	r := batchResult{Version: 1, Pages: make([]asp.Report, len(files)), Diagnostics: []batchDiagnostic{}}
	work := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < jobs; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range work {
				r.Pages[index] = asp.CheckReportWithOverlay(context.Background(), files[index], root, types, rules, sourceTypes, loose, "", warnShared)
			}
		}()
	}
	for i := range files {
		work <- i
	}
	close(work)
	wg.Wait()
	unique := map[asp.Diagnostic]int{}
	for _, page := range r.Pages {
		if page.Error != "" {
			r.Failed++
			continue
		}
		errors := false
		for _, d := range page.Diagnostics {
			if !d.Warning {
				errors = true
			}
			index, ok := unique[d]
			if !ok {
				index = len(r.Diagnostics)
				unique[d] = index
				r.Diagnostics = append(r.Diagnostics, batchDiagnostic{Diagnostic: d, Entries: []string{}})
				if d.Warning {
					r.Warnings++
				} else {
					r.Errors++
				}
			}
			r.Diagnostics[index].Entries = append(r.Diagnostics[index].Entries, page.Entry)
		}
		if errors {
			r.WithErrors++
		} else {
			r.Clean++
		}
	}
	sort.SliceStable(r.Diagnostics, func(i, j int) bool {
		a, b := r.Diagnostics[i], r.Diagnostics[j]
		if a.Warning != b.Warning {
			return !a.Warning
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
	r.Seconds = time.Since(started).Seconds()
	if errorsOnly {
		filtered := []batchDiagnostic{}
		for _, d := range r.Diagnostics {
			if !d.Warning {
				filtered = append(filtered, d)
			}
		}
		r.Diagnostics = filtered
		for i := range r.Pages {
			filtered := []asp.Diagnostic{}
			for _, d := range r.Pages[i].Diagnostics {
				if !d.Warning {
					filtered = append(filtered, d)
				}
			}
			r.Pages[i].Diagnostics = filtered
		}
	}
	if jsonOutput {
		if err := json.NewEncoder(out).Encode(r); err != nil {
			return 2
		}
	} else {
		for _, d := range r.Diagnostics {
			severity := "error"
			if d.Warning {
				severity = "warning"
			}
			fmt.Fprintf(out, "%s(%d,%d): %s TS%d: %s [affects %d pages]\n", d.File, d.Line, d.Column, severity, d.Code, d.Message, len(d.Entries))
		}
		for _, p := range r.Pages {
			if p.Error != "" {
				fmt.Fprintf(out, "ASP: %s: %s\n", p.Entry, p.Error)
			}
		}
		fmt.Fprintf(out, "Scanned %d pages: %d clean, %d with errors, %d failed; unique diagnostics: %d errors, %d warnings; %.2fs.\n", len(files), r.Clean, r.WithErrors, r.Failed, r.Errors, r.Warnings, r.Seconds)
	}
	if r.Failed > 0 {
		return 2
	}
	if r.Errors > 0 {
		return 1
	}
	return 0
}
