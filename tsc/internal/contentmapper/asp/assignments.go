package asp

import (
	"sort"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

type sourceEdit struct {
	start, end int
	text       string
}

// rewriteIndexedAssignments uses the native JS AST rather than matching text,
// so strings, comments, equality comparisons and nested expressions stay intact.
// Plain assignments to call-shaped indexed properties use a checker-only helper.
// Keeping the original getter call checks its target and index arguments without
// inventing a setter method/signature on arbitrary COM objects. Compound and
// update operators remain diagnostics rather than getting an unsafe rewrite.
func (m *MappedFile) rewriteIndexedAssignments() {
	m.rewriteIndexedAssignmentsForScript(core.ScriptKindJS)
}

func (m *MappedFile) rewriteIndexedAssignmentsForScript(kind core.ScriptKind) {
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: tspath.RootedFilePathFromNormalized("/__asp_rewrite.js")}, m.Text, kind)
	var edits []sourceEdit
	var visit ast.Visitor
	visit = func(n *ast.Node) bool {
		if n.Kind == ast.KindBinaryExpression {
			b := n.AsBinaryExpression()
			left := b.Left
			for left.Kind == ast.KindParenthesizedExpression {
				left = left.AsParenthesizedExpression().Expression
			}
			if b.OperatorToken.Kind == ast.KindEqualsToken && left.Kind == ast.KindCallExpression {
				call := left.AsCallExpression()
				if call.QuestionDotToken == nil && call.Flags&ast.NodeFlagsOptionalChain == 0 {
					equals := b.OperatorToken.End() - 1
					if equals >= 0 && m.Text[equals] == '=' {
						edits = append(edits, sourceEdit{scanner.GetTokenPosOfNode(b.Left, file, false), scanner.GetTokenPosOfNode(b.Left, file, false), "__aspSetIndexed("}, sourceEdit{equals, equals + 1, ",("}, sourceEdit{b.Right.End(), b.Right.End(), "))"})
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

func (m *MappedFile) applyEdits(edits []sourceEdit) {
	if len(edits) == 0 {
		return
	}
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var out strings.Builder
	var spans []Span
	copySource := func(start, end int) {
		base := out.Len()
		out.WriteString(m.Text[start:end])
		for _, s := range m.Spans {
			a, b := max(start, s.Start), min(end, s.End)
			if a < b {
				spans = append(spans, Span{base + a - start, base + b - start, s.File, s.OriginalStart + a - s.Start})
			}
		}
	}
	cursor := 0
	for _, edit := range edits {
		copySource(cursor, edit.start)
		out.WriteString(edit.text)
		cursor = edit.end
	}
	copySource(cursor, len(m.Text))
	m.Text, m.Spans = out.String(), spans
}
