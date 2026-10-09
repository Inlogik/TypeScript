package asp

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/osvfs"
)

//go:embed classic-asp.d.ts
var hostTypes string

type Diagnostic struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndLine   int    `json:"endLine"`
	EndColumn int    `json:"endColumn"`
	Code      int32  `json:"code"`
	Message   string `json:"message"`
	Warning   bool   `json:"warning"`
}

// Report is the versioned check-on-save protocol. Positions are one-based UTF-16.
type Report struct {
	EmissionSafe bool         `json:"emissionSafe,omitempty"`
	Version      int          `json:"version"`
	Entry        string       `json:"entry"`
	Dependencies []string     `json:"dependencies"`
	Diagnostics  []Diagnostic `json:"diagnostics"`
	Error        string       `json:"error,omitempty"`
}

func CheckReport(ctx context.Context, entry, root string, typeFiles []string, rulesFile, sourceTypesFile string, looseVariables bool) Report {
	return CheckReportWithOverlay(ctx, entry, root, typeFiles, rulesFile, sourceTypesFile, looseVariables, "")
}

func CheckReportWithOverlay(ctx context.Context, entry, root string, typeFiles []string, rulesFile, sourceTypesFile string, looseVariables bool, overlayFile string, warnShared ...bool) Report {
	abs, err := filepath.Abs(entry)
	r := Report{Version: 1, Entry: abs, Dependencies: []string{}, Diagnostics: []Diagnostic{}}
	if err != nil {
		r.Error = err.Error()
		return r
	}
	var sources map[string]string
	if overlayFile != "" {
		data, err := os.ReadFile(overlayFile)
		if err != nil {
			r.Error = err.Error()
			return r
		}
		if err = json.Unmarshal(data, &sources); err != nil {
			r.Error = err.Error()
			return r
		}
	}
	diagnostics, err := checkWithReportSources(ctx, entry, root, typeFiles, rulesFile, sourceTypesFile, looseVariables, &r, sources, warnShared...)
	if err != nil {
		r.Error = err.Error()
	} else if diagnostics != nil {
		r.Diagnostics = diagnostics
	}
	return r
}

type host struct {
	compiler.CompilerHost
	virtual, types tspath.RootedFilePath
	mapped         *MappedFile
	scriptKind     core.ScriptKind
}

type overlayFS struct {
	vfs.FS
	files map[tspath.RootedFilePath]string
}

func (f *overlayFS) FileExists(path tspath.RootedFilePath) bool {
	if _, ok := f.files[path]; ok {
		return true
	}
	return f.FS.FileExists(path)
}

func (f *overlayFS) ReadFile(path tspath.RootedFilePath) (string, bool) {
	if text, ok := f.files[path]; ok {
		return text, true
	}
	return f.FS.ReadFile(path)
}

func (h *host) GetSourceFile(opts ast.SourceFileParseOptions) *ast.SourceFile {
	if opts.FileName == h.virtual {
		kind := h.scriptKind
		if kind == core.ScriptKindUnknown {
			kind = core.ScriptKindJS
		}
		return parser.ParseSourceFile(opts, h.mapped.Text, kind)
	}
	if opts.FileName == h.types {
		return parser.ParseSourceFile(opts, hostTypes, core.ScriptKindTS)
	}
	return h.CompilerHost.GetSourceFile(opts)
}

// Check uses an in-memory JavaScript source for one ASP entry point. Includes
// share its global scope. It never invokes compiler emission or writes sources.
func Check(ctx context.Context, entry, root string) ([]Diagnostic, error) {
	return CheckWithTypes(ctx, entry, root, nil)
}

// CheckWithTypes adds project-specific ambient declarations without embedding
// application knowledge in the generic ASP host declarations.
func CheckWithTypes(ctx context.Context, entry, root string, typeFiles []string) ([]Diagnostic, error) {
	return CheckWithDependencyRules(ctx, entry, root, typeFiles, "")
}

func CheckWithDependencyRules(ctx context.Context, entry, root string, typeFiles []string, rulesFile string) ([]Diagnostic, error) {
	return CheckWithProject(ctx, entry, root, typeFiles, rulesFile, "")
}

func CheckWithProject(ctx context.Context, entry, root string, typeFiles []string, rulesFile, sourceTypesFile string) ([]Diagnostic, error) {
	return CheckWithOptions(ctx, entry, root, typeFiles, rulesFile, sourceTypesFile, false)
}

// CheckWithOptions optionally relaxes unannotated variable bindings for legacy
// JavaScript. Explicit annotations and project source contracts remain checked.
func CheckWithOptions(ctx context.Context, entry, root string, typeFiles []string, rulesFile, sourceTypesFile string, looseVariables bool) ([]Diagnostic, error) {
	return checkWithReport(ctx, entry, root, typeFiles, rulesFile, sourceTypesFile, looseVariables, nil)
}

func checkWithReport(ctx context.Context, entry, root string, typeFiles []string, rulesFile, sourceTypesFile string, looseVariables bool, report *Report) ([]Diagnostic, error) {
	return checkWithReportSources(ctx, entry, root, typeFiles, rulesFile, sourceTypesFile, looseVariables, report, nil)
}

func checkWithReportSources(ctx context.Context, entry, root string, typeFiles []string, rulesFile, sourceTypesFile string, looseVariables bool, report *Report, sources map[string]string, warnShared ...bool) ([]Diagnostic, error) {
	rules, err := loadDependencyRules(rulesFile, root)
	if err != nil {
		return nil, err
	}
	m, err := mapCheckingSources(entry, root, sources)
	if err != nil {
		return nil, err
	}
	companions, err := m.companionFiles()
	if err != nil {
		return nil, err
	}
	typeFiles = append(append([]string{}, typeFiles...), companions...)
	if report != nil {
		for file := range m.Sources {
			report.Dependencies = append(report.Dependencies, file)
		}
		for _, file := range append(append([]string{}, typeFiles...), rulesFile, sourceTypesFile) {
			if file != "" {
				abs, err := filepath.Abs(file)
				if err != nil {
					return nil, err
				}
				report.Dependencies = append(report.Dependencies, abs)
			}
		}
		sort.Strings(report.Dependencies)
	}
	if err = m.applySourceTypes(sourceTypesFile, root); err != nil {
		return nil, err
	}
	typedEntry := m.hasTypedSource()
	if looseVariables {
		m.loosenVariables()
	}
	m.defaultParameterTypes()
	abs, err := filepath.Abs(entry)
	if err != nil {
		return nil, err
	}
	virtual := tspath.RootedFilePathFromAbsolute(abs + ".__asp_check.js")
	if typedEntry {
		virtual = tspath.RootedFilePathFromAbsolute(abs + ".__asp_check.ts")
	}
	types := tspath.RootedFilePathFromAbsolute(abs + ".__asp_host.d.ts")
	fs := &overlayFS{FS: bundled.WrapFS(osvfs.FS()), files: map[tspath.RootedFilePath]string{virtual: m.Text, types: hostTypes}}
	roots := []tspath.RootedFilePath{virtual, types}
	for _, name := range typeFiles {
		if !strings.HasSuffix(strings.ToLower(name), ".d.ts") {
			return nil, fmt.Errorf("--types must name a .d.ts file: %s", name)
		}
		abs, err := filepath.Abs(name)
		if err != nil {
			return nil, err
		}
		path := tspath.RootedFilePathFromAbsolute(abs)
		if !fs.FileExists(path) {
			return nil, fmt.Errorf("declaration file not found: %s", name)
		}
		roots = append(roots, path)
	}
	h := &host{CompilerHost: compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil), virtual: virtual, types: types, mapped: m}
	if typedEntry {
		h.scriptKind = core.ScriptKindTS
	}
	options := &core.CompilerOptions{
		AllowJs:                    core.TSTrue,
		CheckJs:                    core.TSTrue,
		NoEmit:                     core.TSTrue,
		SkipLibCheck:               core.TSTrue,
		Strict:                     core.TSFalse,
		NoImplicitAny:              core.TSFalse,
		UseUnknownInCatchVariables: core.TSFalse,
		Target:                     core.ScriptTargetES2015,
		Lib:                        []string{"lib.es5.d.ts"},
	}
	config := tsoptions.NewParsedCommandLine(options, roots, nil, virtual.Directory(), fs.CaseSensitivity())
	p := compiler.NewProgram(compiler.ProgramOptions{ProgramConfig: compiler.ProgramConfig{Config: config}, ProgramHosts: compiler.ProgramHosts{Host: h}})
	diags := p.GetProgramDiagnostics()
	diags = append(diags, p.GetGlobalDiagnostics(ctx)...)
	diags = append(diags, p.GetSyntacticDiagnostics(ctx, nil)...)
	diags = append(diags, p.GetSemanticDiagnostics(ctx, nil)...)
	diags = compiler.SortAndDeduplicateDiagnostics(diags)
	warnings := implicitGlobalWarnings(p.GetSourceFile(virtual), diags)
	typeofWarnings := legacyTypeofWarnings(p.GetSourceFile(virtual), diags)
	dependencyWarnings := conditionalDependencyWarnings(p.GetSourceFile(virtual), m, diags, entry, root, rules)
	var result []Diagnostic
	seen := map[Diagnostic]bool{}
	for _, d := range diags {
		r := Diagnostic{Code: d.Code(), Message: d.String(), Warning: warnings[d]}
		if r.Warning {
			r.Message += " (possible implicit global; assignment may not execute before this reference)"
		}
		if note, ok := typeofWarnings[d]; ok {
			r.Warning = true
			r.Message += note
		}
		// Legacy ASP var redeclarations are legal at runtime even when their
		// inferred types differ. Keep the mismatch visible without failing the
		// check; declarations from external .d.ts files are not downgraded.
		if d.Code() == 2403 && d.File() != nil && d.File().FileName() == virtual {
			r.Warning = true
			r.Message += " (legacy redeclaration with differing types; compiler inference is unchanged)"
		}
		if message, ok := dependencyWarnings[d]; ok {
			r.Warning = true
			r.Message = message
		}
		if d.File() != nil {
			if d.File().FileName() == virtual {
				r.File, r.Line, r.Column = m.Position(d.Pos())
				endFile, endLine, endColumn := m.Position(d.End())
				if endFile == r.File && (endLine > r.Line || (endLine == r.Line && endColumn >= r.Column)) {
					r.EndLine, r.EndColumn = endLine, endColumn
				}
				if r.File == "" {
					r.File = abs
					r.Line, r.Column = 1, 1
				}
			} else {
				r.File = d.File().FileName().AsString()
				r.Line, r.Column = LineColumn(d.File().Text(), d.Pos())
				r.EndLine, r.EndColumn = LineColumn(d.File().Text(), d.End())
			}
			if r.EndLine == 0 {
				r.EndLine, r.EndColumn = r.Line, r.Column
			}
		}
		// Repeated textual includes must stay in the program, but identical
		// original-source diagnostics need only appear once per entry report.
		if !seen[r] {
			result = append(result, r)
			seen[r] = true
		}
	}
	if len(warnShared) > 0 && warnShared[0] {
		for _, d := range sharedGlobalDiagnostics(ctx, p, m, virtual, companions) {
			if !seen[d] {
				result = append(result, d)
				seen[d] = true
			}
		}
	}
	for _, d := range m.IncludeDiagnostics {
		if !seen[d] {
			result = append(result, d)
			seen[d] = true
		}
	}
	sortMappedDiagnostics(result)
	return result, nil
}

// Order after source mapping and severity classification, so included sources
// and compatibility warnings have the same predictable order as CLI locations.
func sortMappedDiagnostics(diagnostics []Diagnostic) {
	sort.SliceStable(diagnostics, func(i, j int) bool {
		a, b := diagnostics[i], diagnostics[j]
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
}
