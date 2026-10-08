package asp

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// implicitGlobalWarnings classifies diagnostics, not compiler bindings. A plain
// non-strict assignment to an unresolved identifier is evidence of an implicit
// global. Other non-strict unresolved references to that name in this entry's
// expanded source are warnings too. This is deliberately not a definite-
// assignment analysis: a conditional/later assignment may never execute.
// Strict/class references, updates/compound assignments and unknown read-only
// names are not downgraded. Typos remain visible rather than becoming ambient any.
func implicitGlobalWarnings(file *ast.SourceFile, diagnostics []*ast.Diagnostic) map[*ast.Diagnostic]bool {
	if file == nil {
		return nil
	}
	unresolved := map[int]*ast.Diagnostic{}
	for _, d := range diagnostics {
		if d.File() == file && (d.Code() == 2304 || d.Code() == 2552) {
			unresolved[d.Pos()] = d
		}
	}
	candidates := map[string]bool{}
	references := map[*ast.Diagnostic]string{}
	var walk func(*ast.Node, bool)
	walk = func(n *ast.Node, strict bool) {
		if n.Kind == ast.KindClassDeclaration || n.Kind == ast.KindClassExpression {
			strict = true
		}
		if ast.IsFunctionLike(n) {
			if body := n.Body(); body != nil && body.Kind == ast.KindBlock {
				strict = strict || hasStrictPrologue(body.AsBlock().Statements.Nodes)
			}
		}
		if !strict {
			// A bare for-in/of target is assigned by each iteration, just as a
			// plain assignment can introduce a non-strict implicit global.
			if n.Kind == ast.KindForInStatement || n.Kind == ast.KindForOfStatement {
				left := n.AsForInOrOfStatement().Initializer
				for left != nil && left.Kind == ast.KindParenthesizedExpression {
					left = left.AsParenthesizedExpression().Expression
				}
				if left != nil && ast.IsIdentifier(left) && unresolved[scanner.GetTokenPosOfNode(left, file, false)] != nil {
					candidates[left.Text()] = true
				}
			}
			if n.Kind == ast.KindBinaryExpression {
				b := n.AsBinaryExpression()
				left := b.Left
				for left.Kind == ast.KindParenthesizedExpression {
					left = left.AsParenthesizedExpression().Expression
				}
				if b.OperatorToken.Kind == ast.KindEqualsToken && ast.IsIdentifier(left) {
					if unresolved[scanner.GetTokenPosOfNode(left, file, false)] != nil {
						candidates[left.Text()] = true
					}
				}
			}
			if ast.IsIdentifier(n) {
				if d := unresolved[scanner.GetTokenPosOfNode(n, file, false)]; d != nil {
					references[d] = n.Text()
				}
			}
		}
		n.ForEachChild(func(child *ast.Node) bool { walk(child, strict); return false })
	}
	strict := hasStrictPrologue(file.Statements.Nodes)
	file.ForEachChild(func(n *ast.Node) bool { walk(n, strict); return false })
	warnings := map[*ast.Diagnostic]bool{}
	for d, name := range references {
		if candidates[name] {
			warnings[d] = true
		}
	}
	return warnings
}
