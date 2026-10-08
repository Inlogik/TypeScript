package asp

import (
	"fmt"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// Rename the two verified non-strict legacy identifiers in virtual text only.
// Member/property names are already legal and must not be renamed. Explicit
// strict scopes remain errors. Generated names cannot collide with source text.
func (m *MappedFile) rewriteReservedIdentifiers() {
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: tspath.RootedFilePathFromNormalized("/__asp_reserved.js")}, m.Text, core.ScriptKindJS)
	names := map[string]string{}
	for _, name := range []string{"private", "public"} {
		for i := 0; ; i++ {
			candidate := fmt.Sprintf("__asp_legacy_%s_%d", name, i)
			if !strings.Contains(m.Text, candidate) {
				names[name] = candidate
				break
			}
		}
	}
	var edits []sourceEdit
	var walk func(*ast.Node, *ast.Node, bool)
	walk = func(n, parent *ast.Node, strict bool) {
		if n.Kind == ast.KindClassDeclaration || n.Kind == ast.KindClassExpression {
			strict = true
		}
		if ast.IsFunctionLike(n) {
			if body := n.Body(); body != nil && body.Kind == ast.KindBlock {
				strict = strict || hasStrictPrologue(body.AsBlock().Statements.Nodes)
			}
		}
		if !strict && ast.IsIdentifier(n) && names[n.Text()] != "" {
			property := false
			if parent != nil {
				if parent.Kind == ast.KindPropertyAccessExpression && parent.Name() == n {
					property = true
				}
				if parent.Kind == ast.KindPropertyAssignment && parent.Name() == n {
					property = true
				}
			}
			if !property {
				start := scanner.GetTokenPosOfNode(n, file, false)
				replacement := names[n.Text()]
				if parent != nil && parent.Kind == ast.KindShorthandPropertyAssignment {
					replacement = n.Text() + ": " + replacement
				}
				edits = append(edits, sourceEdit{start, n.End(), replacement})
			}
		}
		n.ForEachChild(func(child *ast.Node) bool { walk(child, n, strict); return false })
	}
	strict := hasStrictPrologue(file.Statements.Nodes)
	file.ForEachChild(func(n *ast.Node) bool { walk(n, nil, strict); return false })
	m.applyEdits(edits)
}
