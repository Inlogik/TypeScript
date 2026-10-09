package asp

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// loosenVariables changes binding inference only, not parameters or host types.
// Existing JSDoc is conservatively preserved, including project-injected contracts.
// Run after applySourceTypes so explicit project types win over this legacy mode.
func (m *MappedFile) loosenVariables() {
	kind := core.ScriptKindJS
	if m.hasTypedSource() {
		kind = core.ScriptKindTS
	}
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: tspath.RootedFilePathFromNormalized("/__asp_loose.js")}, m.Text, kind)
	var edits []sourceEdit
	var walk func(*ast.Node, []*ast.Node)
	walk = func(n *ast.Node, ancestors []*ast.Node) {
		if n.Kind == ast.KindVariableDeclaration {
			// Native JS checking rejects typed for-in bindings (TS2404). Leave
			// those bindings inferred rather than generating invalid compiler text.
			if len(ancestors) >= 2 && ancestors[len(ancestors)-1].Kind == ast.KindVariableDeclarationList && ancestors[len(ancestors)-2].Kind == ast.KindForInStatement {
				return
			}
			annotated := n.Type() != nil || len(n.JSDoc(file)) > 0
			// JSDoc on a declaration statement/list applies to its bindings too.
			for i := len(ancestors) - 1; i >= 0; i-- {
				parent := ancestors[i]
				if parent.Kind != ast.KindVariableDeclarationList && parent.Kind != ast.KindVariableStatement {
					break
				}
				annotated = annotated || len(parent.JSDoc(file)) > 0
			}
			if !annotated {
				start := scanner.GetTokenPosOfNode(n, file, false)
				if kind == core.ScriptKindTS && ast.IsIdentifier(n.Name()) {
					end := n.Name().End()
					edits = append(edits, sourceEdit{end, end, ": any"})
				} else if kind == core.ScriptKindJS {
					edits = append(edits, sourceEdit{start, start, "/** @type {any} */ "})
				}
			}
		}
		n.ForEachChild(func(child *ast.Node) bool {
			walk(child, append(ancestors, n))
			return false
		})
	}
	file.ForEachChild(func(n *ast.Node) bool { walk(n, nil); return false })
	m.applyEdits(edits)
}
