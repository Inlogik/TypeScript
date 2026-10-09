package asp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf16"
	"unicode/utf8"
)

type VirtualSpan struct {
	Start         int    `json:"start"`
	End           int    `json:"end"`
	File          string `json:"file"`
	OriginalStart int    `json:"originalStart"`
}

// VirtualDocument uses UTF-16 offsets for VS Code, unlike the native compiler's
// byte offsets. Declarations are a separate TS document, not invalid JS prefixes.
type VirtualDocument struct {
	Version            int               `json:"version"`
	Entry              string            `json:"entry"`
	Text               string            `json:"text"`
	Declarations       string            `json:"declarations"`
	DeclarationSpans   []VirtualSpan     `json:"declarationSpans"`
	DeclarationSources map[string]string `json:"declarationSources"`
	Spans              []VirtualSpan     `json:"spans"`
	Sources            map[string]string `json:"sources"`
	Error              string            `json:"error,omitempty"`
	ScriptKind         string            `json:"scriptKind"`
}

func utf16Length(text string) int { return len(utf16.Encode([]rune(text))) }

// Build once per source instead of converting every span prefix. Large include
// graphs contain thousands of spans; repeated prefix conversion was quadratic.
func utf16Offsets(text string) []int {
	positions := make([]int, len(text)+1)
	units := 0
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		for j := 0; j < size; j++ {
			positions[i+j] = units
		}
		units += utf16.RuneLen(r)
		i += size
		positions[i] = units
	}
	return positions
}

// PrepareVirtual keeps variable inference for IntelliSense, independently of
// diagnostic loose-variable mode. Explicit project annotations still apply.
func PrepareVirtual(entry, root string, typeFiles []string, sourceTypes string) VirtualDocument {
	return PrepareVirtualWithOverlay(entry, root, typeFiles, sourceTypes, "")
}

func PrepareVirtualWithOverlay(entry, root string, typeFiles []string, sourceTypes, overlayFile string) VirtualDocument {
	abs, err := filepath.Abs(entry)
	r := VirtualDocument{Version: 1, Entry: abs, Spans: []VirtualSpan{}, Sources: map[string]string{}}
	if err != nil {
		r.Error = err.Error()
		return r
	}
	var sources map[string]string
	if overlayFile != "" {
		data, err := os.ReadFile(overlayFile)
		if err != nil {
			r.Error = err.Error()
			return r
		}
		if err = json.Unmarshal(data, &sources); err != nil {
			r.Error = err.Error()
			return r
		}
	}
	m, err := mapCheckingSources(entry, root, sources)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	companions, err := m.companionFiles()
	if err != nil {
		r.Error = err.Error()
		return r
	}
	typeFiles = append(append([]string{}, typeFiles...), companions...)
	if err = m.applySourceTypes(sourceTypes, root); err != nil {
		r.Error = err.Error()
		return r
	}
	r.ScriptKind = "javascript"
	if m.hasTypedSource() {
		r.ScriptKind = "typescript"
	}
	m.defaultParameterTypes()
	// VS Code normalizes mixed line endings when opening a document. Emit a
	// uniform LF virtual document before calculating editor offsets, composing
	// deletions through the source map rather than using pre-normalization offsets.
	var newlines []sourceEdit
	for i := 0; i < len(m.Text); i++ {
		if m.Text[i] == '\r' {
			if i+1 < len(m.Text) && m.Text[i+1] == '\n' {
				newlines = append(newlines, sourceEdit{i, i + 1, ""})
			} else {
				newlines = append(newlines, sourceEdit{i, i + 1, "\n"})
			}
		}
	}
	m.applyEdits(newlines)
	r.Text, r.Sources, r.Declarations = m.Text, m.Sources, hostTypes
	r.DeclarationSpans = []VirtualSpan{}
	r.DeclarationSources = map[string]string{}
	for _, file := range typeFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			r.Error = err.Error()
			return r
		}
		absolute, err := filepath.Abs(file)
		if err != nil {
			r.Error = err.Error()
			return r
		}
		start := utf16Length(r.Declarations) + 1
		r.Declarations += "\n" + string(data)
		r.DeclarationSpans = append(r.DeclarationSpans, VirtualSpan{Start: start, End: start + utf16Length(string(data)), File: absolute, OriginalStart: 0})
		r.DeclarationSources[absolute] = string(data)
	}
	virtualOffsets := utf16Offsets(m.Text)
	originalOffsets := map[string][]int{}
	for file, text := range m.Sources {
		originalOffsets[file] = utf16Offsets(text)
	}
	for _, s := range m.Spans {
		r.Spans = append(r.Spans, VirtualSpan{virtualOffsets[s.Start], virtualOffsets[s.End], s.File, originalOffsets[s.File][s.OriginalStart]})
	}
	sort.SliceStable(r.Spans, func(i, j int) bool { return r.Spans[i].Start < r.Spans[j].Start })
	return r
}
