package asp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"g3pix.com.br/axonasp/v2/vbscript"
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/osvfs"
)

type eraseRange struct {
	start, end  int
	declaration bool
}

// EraseASP checks a TS server program and returns the original document with
// type-only spans replaced by whitespace. It never writes or expands includes
// in the returned document. Runtime JS syntax is not downleveled.
func EraseASP(ctx context.Context, entry, root string, typeFiles []string) (string, Report) {
	return EraseASPWithOptions(ctx, entry, root, typeFiles, false)
}

func EraseASPWithOptions(ctx context.Context, entry, root string, typeFiles []string, eraseOnly bool) (string, Report) {
	return EraseASPWithCache(ctx, entry, root, typeFiles, eraseOnly, nil)
}

func EraseASPWithCache(ctx context.Context, entry, root string, typeFiles []string, eraseOnly bool, cache *SourceCache, emitOnError ...bool) (string, Report) {
	abs, err := filepath.Abs(entry)
	r := Report{Version: 1, Entry: abs, Dependencies: []string{}, Diagnostics: []Diagnostic{}}
	fail := func(err error) (string, Report) { r.Error = err.Error(); return "", r }
	if err != nil {
		return fail(err)
	}
	if !strings.HasSuffix(strings.ToLower(abs), ".asp.ts") && !strings.HasSuffix(strings.ToLower(abs), ".inc.ts") {
		return fail(fmt.Errorf("erasure requires a .asp.ts or .inc.ts source"))
	}
	m, err := mapSources(abs, root, nil, false, cache)
	if err != nil {
		return fail(err)
	}
	for file, source := range m.Sources {
		var regions []vbscript.ASPRegion
		var err error
		if cache != nil {
			regions, err = cache.scan(file, source)
		} else {
			regions, err = vbscript.ScanASP(source)
		}
		if err != nil {
			return fail(err)
		}
		for _, region := range regions {
			if region.Kind == "include" && strings.HasSuffix(strings.ToLower(region.Path), ".ts") {
				return fail(fmt.Errorf("TypeScript include paths are unsupported; preserve plain runtime include paths and use .d.ts contracts"))
			}
			if region.Kind == "directive" {
				s := scanner.NewScanner()
				s.SetText(source[region.CodeStart:region.CodeEnd])
				for k := s.Scan(); k != ast.KindEndOfFile; k = s.Scan() {
					if k == ast.KindClassKeyword || k == ast.KindFunctionKeyword || k == ast.KindEnumKeyword {
						return fail(fmt.Errorf("server code/decorators cannot appear in an ASP directive"))
					}
				}
			}
		}
	}
	for file := range m.Sources {
		r.Dependencies = append(r.Dependencies, file)
	}
	// Classic ASP call-shaped setters are legal runtime syntax but invalid TS
	// assignment targets. Check a normalized virtual view; output continues to
	// use original ASP text and mapped type-only deletions, never these helpers.
	m.normalizeStringContinuations()
	m.rewriteMemberFunctions()
	m.rewriteIndexedAssignmentsForScript(core.ScriptKindTS)
	m.rewriteDeleteIdentifiers()
	m.rewriteCollectionCountCalls()
	m.rewriteReservedIdentifiers()
	sort.Strings(r.Dependencies)
	virtual := tspath.RootedFilePathFromAbsolute(abs + ".__asp_erase.ts")
	types := tspath.RootedFilePathFromAbsolute(abs + ".__asp_host.d.ts")
	fs := &overlayFS{FS: bundled.WrapFS(osvfs.FS()), files: map[tspath.RootedFilePath]string{virtual: m.Text, types: hostTypes}}
	roots := []tspath.RootedFilePath{virtual, types}
	companions, err := m.companionFiles()
	if err != nil {
		return fail(err)
	}
	for _, name := range append(append([]string{}, typeFiles...), companions...) {
		if !strings.HasSuffix(strings.ToLower(name), ".d.ts") {
			return fail(fmt.Errorf("--types must name a .d.ts file: %s", name))
		}
		path, err := filepath.Abs(name)
		if err != nil {
			return fail(err)
		}
		p := tspath.RootedFilePathFromAbsolute(path)
		if !fs.FileExists(p) {
			return fail(fmt.Errorf("declaration file not found: %s", name))
		}
		roots = append(roots, p)
		r.Dependencies = append(r.Dependencies, path)
	}
	h := &host{CompilerHost: compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil), virtual: virtual, types: types, mapped: m, scriptKind: core.ScriptKindTS}
	options := &core.CompilerOptions{NoEmit: core.TSTrue, SkipLibCheck: core.TSTrue, Strict: core.TSFalse, NoImplicitAny: core.TSFalse, UseUnknownInCatchVariables: core.TSFalse, Target: core.ScriptTargetESNext, Lib: []string{"lib.es5.d.ts"}}
	config := tsoptions.NewParsedCommandLine(options, roots, nil, virtual.Directory(), fs.CaseSensitivity())
	p := compiler.NewProgram(compiler.ProgramOptions{ProgramConfig: compiler.ProgramConfig{Config: config}, ProgramHosts: compiler.ProgramHosts{Host: h}})
	addError := func(pos int, message string) {
		file, line, col := m.Position(pos)
		r.Diagnostics = append(r.Diagnostics, Diagnostic{File: file, Line: line, Column: col, EndLine: line, EndColumn: col, Code: 95001, Message: message})
	}
	sf := p.GetSourceFile(virtual)
	if sf == nil {
		return fail(fmt.Errorf("could not parse virtual erasure source"))
	}
	ranges := typeErasureRanges(sf, addError)
	diags := p.GetProgramDiagnostics()
	diags = append(diags, p.GetSyntacticDiagnostics(ctx, nil)...)
	unsafe := len(r.Diagnostics) > 0 || len(diags) > 0
	// No-type input has a syntax/shape-validated no-op output. Avoid constructing
	// checkers and computing semantic diagnostics that would be discarded below.
	// Typed input still receives the same full validation before any output write.
	if !eraseOnly {
		diags = append(diags, p.GetGlobalDiagnostics(ctx)...)
		diags = append(diags, p.GetSemanticDiagnostics(ctx, nil)...)
	}
	for _, d := range compiler.SortAndDeduplicateDiagnostics(diags) {
		item := Diagnostic{Code: d.Code(), Message: d.String()}
		if d.File() != nil && d.File().FileName() == virtual {
			item.File, item.Line, item.Column = m.Position(d.Pos())
			_, item.EndLine, item.EndColumn = m.Position(d.End())
		} else if d.File() != nil {
			item.File = string(d.File().FileName())
			item.Line, item.Column = LineColumn(d.File().Text(), d.Pos())
			item.EndLine, item.EndColumn = LineColumn(d.File().Text(), d.End())
		}
		r.Diagnostics = append(r.Diagnostics, item)
	}
	beforeSafety := len(r.Diagnostics)
	var original []eraseRange
	for _, edit := range ranges {
		found := false
		for _, s := range m.Spans {
			if edit.start >= s.Start && edit.end <= s.End {
				found = true
				if s.File == abs {
					original = append(original, eraseRange{s.OriginalStart + edit.start - s.Start, s.OriginalStart + edit.end - s.Start, edit.declaration})
				} else if !strings.HasSuffix(strings.ToLower(s.File), ".inc.ts") && !strings.HasSuffix(strings.ToLower(s.File), ".asp.ts") {
					addError(edit.start, "typed include content must use a .inc.ts or .asp.ts source so it is emitted separately")
				}
				break
			}
		}
		if !found {
			addError(edit.start, "type syntax must be contained in one server-code region (not across ASP delimiters or includes)")
		}
	}
	// Parsing the erased program catches unsupported residual TS and accidental
	// token joins. No compiler emitter or legacy JScript rewriting is involved.
	erased := maskRanges(m.Text, ranges)
	js := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: virtual}, erased, core.ScriptKindJS)
	for _, d := range append(js.Diagnostics(), js.JSDiagnostics()...) {
		addError(d.Pos(), "erased server code is not valid JavaScript: "+d.String())
	}
	if runtimeShape(sf.AsNode()) != runtimeShape(js.AsNode()) {
		addError(0, "erasure would change JavaScript parsing (for example automatic semicolon insertion); add explicit semicolons or parentheses")
	}
	compactJS := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: virtual}, removeTypeRanges(m.Text, ranges), core.ScriptKindJS)
	if len(compactJS.Diagnostics()) > 0 || len(compactJS.JSDiagnostics()) > 0 || runtimeShape(js.AsNode()) != runtimeShape(compactJS.AsNode()) {
		addError(0, "removing type whitespace would change JavaScript parsing; add explicit separators or semicolons")
	}
	unsafe = unsafe || len(r.Diagnostics) > beforeSafety
	if unsafe || (len(r.Diagnostics) != 0 && !(len(emitOnError) > 0 && emitOnError[0])) {
		return "", r
	}
	r.EmissionSafe = true
	return removeTypeRanges(m.Sources[abs], original), r
}

// Remove erased syntax rather than leaving columns of padding. Preserve line
// breaks and insert a separator only when deletion would join identifier tokens.
// Untouched input (no type ranges) remains byte-for-byte unchanged.
func removeTypeRanges(text string, ranges []eraseRange) string {
	if len(ranges) == 0 {
		return text
	}
	edits := append([]eraseRange(nil), ranges...)
	for i := range edits {
		for edits[i].start > 0 && (text[edits[i].start-1] == ' ' || text[edits[i].start-1] == '\t') {
			edits[i].start--
		}
		if edits[i].declaration {
			// Remove complete declaration-only lines, never HTML/code sharing
			// the line. Include one line terminator so no empty placeholder remains.
			start := edits[i].start
			end := edits[i].end
			for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
				end++
			}
			if (start == 0 || text[start-1] == '\r' || text[start-1] == '\n') && (end == len(text) || text[end] == '\r' || text[end] == '\n') {
				if end < len(text) {
					if text[end] == '\r' && end+1 < len(text) && text[end+1] == '\n' {
						end += 2
					} else {
						end++
					}
				}
				edits[i].end = end
			} else {
				edits[i].declaration = false
			}
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	merged := []eraseRange{}
	for _, edit := range edits {
		if len(merged) > 0 && edit.start <= merged[len(merged)-1].end {
			merged[len(merged)-1].end = max(merged[len(merged)-1].end, edit.end)
			merged[len(merged)-1].declaration = merged[len(merged)-1].declaration && edit.declaration
		} else {
			merged = append(merged, edit)
		}
	}
	word := func(c byte) bool {
		return c >= 128 || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '$'
	}
	var out strings.Builder
	cursor := 0
	for _, edit := range merged {
		out.WriteString(text[cursor:edit.start])
		newline := false
		for i := edit.start; !edit.declaration && i < edit.end; i++ {
			if text[i] == '\r' || text[i] == '\n' {
				out.WriteByte(text[i])
				newline = true
			}
		}
		if !newline && edit.start > 0 && edit.end < len(text) && word(text[edit.start-1]) && word(text[edit.end]) {
			out.WriteByte(' ')
		}
		cursor = edit.end
	}
	out.WriteString(text[cursor:])
	return out.String()
}

// Compare runtime AST structure as a final fail-closed guard against ASI and
// precedence changes when types or overload declarations are removed.
func runtimeShape(root *ast.Node) string {
	var out strings.Builder
	var visit ast.Visitor
	visit = func(n *ast.Node) bool {
		if n == nil {
			return false
		}
		if (ast.IsTypeNode(n) && n.Kind != ast.KindExpressionWithTypeArguments) || n.Kind == ast.KindTypeParameter {
			return false
		}
		if n.Kind == ast.KindInterfaceDeclaration || n.Kind == ast.KindTypeAliasDeclaration || n.ModifierFlags()&ast.ModifierFlagsAmbient != 0 {
			return false
		}
		if n.Kind == ast.KindHeritageClause && n.AsHeritageClause().Token == ast.KindImplementsKeyword {
			return false
		}
		if n.Kind == ast.KindIndexSignature {
			return false
		}
		if n.FunctionLikeData() != nil && n.Body() == nil {
			return false
		}
		switch n.Kind {
		case ast.KindAsExpression, ast.KindSatisfiesExpression, ast.KindTypeAssertionExpression, ast.KindNonNullExpression, ast.KindExpressionWithTypeArguments:
			visit(n.Expression())
			return false
		case ast.KindPublicKeyword, ast.KindPrivateKeyword, ast.KindProtectedKeyword, ast.KindReadonlyKeyword, ast.KindAbstractKeyword, ast.KindOverrideKeyword:
			return false
		}
		out.WriteString(n.Kind.String())
		out.WriteByte('(')
		n.ForEachChild(func(child *ast.Node) bool {
			if n.Kind == ast.KindPropertyDeclaration && child == n.AsPropertyDeclaration().PostfixToken {
				return false
			}
			if n.Kind == ast.KindParameter && child == n.QuestionToken() {
				return false
			}
			if n.Kind == ast.KindVariableDeclaration && child == n.AsVariableDeclaration().ExclamationToken {
				return false
			}
			return visit(child)
		})
		out.WriteByte(')')
		return false
	}
	visit(root)
	return out.String()
}

func maskRanges(text string, ranges []eraseRange) string {
	bytes := []byte(text)
	for _, edit := range ranges {
		for i := edit.start; i < edit.end; i++ {
			if bytes[i] != '\r' && bytes[i] != '\n' {
				bytes[i] = ' '
			}
		}
	}
	return string(bytes)
}

func typeErasureRanges(sf *ast.SourceFile, reject func(int, string)) []eraseRange {
	text := sf.Text()
	var ranges []eraseRange
	start := func(n *ast.Node) int { return scanner.SkipTrivia(text, n.Pos()) }
	erase := func(a, b int) {
		if a < b {
			ranges = append(ranges, eraseRange{start: a, end: b})
		}
	}
	eraseDeclaration := func(a, b int) {
		if a < b {
			ranges = append(ranges, eraseRange{start: a, end: b, declaration: true})
		}
	}
	// Punctuation is found with the compiler scanner, never regex TS parsing.
	tokenBefore := func(lower, pos int, kind ast.Kind) int {
		s := scanner.NewScanner()
		s.SetText(text[lower:pos])
		result := -1
		for k := s.Scan(); k != ast.KindEndOfFile; k = s.Scan() {
			if k == kind {
				result = s.TokenEnd() - len(scanner.TokenToString(kind))
			}
		}
		if result < 0 {
			return -1
		}
		return lower + result
	}
	eraseList := func(n *ast.Node, list *ast.NodeList) {
		if list == nil || len(list.Nodes) == 0 {
			return
		}
		a := tokenBefore(start(n), start(list.Nodes[0]), ast.KindLessThanToken)
		b := scanner.SkipTrivia(text, list.End())
		if a < 0 || b >= len(text) || text[b] != '>' {
			reject(list.Pos(), "cannot safely erase type parameter/argument delimiters")
			return
		}
		erase(a, b+1)
	}
	var visit ast.Visitor
	visit = func(n *ast.Node) bool {
		if n == nil {
			return false
		}
		switch n.Kind {
		case ast.KindEnumDeclaration, ast.KindModuleDeclaration, ast.KindImportDeclaration, ast.KindImportEqualsDeclaration, ast.KindExportDeclaration, ast.KindExportAssignment:
			reject(start(n), "enums, namespaces and imports/exports are unsupported in ASP erasure mode")
			return false
		case ast.KindClassDeclaration, ast.KindClassExpression:
			eraseList(n, n.ClassLikeData().TypeParameters)
		case ast.KindHeritageClause:
			if n.AsHeritageClause().Token == ast.KindImplementsKeyword {
				erase(start(n), n.End())
				return false
			}
		case ast.KindIndexSignature:
			eraseDeclaration(start(n), n.End())
			return false
		}
		for _, mod := range n.ModifierNodes() {
			switch mod.Kind {
			case ast.KindExportKeyword, ast.KindDefaultKeyword, ast.KindDecorator, ast.KindAccessorKeyword:
				reject(start(mod), "module transformations, decorators and auto-accessors are unsupported")
			case ast.KindDeclareKeyword:
				eraseDeclaration(start(n), n.End())
				return false
			case ast.KindPublicKeyword, ast.KindPrivateKeyword, ast.KindProtectedKeyword, ast.KindReadonlyKeyword, ast.KindAbstractKeyword, ast.KindOverrideKeyword:
				if n.Kind == ast.KindParameter {
					reject(start(mod), "parameter properties require runtime generation")
				} else {
					erase(start(mod), mod.End())
				}
			}
		}
		switch n.Kind {
		case ast.KindInterfaceDeclaration, ast.KindTypeAliasDeclaration:
			eraseDeclaration(start(n), n.End())
			return false
		case ast.KindAsExpression, ast.KindSatisfiesExpression:
			erase(n.Expression().End(), n.End())
			visit(n.Expression())
			return false
		case ast.KindTypeAssertionExpression:
			erase(start(n), start(n.Expression()))
			visit(n.Expression())
			return false
		case ast.KindNonNullExpression:
			erase(n.Expression().End(), n.End())
			visit(n.Expression())
			return false
		case ast.KindCallExpression, ast.KindNewExpression, ast.KindTaggedTemplateExpression:
			eraseList(n, n.TypeArgumentList())
		case ast.KindExpressionWithTypeArguments:
			eraseList(n, n.TypeArgumentList())
			visit(n.Expression())
			return false
		case ast.KindVariableDeclaration:
			if token := n.AsVariableDeclaration().ExclamationToken; token != nil {
				erase(start(token), token.End())
			}
		case ast.KindParameter:
			if n.Name() != nil && ast.IsIdentifier(n.Name()) && n.Name().Text() == "this" {
				reject(start(n), "explicit this parameters are outside the ASP erasure MVP")
			}
			if token := n.QuestionToken(); token != nil {
				erase(start(token), token.End())
			}
		case ast.KindPropertyDeclaration:
			if token := n.AsPropertyDeclaration().PostfixToken; token != nil {
				erase(start(token), token.End())
			}
		}
		if fn := n.FunctionLikeData(); fn != nil {
			if n.Body() == nil {
				eraseDeclaration(start(n), n.End())
				return false
			}
			eraseList(n, fn.TypeParameters)
		}
		if t := n.Type(); t != nil {
			a := tokenBefore(start(n), start(t), ast.KindColonToken)
			if a < start(n) {
				reject(start(t), "cannot safely locate type annotation")
			} else {
				erase(a, t.End())
			}
		}
		// Type children are erased by their owning syntax, not independently.
		if ast.IsTypeNode(n) || n.Kind == ast.KindTypeParameter {
			return false
		}
		n.ForEachChild(visit)
		return false
	}
	sf.AsNode().ForEachChild(visit)
	return ranges
}

// WriteErasedASP creates a separate .asp file exclusively. Existing destinations,
// including dangling symlinks, are never overwritten.
func WriteErasedASP(entry, outDir, text string) (string, error) {
	if !strings.HasSuffix(strings.ToLower(entry), ".asp.ts") && !strings.HasSuffix(strings.ToLower(entry), ".inc.ts") {
		return "", fmt.Errorf("erasure requires a .asp.ts or .inc.ts source")
	}
	destination := entry[:len(entry)-3]
	if outDir != "" {
		destination = filepath.Join(outDir, filepath.Base(destination))
	}
	dest, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	input, err := os.Stat(entry)
	if err != nil {
		return "", err
	}
	if output, err := os.Stat(dest); err == nil && os.SameFile(input, output) {
		return "", fmt.Errorf("output must not overwrite the input")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	success := false
	defer func() {
		if !success {
			os.Remove(dest)
		}
	}()
	_, err = f.WriteString(text)
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	success = true
	return dest, nil
}
