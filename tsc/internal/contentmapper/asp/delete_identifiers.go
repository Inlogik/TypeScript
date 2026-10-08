package asp

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// AxonASP compiles non-strict bare-identifier deletion to a true-valued no-op.
// Retain the identifier in a virtual helper so undefined-name checks still work.
// Explicit strict directives and class bodies retain native delete diagnostics.
func (m *MappedFile) rewriteDeleteIdentifiers() {
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: tspath.RootedFilePathFromNormalized("/__asp_delete.js")}, m.Text, core.ScriptKindJS)
	var edits []sourceEdit
	var walk func(*ast.Node, bool) bool
	walk = func(n *ast.Node, strict bool) bool {
		if n.Kind == ast.KindClassDeclaration || n.Kind == ast.KindClassExpression {
			strict = true
		}
		if ast.IsFunctionLike(n) {
			if body := n.Body(); body != nil && body.Kind == ast.KindBlock {
				strict = strict || hasStrictPrologue(body.AsBlock().Statements.Nodes)
			}
		}
		if !strict && n.Kind == ast.KindDeleteExpression {
			operand := n.AsDeleteExpression().Expression
			inner := operand
			for inner.Kind == ast.KindParenthesizedExpression {
				inner = inner.AsParenthesizedExpression().Expression
			}
			if ast.IsIdentifier(inner) {
				start := scanner.GetTokenPosOfNode(n, file, false)
				edits = append(edits, sourceEdit{start, start + len("delete"), "__aspDeleteIdentifier("}, sourceEdit{operand.End(), operand.End(), ")"})
			}
		}
		n.ForEachChild(func(child *ast.Node) bool { return walk(child, strict) })
		return false
	}
	strict := hasStrictPrologue(file.Statements.Nodes)
	file.ForEachChild(func(n *ast.Node) bool { return walk(n, strict) })
	m.applyEdits(edits)
}

func hasStrictPrologue(statements []*ast.Node) bool {
	for _, s := range statements {
		if !ast.IsPrologueDirective(s) {
			break
		}
		if s.AsExpressionStatement().Expression.Text() == "use strict" {
			return true
		}
	}
	return false
}
