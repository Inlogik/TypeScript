package asp

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// AxonASP permits Count() as well as Count on built-in collections. Normalize
// only known host collection paths, not arbitrary application Count methods.
func (m *MappedFile) rewriteCollectionCountCalls() {
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: tspath.RootedFilePathFromNormalized("/__asp_count.js")}, m.Text, core.ScriptKindJS)
	paths := map[string]bool{"Request.QueryString.Count": true, "Request.Form.Count": true, "Request.ServerVariables.Count": true, "Request.Cookies.Count": true, "Response.Cookies.Count": true}
	var edits []sourceEdit
	var visit ast.Visitor
	visit = func(n *ast.Node) bool {
		if n.Kind == ast.KindCallExpression {
			c := n.AsCallExpression()
			if c.Expression.Kind == ast.KindPropertyAccessExpression && len(c.Arguments.Nodes) == 0 && c.QuestionDotToken == nil {
				var name func(*ast.Node) string
				name = func(n *ast.Node) string {
					if ast.IsIdentifier(n) {
						return n.Text()
					}
					if n.Kind == ast.KindPropertyAccessExpression {
						p := n.AsPropertyAccessExpression()
						return name(p.Expression) + "." + p.Name().Text()
					}
					return ""
				}
				if paths[name(c.Expression)] {
					start := c.Expression.End()
					s := scanner.NewScanner()
					s.SetText(m.Text)
					s.ResetPos(start)
					if s.Scan() == ast.KindOpenParenToken {
						open := s.TokenStart()
						if s.Scan() == ast.KindCloseParenToken {
							edits = append(edits, sourceEdit{open, open + 1, " "}, sourceEdit{s.TokenStart(), s.TokenEnd(), strings.Repeat(" ", s.TokenEnd()-s.TokenStart())})
						}
					}
				}
			}
		}
		n.ForEachChild(visit)
		return false
	}
	file.ForEachChild(visit)
	m.applyEdits(edits)
}
