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

// Batch emission reuses immutable input snapshots/scans, never compiler symbols.
// Each entry retains isolated full semantic checking unless erase-only is explicit.
func runEraseScan(out io.Writer, directory, root string, types []string, outDir string, eraseOnly, write bool, jobs int, jsonOutput bool, emitOnError bool, errorsOnly ...bool) int {
	hideWarnings := len(errorsOnly) > 0 && errorsOnly[0]
	return runEraseScanWithPolicy(out, directory, root, types, outDir, eraseOnly, write, jobs, jsonOutput, emitOnError, hideWarnings, buildCheckPolicy{})
}

type buildCheckPolicy struct {
	rules, sourceTypes string
	loose, warnShared  bool
}

func runEraseScanWithPolicy(out io.Writer, directory, root string, types []string, outDir string, eraseOnly, write bool, jobs int, jsonOutput bool, emitOnError, errorsOnly bool, policy buildCheckPolicy) int {
	started := time.Now()
	dir, err := filepath.Abs(directory)
	if err != nil {
		fmt.Fprintf(out, "ASP: %v\n", err)
		return 2
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		fmt.Fprintln(out, "ASP: emission scan requires a directory")
		return 2
	}
	var files []string
	var passthrough []string
	outputRoot := ""
	if outDir != "" {
		outputRoot, err = filepath.Abs(outDir)
		if err != nil {
			fmt.Fprintf(out, "ASP: %v\n", err)
			return 2
		}
		if strings.EqualFold(outputRoot, dir) {
			fmt.Fprintln(out, "ASP: mixed output directory must differ from source directory")
			return 2
		}
	}
	err = filepath.WalkDir(dir, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if outputRoot != "" && strings.EqualFold(file, outputRoot) {
				return filepath.SkipDir
			}
			if file != dir && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("mixed scan does not follow symlinks: %s", file)
		}
		if strings.HasPrefix(entry.Name(), ".") || strings.HasSuffix(strings.ToLower(file), ".d.ts") {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(file), ".asp.ts") || strings.HasSuffix(strings.ToLower(file), ".inc.ts") {
			files = append(files, file)
		} else if outputRoot != "" && !strings.HasSuffix(strings.ToLower(file), ".ts") {
			passthrough = append(passthrough, file)
		}
		return nil
	})
	if err != nil || len(files)+len(passthrough) == 0 {
		fmt.Fprintf(out, "ASP: emission scan failed or no deployable files found: %v\n", err)
		return 2
	}
	if write && len(passthrough) > 0 && outDir == "" {
		fmt.Fprintln(out, "ASP: mixed-tree emission requires --out-dir to copy existing files safely")
		return 2
	}
	sort.Strings(files)
	sort.Strings(passthrough)
	// Preflight every destination before validating or writing. Case folding also
	// detects Windows deployment collisions when builds are run under Linux.
	destinations := map[string]string{}
	manifest := []struct {
		source, dest string
		copy         bool
	}{}
	for _, copyFile := range []bool{false, true} {
		inputs := files
		if copyFile {
			inputs = passthrough
		}
		for _, file := range inputs {
			rel, err := filepath.Rel(dir, file)
			if err != nil {
				fmt.Fprintf(out, "ASP: %v\n", err)
				return 2
			}
			if !copyFile {
				rel = rel[:len(rel)-3]
			}
			dest := filepath.Join(outputRoot, rel)
			if outputRoot == "" {
				dest = file[:len(file)-3]
				if copyFile {
					dest = file
				}
			}
			key := strings.ToLower(filepath.Clean(dest))
			if previous, ok := destinations[key]; ok {
				fmt.Fprintf(out, "ASP: output collision: %s and %s target %s\n", previous, file, dest)
				return 2
			}
			destinations[key] = file
			if write {
				if _, err := os.Lstat(dest); err == nil {
					fmt.Fprintf(out, "ASP: output already exists: %s\n", dest)
					return 2
				} else if !os.IsNotExist(err) {
					fmt.Fprintf(out, "ASP: %v\n", err)
					return 2
				}
			}
			manifest = append(manifest, struct {
				source, dest string
				copy         bool
			}{file, dest, copyFile})
		}
	}
	cache := asp.NewSourceCache()
	reports := make([]asp.Report, len(files))
	texts := make([]string, len(files))
	work := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < jobs; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				texts[i], reports[i] = asp.EraseASPWithCache(context.Background(), files[i], root, types, eraseOnly, cache, emitOnError)
			}
		}()
	}
	for i := range files {
		work <- i
	}
	close(work)
	wg.Wait()
	// Existing ASP pages are copied unchanged, but still receive ordinary checker
	// diagnostics. Their type errors are reported without preventing deployment.
	var plainPages []string
	for _, file := range passthrough {
		if !eraseOnly && strings.EqualFold(filepath.Ext(file), ".asp") {
			plainPages = append(plainPages, file)
		}
	}
	plainReports := make([]asp.Report, len(plainPages))
	plainWork := make(chan int)
	for worker := 0; worker < jobs; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range plainWork {
				plainReports[i] = asp.CheckReportWithOverlay(context.Background(), plainPages[i], root, types, policy.rules, policy.sourceTypes, policy.loose, "", policy.warnShared)
			}
		}()
	}
	for i := range plainPages {
		plainWork <- i
	}
	close(plainWork)
	wg.Wait()
	// Finish validation for every page before any output. A syntax/erasure/input
	// failure blocks the entire batch; semantic diagnostics alone do not block
	// the default emission policy. The strict policy also gates the whole batch.
	blocked := false
	for _, report := range reports {
		if report.Error != "" || !report.EmissionSafe {
			blocked = true
		}
	}
	for _, report := range plainReports {
		if report.Error != "" {
			blocked = true
		}
		if !emitOnError {
			for _, d := range report.Diagnostics {
				if !d.Warning {
					blocked = true
				}
			}
		}
	}
	failed, errors, clean := 0, 0, 0
	created := []string{}
	rollback := func() {
		for _, file := range created {
			os.Remove(file)
		}
	}
	for i := range reports {
		r := &reports[i]
		if r.Error != "" {
			failed++
			continue
		}
		if len(r.Diagnostics) > 0 {
			errors++
			if !r.EmissionSafe {
				continue
			}
		}
		if write && !blocked {
			outputDirectory := outDir
			if outDir != "" {
				rel, err := filepath.Rel(dir, filepath.Dir(files[i]))
				if err != nil {
					r.Error = err.Error()
					failed++
					continue
				}
				outputDirectory = filepath.Join(outDir, rel)
			}
			if _, err := asp.WriteErasedASP(files[i], outputDirectory, texts[i]); err != nil {
				r.Error = err.Error()
				failed++
				blocked = true
				rollback()
				continue
			}
			created = append(created, manifest[i].dest)
		}
		if len(r.Diagnostics) == 0 {
			clean++
		}
	}
	copied := 0
	if write && !blocked {
		for _, item := range manifest {
			if !item.copy {
				continue
			}
			if err := copyBuildFile(item.source, item.dest); err != nil {
				fmt.Fprintf(out, "ASP: copy failed: %v\n", err)
				failed++
				blocked = true
				rollback()
				break
			}
			created = append(created, item.dest)
			copied++
		}
	}
	if blocked {
		copied = 0
	}
	for _, report := range plainReports {
		if report.Error != "" {
			failed++
			continue
		}
		hasErrors := false
		for _, d := range report.Diagnostics {
			if !d.Warning {
				hasErrors = true
			}
		}
		if hasErrors {
			errors++
		} else {
			clean++
		}
	}
	reports = append(reports, plainReports...)
	if errorsOnly {
		for i := range reports {
			filtered := []asp.Diagnostic{}
			for _, d := range reports[i].Diagnostics {
				if !d.Warning {
					filtered = append(filtered, d)
				}
			}
			reports[i].Diagnostics = filtered
		}
	}
	if jsonOutput {
		result := struct {
			Version         int          `json:"version"`
			Pages           []asp.Report `json:"pages"`
			Clean           int          `json:"clean"`
			WithErrors      int          `json:"withErrors"`
			Failed          int          `json:"failed"`
			Seconds         float64      `json:"seconds"`
			Copied          int          `json:"copied"`
			CopyCandidates  int          `json:"copyCandidates"`
			EmissionBlocked bool         `json:"emissionBlocked"`
		}{1, reports, clean, errors, failed, time.Since(started).Seconds(), copied, len(passthrough), blocked}
		if err := json.NewEncoder(out).Encode(result); err != nil {
			return 2
		}
	} else {
		if write && blocked {
			fmt.Fprintln(out, "Emission blocked: at least one page failed validation; no batch outputs were written.")
		}
		for _, r := range reports {
			if r.Error != "" {
				fmt.Fprintf(out, "ASP: %s: %s\n", r.Entry, r.Error)
			}
			for _, d := range r.Diagnostics {
				severity := "error"
				if d.Warning {
					severity = "warning"
				}
				fmt.Fprintf(out, "%s(%d,%d): %s TS%d: %s\n", d.File, d.Line, d.Column, severity, d.Code, d.Message)
			}
		}
		fmt.Fprintf(out, "Checked %d typed and %d existing ASP pages: %d passed, %d with errors, %d failed; copied %d unchanged files (%d candidates); %.2fs (jobs=%d, erase-only=%t).\n", len(files), len(plainPages), clean, errors, failed, copied, len(passthrough), time.Since(started).Seconds(), jobs, eraseOnly)
	}
	if failed > 0 {
		return 2
	}
	if errors > 0 {
		return 1
	}
	return 0
}

func copyBuildFile(source, dest string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	output, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, writeErr := io.Copy(output, input)
	closeErr := output.Close()
	if writeErr != nil || closeErr != nil {
		os.Remove(dest)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return nil
}
