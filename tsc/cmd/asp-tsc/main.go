package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/microsoft/TypeScript/tsc/internal/contentmapper/asp"
)

type typeFiles []string

func (f *typeFiles) String() string         { return fmt.Sprint([]string(*f)) }
func (f *typeFiles) Set(value string) error { *f = append(*f, value); return nil }

func run(args []string, out io.Writer) int {
	flags := flag.NewFlagSet("asp-tsc", flag.ContinueOnError)
	flags.SetOutput(out)
	root := flags.String("root", "", "application root for virtual includes")
	rules := flags.String("dependency-rules", "", "project JSON conditional include rules (requires --root)")
	sourceTypes := flags.String("source-types", "", "project JSON virtual variable annotations (requires --root)")
	looseVariables := flags.Bool("loose-variables", false, "treat unannotated variable bindings as any; explicit contracts remain checked")
	errorsOnly := flags.Bool("errors-only", false, "hide warnings; exit codes and checking are unchanged")
	jsonOutput := flags.Bool("json", false, "versioned JSON diagnostics and include dependencies")
	virtualOutput := flags.Bool("virtual-json", false, "inference-enabled virtual JS and UTF-16 mappings for editor requests")
	overlay := flags.String("source-overlay", "", "JSON absolute filename/text overlays for checking or virtual output")
	project := flags.String("project", "", "project JSON config; contained paths are config-relative")
	noProject := flags.Bool("no-project", false, "disable automatic asp-check.json discovery")
	scan := flags.String("scan", "", "recursively check all .asp entry files in this directory")
	jobs := flags.Int("jobs", min(8, runtime.NumCPU()), "batch concurrency (1-16; default up to 8 logical CPUs)")
	warnShared := flags.Bool("warn-shared-globals", false, "warn on cross-file global variable dependencies not acknowledged by companions")
	emitASP := flags.Bool("emit-asp", false, "check erasable .asp.ts server code and emit a separate .asp document")
	eraseOnly := flags.Bool("erase-only", false, "with --emit-asp, skip semantic checking explicitly; syntax and erasure guards remain")
	emitOnError := flags.Bool("emit-on-error", true, "emit despite semantic errors (default); syntax/erasure errors still block output")
	noEmitOnError := flags.Bool("no-emit-on-error", false, "with --emit-asp, block output on semantic errors too")
	outDir := flags.String("out-dir", "", "optional output directory for --emit-asp; default is beside input, without .ts")
	var types typeFiles
	flags.Var(&types, "types", "project .d.ts file (repeatable; relative to working directory)")
	noEmit := flags.Bool("noEmit", true, "check only; --emit-asp writes unless --noEmit is explicitly true")
	defaultEntry := ""
	if err := flags.Parse(args); err != nil {
		return 2
	}
	set := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if *outDir != "" {
		if set["emit-asp"] && !*emitASP {
			fmt.Fprintln(out, "ASP: --out-dir conflicts with --emit-asp=false")
			return 2
		}
		*emitASP = true
	}
	if *noProject && *project != "" {
		fmt.Fprintln(out, "ASP: --project and --no-project cannot be combined")
		return 2
	}
	if *project == "" && !*noProject {
		start := *scan
		if start == "" && flags.NArg() > 0 {
			start = filepath.Dir(flags.Arg(0))
		}
		if start == "" {
			start = "."
		}
		found, err := discoverProject(start)
		if err != nil {
			fmt.Fprintf(out, "ASP: %v\n", err)
			return 2
		}
		*project = found
	}
	if !*noEmit && !*emitASP {
		fmt.Fprintln(out, "asp-tsc is check-only: --noEmit=false is unsupported")
		return 2
	}
	if *eraseOnly && !*emitASP {
		fmt.Fprintln(out, "ASP: --erase-only requires --emit-asp")
		return 2
	}
	if (set["emit-on-error"] && *emitOnError || *noEmitOnError) && (!*emitASP || *eraseOnly) {
		fmt.Fprintln(out, "ASP: --emit-on-error requires --emit-asp and cannot combine with --erase-only")
		return 2
	}
	if *noEmitOnError {
		*emitOnError = false
	}
	if (*outDir != "" && !*emitASP) || (*emitASP && (*virtualOutput || *overlay != "")) {
		fmt.Fprintln(out, "ASP: --emit-asp is single-entry only and does not support legacy checking transforms, scan, overlays or virtual output; --out-dir requires --emit-asp")
		return 2
	}
	if *project != "" {
		p, err := readProject(*project)
		if err != nil {
			fmt.Fprintf(out, "ASP: %v\n", err)
			return 2
		}
		set := map[string]bool{}
		flags.Visit(func(f *flag.Flag) { set[f.Name] = true })
		if !set["root"] {
			*root = p.Root
		}
		if !set["types"] {
			types = p.Types
		}
		if !set["dependency-rules"] {
			*rules = p.DependencyRules
		}
		if !set["source-types"] {
			*sourceTypes = p.SourceTypes
		}
		if !set["loose-variables"] {
			*looseVariables = p.LooseVariables
		}
		defaultEntry = p.Entry
		// Explicit entry files (including editor requests) must not unexpectedly
		// become whole-project scans. Config scan is the default only without one.
		if !set["scan"] && flags.NArg() == 0 {
			*scan = p.Scan
		}
	}
	if *scan != "" {
		if flags.NArg() != 0 || *virtualOutput || *overlay != "" || *jobs < 1 || *jobs > 16 {
			fmt.Fprintln(out, "ASP: --scan requires no entry argument/overlay/virtual output and --jobs 1-16")
			return 2
		}
		if *emitASP {
			return runEraseScanWithPolicy(out, *scan, *root, types, *outDir, *eraseOnly, !(set["noEmit"] && *noEmit), *jobs, *jsonOutput, *emitOnError, *errorsOnly, buildCheckPolicy{rules: *rules, sourceTypes: *sourceTypes, loose: *looseVariables, warnShared: *warnShared})
		}
		return runScan(out, *scan, *root, types, *rules, *sourceTypes, *looseVariables, *jobs, *jsonOutput, *errorsOnly, *warnShared)
	}
	entry := flags.Arg(0)
	if flags.NArg() == 0 {
		entry = defaultEntry
	}
	if flags.NArg() > 1 || entry == "" {
		fmt.Fprintln(out, "usage: asp-tsc [--noEmit] [--root PATH] [--types FILE.d.ts] page.asp")
		return 2
	}
	writeASP := *emitASP && !(set["noEmit"] && *noEmit)
	if *emitASP && (*rules != "" || *sourceTypes != "" || *looseVariables) {
		fmt.Fprintln(out, "ASP: --emit-asp does not support legacy project checking transforms")
		return 2
	}
	if *virtualOutput {
		report := asp.PrepareVirtualWithOverlay(entry, *root, types, *sourceTypes, *overlay)
		if err := json.NewEncoder(out).Encode(report); err != nil {
			return 2
		}
		if report.Error != "" {
			return 2
		}
		return 0
	}
	var report asp.Report
	if *emitASP {
		var text string
		text, report = asp.EraseASPWithCache(context.Background(), entry, *root, types, *eraseOnly, nil, *emitOnError)
		if writeASP && report.Error == "" && report.EmissionSafe {
			if _, err := asp.WriteErasedASP(entry, *outDir, text); err != nil {
				report.Error = err.Error()
			}
		}
	} else {
		report = asp.CheckReportWithOverlay(context.Background(), entry, *root, types, *rules, *sourceTypes, *looseVariables, *overlay, *warnShared)
	}
	if *jsonOutput {
		if *errorsOnly {
			filtered := []asp.Diagnostic{}
			for _, d := range report.Diagnostics {
				if !d.Warning {
					filtered = append(filtered, d)
				}
			}
			report.Diagnostics = filtered
		}
		if err := json.NewEncoder(out).Encode(report); err != nil {
			return 2
		}
		if report.Error != "" {
			return 2
		}
		for _, d := range report.Diagnostics {
			if !d.Warning {
				return 1
			}
		}
		return 0
	}
	if report.Error != "" {
		fmt.Fprintf(out, "ASP: %s\n", report.Error)
		return 2
	}
	hasErrors := false
	for _, d := range report.Diagnostics {
		if *errorsOnly && d.Warning {
			continue
		}
		if d.File != "" {
			fmt.Fprintf(out, "%s(%d,%d): ", d.File, d.Line, d.Column)
		}
		severity := "error"
		if d.Warning {
			severity = "warning"
		} else {
			hasErrors = true
		}
		fmt.Fprintf(out, "%s TS%d: %s\n", severity, d.Code, d.Message)
	}
	if hasErrors {
		return 1
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }
