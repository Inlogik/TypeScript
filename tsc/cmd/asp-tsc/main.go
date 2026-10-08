package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

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
	scan := flags.String("scan", "", "recursively check all .asp entry files in this directory")
	jobs := flags.Int("jobs", 2, "batch concurrency (1-16)")
	warnShared := flags.Bool("warn-shared-globals", false, "warn on cross-file global variable dependencies not acknowledged by companions")
	var types typeFiles
	flags.Var(&types, "types", "project .d.ts file (repeatable; relative to working directory)")
	noEmit := flags.Bool("noEmit", true, "checking only; emission is always disabled")
	defaultEntry := ""
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !*noEmit {
		fmt.Fprintln(out, "asp-tsc is check-only: --noEmit=false is unsupported")
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
	}
	if *scan != "" {
		if flags.NArg() != 0 || *virtualOutput || *overlay != "" || *jobs < 1 || *jobs > 16 {
			fmt.Fprintln(out, "ASP: --scan requires no entry argument/overlay/virtual output and --jobs 1-16")
			return 2
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
	report := asp.CheckReportWithOverlay(context.Background(), entry, *root, types, *rules, *sourceTypes, *looseVariables, *overlay, *warnShared)
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
