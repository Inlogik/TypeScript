package asp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// DependencyRule is a project-owned conditional include contract. RequiredEntries
// is an explicit list of entries known to execute the dependent branch; those
// entries retain errors. This policy does not infer branch reachability.
type DependencyRule struct {
	Source          string   `json:"source"`
	Consumer        string   `json:"consumer"`
	Names           []string `json:"names"`
	Provider        string   `json:"provider"`
	Condition       string   `json:"condition"`
	RequiredEntries []string `json:"requiredEntries,omitempty"`
}

func loadDependencyRules(file, root string) ([]DependencyRule, error) {
	if file == "" {
		return nil, nil
	}
	if root == "" {
		return nil, fmt.Errorf("--dependency-rules requires --root")
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rules []DependencyRule
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err = d.Decode(&rules); err != nil {
		return nil, fmt.Errorf("dependency rules: %w", err)
	}
	for _, r := range rules {
		if r.Source == "" || r.Consumer == "" || len(r.Names) == 0 || r.Provider == "" || r.Condition == "" {
			return nil, fmt.Errorf("dependency rules require source, consumer, names, provider and condition")
		}
		for _, path := range append([]string{r.Source, r.Provider}, r.RequiredEntries...) {
			clean := filepath.Clean(filepath.FromSlash(path))
			if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("dependency rule paths must be root-relative: %s", path)
			}
		}
	}
	return rules, nil
}

func conditionalDependencyWarnings(file *ast.SourceFile, m *MappedFile, diags []*ast.Diagnostic, entry, root string, rules []DependencyRule) map[*ast.Diagnostic]string {
	result := map[*ast.Diagnostic]string{}
	if file == nil || len(rules) == 0 {
		return result
	}
	absRoot, _ := filepath.Abs(root)
	absEntry, _ := filepath.Abs(entry)
	same := func(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
	for _, r := range rules {
		source := filepath.Join(absRoot, filepath.FromSlash(r.Source))
		provider := filepath.Join(absRoot, filepath.FromSlash(r.Provider))
		skip := false
		for name := range m.Sources {
			if same(name, provider) {
				skip = true
			}
		}
		for _, name := range r.RequiredEntries {
			if same(absEntry, filepath.Join(absRoot, filepath.FromSlash(name))) {
				skip = true
			}
		}
		if skip {
			continue
		}
		for _, d := range diags {
			if d.File() != file || (d.Code() != 2304 && d.Code() != 2552) {
				continue
			}
			original, _, _ := m.Position(d.Pos())
			if !same(original, source) {
				continue
			}
			var walk func(*ast.Node, string)
			walk = func(n *ast.Node, consumer string) {
				if ast.IsFunctionLike(n) {
					consumer = ""
					if n.Kind == ast.KindFunctionDeclaration && n.Name() != nil {
						consumer = n.Name().Text()
					}
				}
				if consumer == r.Consumer && ast.IsIdentifier(n) && scanner.GetTokenPosOfNode(n, file, false) == d.Pos() {
					for _, name := range r.Names {
						if n.Text() == name {
							result[d] = fmt.Sprintf("Conditional include dependency: %s requires %s from %s when %s; provider is absent from this entry's include graph (reachability not inferred).", r.Consumer, name, r.Provider, r.Condition)
						}
					}
				}
				n.ForEachChild(func(child *ast.Node) bool { walk(child, consumer); return false })
			}
			file.ForEachChild(func(n *ast.Node) bool { walk(n, ""); return false })
		}
	}
	return result
}
