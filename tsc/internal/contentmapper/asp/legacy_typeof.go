package asp

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// Only legacy typeof/date and typeof/Function comparisons are advisory.
// Other impossible comparisons (including typeof/null) keep their severity.
func legacyTypeofWarnings(file *ast.SourceFile, diags []*ast.Diagnostic) map[*ast.Diagnostic]string {
	warnings := map[*ast.Diagnostic]string{}
	if file == nil {
		return warnings
	}
	type comparison struct {
		start, end int
		value      string
	}
	var ranges []comparison
	unwrap := func(n *ast.Node) *ast.Node {
		for n.Kind == ast.KindParenthesizedExpression {
			n = n.AsParenthesizedExpression().Expression
		}
		return n
	}
	var visit ast.Visitor
	visit = func(n *ast.Node) bool {
		if n.Kind == ast.KindBinaryExpression {
			b := n.AsBinaryExpression()
			switch b.OperatorToken.Kind {
			case ast.KindEqualsEqualsToken, ast.KindEqualsEqualsEqualsToken, ast.KindExclamationEqualsToken, ast.KindExclamationEqualsEqualsToken:
				left, right := unwrap(b.Left), unwrap(b.Right)
				if right.Kind == ast.KindTypeOfExpression {
					left, right = right, left
				}
				if left.Kind == ast.KindTypeOfExpression && right.Kind == ast.KindStringLiteral && (right.Text() == "date" || right.Text() == "Function") {
					ranges = append(ranges, comparison{scanner.GetTokenPosOfNode(n, file, false), n.End(), right.Text()})
				}
			}
		}
		n.ForEachChild(visit)
		return false
	}
	file.ForEachChild(visit)
	for _, d := range diags {
		if d.File() != file || d.Code() != 2367 {
			continue
		}
		for _, r := range ranges {
			if d.Pos() >= r.start && d.End() <= r.end {
				if r.value == "date" {
					warnings[d] = " (legacy typeof/date comparison; AxonASP date values report typeof object)"
				} else {
					warnings[d] = " (legacy typeof/Function comparison; AxonASP callable values report lowercase function; historical IIS behavior unverified)"
				}
				break
			}
		}
	}
	return warnings
}
