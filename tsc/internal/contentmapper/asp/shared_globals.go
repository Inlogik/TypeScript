package asp

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// Shared-global lint uses resolved symbols, so parameters/locals/shadowing are
// not confused with page globals. Companions acknowledge a dependency only for
// their corresponding source, even though types participate in the whole program.
func sharedGlobalDiagnostics(ctx context.Context, p *compiler.Program, m *MappedFile, virtual tspath.RootedFilePath, companions []string) []Diagnostic {
	file := p.GetSourceFile(virtual)
	if file == nil {
		return nil
	}
	c, release := p.GetTypeCheckerForFile(ctx, file)
	defer release()
	contracts := map[string]map[string]bool{}
	for _, companion := range companions {
		sf := p.GetSourceFile(tspath.RootedFilePathFromAbsolute(companion))
		if sf == nil {
			continue
		}
		names := map[string]bool{}
		var visit ast.Visitor
		visit = func(n *ast.Node) bool {
			if n.Kind == ast.KindVariableDeclaration && ast.IsIdentifier(n.Name()) {
				names[n.Name().Text()] = true
			}
			n.ForEachChild(visit)
			return false
		}
		sf.ForEachChild(visit)
		contracts[strings.ToLower(filepath.Clean(strings.TrimSuffix(companion, ".d.ts")))] = names
	}
	var result []Diagnostic
	var walk func(*ast.Node, *ast.Node)
	walk = func(n, parent *ast.Node) {
		if ast.IsIdentifier(n) {
			// Declaration names and named property keys are not variable reads.
			isName := parent != nil && parent.Name() == n && parent.Kind != ast.KindShorthandPropertyAssignment
			if !isName {
				symbol := c.GetSymbolAtLocation(n)
				if symbol != nil {
					pos := scanner.GetTokenPosOfNode(n, file, false)
					source, line, column := m.Position(pos)
					local := false
					variable := false
					providers := map[string]bool{}
					for _, decl := range symbol.Declarations {
						if ast.GetSourceFileOfNode(decl) != file {
							continue
						} // host/project globals allowed
						if decl.Kind != ast.KindVariableDeclaration && decl.Kind != ast.KindBindingElement {
							continue
						}
						global := true
						for ancestor := decl.Parent; ancestor != nil; ancestor = ancestor.Parent {
							if ast.IsFunctionLike(ancestor) {
								global = false
								break
							}
							if ancestor.Kind == ast.KindVariableDeclarationList && ancestor.Flags&(ast.NodeFlagsLet|ast.NodeFlagsConst) != 0 {
								global = false
								break
							}
						}
						if !global {
							local = true
							continue
						}
						variable = true
						provider, _, _ := m.Position(scanner.GetTokenPosOfNode(decl, file, false))
						if strings.EqualFold(filepath.Clean(provider), filepath.Clean(source)) {
							local = true
						} else if provider != "" {
							providers[provider] = true
						}
					}
					acknowledged := contracts[strings.ToLower(filepath.Clean(source))][n.Text()]
					if variable && !local && !acknowledged && source != "" && len(providers) > 0 {
						names := []string{}
						for name := range providers {
							names = append(names, name)
						}
						sort.Strings(names)
						access := "read"
						if parent != nil && (parent.Kind == ast.KindPostfixUnaryExpression || parent.Kind == ast.KindPrefixUnaryExpression) {
							operator := ast.KindUnknown
							if parent.Kind == ast.KindPostfixUnaryExpression {
								operator = parent.AsPostfixUnaryExpression().Operator
							} else {
								operator = parent.AsPrefixUnaryExpression().Operator
							}
							if operator == ast.KindPlusPlusToken || operator == ast.KindMinusMinusToken {
								access = "read/write"
							}
						}
						if parent != nil && parent.Kind == ast.KindBinaryExpression && parent.AsBinaryExpression().Left == n {
							op := parent.AsBinaryExpression().OperatorToken.Kind
							if op == ast.KindEqualsToken {
								access = "write"
							} else if op >= ast.KindFirstCompoundAssignment && op <= ast.KindLastCompoundAssignment {
								access = "read/write"
							}
						}
						endFile, endLine, endColumn := m.Position(n.End())
						if endFile != source {
							endLine, endColumn = line, column
						}
						result = append(result, Diagnostic{File: source, Line: line, Column: column, EndLine: endLine, EndColumn: endColumn, Code: 90001, Warning: true, Message: "Shared-global " + access + ": '" + n.Text() + "' is supplied by " + strings.Join(names, ", ") + ". Declare this dependency in " + filepath.Base(source) + ".d.ts or pass it explicitly."})
					}
				}
			}
		}
		n.ForEachChild(func(child *ast.Node) bool { walk(child, n); return false })
	}
	file.ForEachChild(func(n *ast.Node) bool { walk(n, nil); return false })
	return result
}
