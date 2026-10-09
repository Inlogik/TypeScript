package asp

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"strings"
)

// defaultParameterTypes prevents inference from making unannotated legacy
// parameters narrower than any. Explicit parameter types and callable contracts
// are preserved. Unannotated identifier parameters get a virtual undefined
// default to preserve legacy omitted-argument behavior; extra arguments remain
// checked. This compiler text is never executed or emitted.
func (m *MappedFile) defaultParameterTypes() {
	kind := core.ScriptKindJS
	if m.hasTypedSource() {
		kind = core.ScriptKindTS
	}
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: tspath.RootedFilePathFromNormalized("/__asp_parameters.js")}, m.Text, kind)
	var edits []sourceEdit
	var walk func(*ast.Node, *ast.Node)
	walk = func(n, parent *ast.Node) {
		if n.Kind == ast.KindFunctionDeclaration || n.Kind == ast.KindFunctionExpression || n.Kind == ast.KindArrowFunction || n.Kind == ast.KindMethodDeclaration || n.Kind == ast.KindConstructor || n.Kind == ast.KindGetAccessor || n.Kind == ast.KindSetAccessor {
			docs := n.JSDoc(file)
			if parent != nil {
				docs = append(docs, parent.JSDoc(file)...)
			}
			wholeSignature := false
			typedNames := map[string]bool{}
			var inspect ast.Visitor
			inspect = func(tag *ast.Node) bool {
				if tag.Kind == ast.KindJSDocTypeTag || tag.Kind == ast.KindJSDocOverloadTag {
					wholeSignature = true
				}
				if tag.Kind == ast.KindJSDocParameterTag && tag.Name() != nil {
					typedNames[tag.Name().Text()] = true
				}
				tag.ForEachChild(inspect)
				return false
			}
			for _, doc := range docs {
				inspect(doc)
			}
			if !wholeSignature {
				for _, parameter := range n.Parameters() {
					if parameter.Type() != nil || len(parameter.JSDoc(file)) > 0 || (parameter.Name() != nil && typedNames[parameter.Name().Text()]) {
						continue
					}
					start := scanner.GetTokenPosOfNode(parameter, file, false)
					if kind == core.ScriptKindTS {
						source, _, _ := m.Position(start)
						// Newly typed implementations keep TypeScript inference; only
						// untouched JScript siblings retain legacy optional-any params.
						if strings.HasSuffix(strings.ToLower(source), ".ts") || !ast.IsIdentifier(parameter.Name()) {
							continue
						}
						end := parameter.Name().End()
						annotation := ": any"
						if parameter.AsParameterDeclaration().DotDotDotToken != nil {
							annotation = ": any[]"
						} else if parameter.AsParameterDeclaration().Initializer == nil {
							annotation += " = undefined"
						}
						edits = append(edits, sourceEdit{end, end, annotation})
						continue
					}
					annotation := "/** @type {any} */ "
					if parameter.AsParameterDeclaration().DotDotDotToken != nil {
						annotation = "/** @type {any[]} */ "
					}
					edits = append(edits, sourceEdit{start, start, annotation})
					if parameter.Name() != nil && ast.IsIdentifier(parameter.Name()) && parameter.AsParameterDeclaration().Initializer == nil && parameter.AsParameterDeclaration().DotDotDotToken == nil {
						edits = append(edits, sourceEdit{parameter.Name().End(), parameter.Name().End(), "=undefined"})
					}
				}
			}
		}
		n.ForEachChild(func(child *ast.Node) bool { walk(child, n); return false })
	}
	file.ForEachChild(func(n *ast.Node) bool { walk(n, nil); return false })
	m.applyEdits(edits)
}
