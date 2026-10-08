package asp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// SourceType annotates a variable in checker input only. Project contracts stay
// outside generic ASP declarations; initializers are still checked against them.
type SourceType struct {
	Source   string `json:"source"`
	Variable string `json:"variable"`
	Type     string `json:"type"`
	Function string `json:"function,omitempty"`
	Variadic bool   `json:"variadic,omitempty"`
	JSDoc    string `json:"jsdoc,omitempty"`
}

func (m *MappedFile) applySourceTypes(config, root string) error {
	if config == "" {
		return nil
	}
	if root == "" {
		return fmt.Errorf("--source-types requires --root")
	}
	f, err := os.Open(config)
	if err != nil {
		return err
	}
	defer f.Close()
	var rules []SourceType
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err = d.Decode(&rules); err != nil {
		return fmt.Errorf("source types: %w", err)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	for _, r := range rules {
		path := filepath.Clean(filepath.FromSlash(r.Source))
		if r.Source == "" || (r.Variable == "" && r.Function == "") || (r.Variable != "" && r.Function != "") || (r.Variable != "" && r.Type == "") || (r.Function != "" && !r.Variadic && r.JSDoc == "") || (r.Variable != "" && r.JSDoc != "") || strings.Contains(r.Type, "*/") || strings.ContainsAny(r.Type, "\r\n") || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid source-type rule: %+v", r)
		}
		if r.JSDoc != "" && !validSourceJSDoc(r.JSDoc) {
			return fmt.Errorf("source-type jsdoc must contain only complete JSDoc comments: %s", r.Function)
		}
	}
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: tspath.RootedFilePathFromNormalized("/__asp_types.js")}, m.Text, core.ScriptKindJS)
	var edits []sourceEdit
	restName := "__aspRestArguments"
	for strings.Contains(m.Text, restName) {
		restName += "_"
	}
	var visit ast.Visitor
	visit = func(n *ast.Node) bool {
		if n.Kind == ast.KindExpressionStatement {
			expr := n.AsExpressionStatement().Expression
			if expr.Kind == ast.KindBinaryExpression {
				b := expr.AsBinaryExpression()
				if b.OperatorToken.Kind == ast.KindEqualsToken && b.Right.Kind == ast.KindFunctionExpression {
					start := scanner.GetTokenPosOfNode(n, file, false)
					original, _, _ := m.Position(start)
					var memberPath func(*ast.Node) string
					memberPath = func(node *ast.Node) string {
						if ast.IsIdentifier(node) {
							return node.Text()
						}
						if node.Kind == ast.KindPropertyAccessExpression {
							p := node.AsPropertyAccessExpression()
							return memberPath(p.Expression) + "." + p.Name().Text()
						}
						return ""
					}
					for _, r := range rules {
						if r.JSDoc != "" && r.Function == memberPath(b.Left) && strings.EqualFold(filepath.Clean(original), filepath.Join(absRoot, filepath.FromSlash(r.Source))) {
							edits = append(edits, sourceEdit{start, start, "\n" + r.JSDoc + "\n"})
						}
					}
				}
			}
		}
		if n.Kind == ast.KindFunctionDeclaration || n.Kind == ast.KindPropertyAssignment {
			start := scanner.GetTokenPosOfNode(n, file, false)
			original, _, _ := m.Position(start)
			for _, r := range rules {
				if r.Function != "" && n.Name() != nil && n.Name().Text() == r.Function && strings.EqualFold(filepath.Clean(original), filepath.Join(absRoot, filepath.FromSlash(r.Source))) {
					fn := n
					if n.Kind == ast.KindPropertyAssignment {
						fn = n.AsPropertyAssignment().Initializer
					}
					if !ast.IsFunctionLike(fn) {
						continue
					}
					if r.JSDoc != "" {
						edits = append(edits, sourceEdit{start, start, "\n" + r.JSDoc + "\n"})
					}
					if !r.Variadic {
						continue
					}
					params := fn.ParameterList()
					if len(params.Nodes) > 0 && params.Nodes[len(params.Nodes)-1].AsParameterDeclaration().DotDotDotToken != nil {
						continue
					}
					// A virtual rest parameter changes no production source. Preserve
					// existing named parameters and their typing; only extra arguments
					// gain the legacy variadic contract.
					position := params.End()
					prefix := ""
					if len(params.Nodes) > 0 {
						prefix = ","
					}
					edits = append(edits, sourceEdit{position, position, prefix + "..." + restName})
				}
			}
		}
		if n.Kind == ast.KindVariableDeclaration && ast.IsIdentifier(n.Name()) {
			start := scanner.GetTokenPosOfNode(n, file, false)
			original, _, _ := m.Position(start)
			for _, r := range rules {
				if n.Name().Text() == r.Variable && strings.EqualFold(filepath.Clean(original), filepath.Join(absRoot, filepath.FromSlash(r.Source))) {
					edits = append(edits, sourceEdit{start, start, "/** @type {" + r.Type + "} */ "})
				}
			}
		}
		n.ForEachChild(visit)
		return false
	}
	file.ForEachChild(visit)
	m.applyEdits(edits)
	return nil
}

// Reject executable text and unterminated comments before inserting annotations.
func validSourceJSDoc(text string) bool {
	remaining := strings.TrimSpace(text)
	count := 0
	for remaining != "" {
		if !strings.HasPrefix(remaining, "/**") {
			return false
		}
		end := strings.Index(remaining[3:], "*/")
		if end < 0 {
			return false
		}
		remaining = strings.TrimSpace(remaining[3+end+2:])
		count++
	}
	return count > 0
}
