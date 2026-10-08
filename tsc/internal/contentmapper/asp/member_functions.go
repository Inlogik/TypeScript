package asp

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// rewriteMemberFunctions identifies recovered function declarations in the native
// parser before examining their tokens. This avoids rewriting strings, comments,
// regex literals, or text inside templates. No compiler grammar is modified.
func (m *MappedFile) rewriteMemberFunctions() {
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: tspath.RootedFilePathFromNormalized("/__asp_members.js")}, m.Text, core.ScriptKindJS)
	var edits []sourceEdit
	var visit ast.Visitor
	visit = func(n *ast.Node) bool {
		if n.Kind == ast.KindFunctionDeclaration {
			start := scanner.GetTokenPosOfNode(n, file, false)
			s := scanner.NewScanner()
			s.SetText(m.Text)
			s.ResetPos(start)
			if s.Scan() == ast.KindFunctionKeyword {
				keywordEnd := s.TokenEnd()
				if s.Scan() == ast.KindIdentifier {
					nameEnd := s.TokenEnd()
					members := 0
					for s.Scan() == ast.KindDotToken {
						if s.Scan() != ast.KindIdentifier {
							break
						}
						members++
						nameEnd = s.TokenEnd()
					}
					if members > 0 && s.Token() == ast.KindOpenParenToken {
						// Keep the original member name and JSDoc adjacent to the
						// assignment. Only syntax glue is synthesized.
						edits = append(edits, sourceEdit{start, keywordEnd, ""}, sourceEdit{nameEnd, nameEnd, " = function"})
					}
				}
			}
		}
		n.ForEachChild(visit)
		return false
	}
	file.ForEachChild(visit)
	converted := map[int]bool{}
	delta := 0
	for _, edit := range edits {
		if edit.start == edit.end {
			converted[edit.start+delta+3] = true
		}
		delta += len(edit.text) - (edit.end - edit.start)
	}
	m.applyEdits(edits)
	if len(edits) == 0 {
		return
	}
	// Assignment statements need an explicit semicolon: otherwise a following
	// IIFE or bracket expression may attach to the function expression.
	file = parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: tspath.RootedFilePathFromNormalized("/__asp_members.js")}, m.Text, core.ScriptKindJS)
	edits = nil
	visit = func(n *ast.Node) bool {
		if n.Kind == ast.KindFunctionExpression && converted[scanner.GetTokenPosOfNode(n, file, false)] {
			edits = append(edits, sourceEdit{n.End(), n.End(), ";"})
		}
		n.ForEachChild(visit)
		return false
	}
	file.ForEachChild(visit)
	m.applyEdits(edits)
}
