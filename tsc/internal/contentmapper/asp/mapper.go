// Package asp maps Classic ASP into check-only JavaScript. Source positions use
// UTF-8 byte offsets, matching the native compiler; displayed columns use UTF-16.
package asp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"g3pix.com.br/axonasp/v2/vbscript"
)

type Span struct {
	Start, End    int
	File          string
	OriginalStart int
}

type MappedFile struct {
	Text    string
	Spans   []Span
	Sources map[string]string
}

// Map expands file/virtual includes textually before scanning, so includes may
// contribute fragments of a script block. Root is required for virtual includes.
func Map(entry, root string) (*MappedFile, error) {
	return mapWithSources(entry, root, nil)
}

func mapWithSources(entry, root string, sources map[string]string) (*MappedFile, error) {
	entry, err := filepath.Abs(entry)
	if err != nil {
		return nil, err
	}
	if root != "" {
		root, err = filepath.Abs(root)
		if err != nil {
			return nil, err
		}
	}
	m := &MappedFile{Sources: map[string]string{}}
	var expanded strings.Builder
	var origins []Span
	var expand func(string, int, map[string]bool) error
	expand = func(file string, depth int, active map[string]bool) error {
		if depth > 32 {
			return fmt.Errorf("include recursion limit exceeded: %s", file)
		}
		key := strings.ToLower(file)
		if active[key] {
			return fmt.Errorf("circular include: %s", file)
		}
		active[key] = true
		defer delete(active, key)
		bytes, err := os.ReadFile(file)
		for name, text := range sources {
			if strings.EqualFold(filepath.Clean(name), filepath.Clean(file)) {
				bytes = []byte(text)
				err = nil
				break
			}
		}
		if err != nil {
			return err
		}
		text := string(bytes)
		if !utf8.ValidString(text) {
			return fmt.Errorf("%s: only UTF-8 ASP sources are supported", file)
		}
		m.Sources[file] = text
		appendSource := func(start, end int) {
			if start == end {
				return
			}
			origins = append(origins, Span{expanded.Len(), expanded.Len() + end - start, file, start})
			expanded.WriteString(text[start:end])
		}
		cursor := 0
		if strings.HasPrefix(text, "\uFEFF") {
			cursor = 3
		}
		// Match the runtime preprocessor: includes are recognized outside <% %>.
		for cursor < len(text) {
			script := strings.Index(text[cursor:], "<%")
			limit := len(text)
			if script >= 0 {
				limit = cursor + script
			}
			search := cursor
			found := false
			for search < limit {
				i := strings.Index(text[search:limit], "<!--")
				if i < 0 {
					break
				}
				start := search + i
				j := strings.Index(text[start:limit], "-->")
				if j < 0 {
					break
				}
				end := start + j + 3
				path, virtual, ok := vbscript.ParseASPIncludeComment(text[start:end])
				if !ok {
					search = start + 4
					continue
				}
				appendSource(cursor, start)
				resolved, err := resolve(file, path, virtual, root)
				if err != nil {
					return fmt.Errorf("%s: include %q: %w", file, path, err)
				}
				if err = expand(resolved, depth+1, active); err != nil {
					return err
				}
				cursor = end
				found = true
				break
			}
			if found {
				continue
			}
			appendSource(cursor, limit)
			cursor = limit
			if script < 0 {
				break
			}
			end := strings.Index(text[cursor:], "%>")
			if end < 0 {
				appendSource(cursor, len(text))
				break
			}
			end += cursor + 2
			appendSource(cursor, end)
			cursor = end
		}
		return nil
	}
	if err = expand(entry, 0, map[string]bool{}); err != nil {
		return nil, err
	}
	text := expanded.String()
	regions, err := vbscript.ScanASP(text)
	if err != nil {
		return nil, err
	}
	var out strings.Builder
	copyRegion := func(start, end int) {
		base := out.Len()
		out.WriteString(text[start:end])
		for _, s := range origins {
			a, b := max(start, s.Start), min(end, s.End)
			if a < b {
				m.Spans = append(m.Spans, Span{base + a - start, base + b - start, s.File, s.OriginalStart + a - s.Start})
			}
		}
	}
	for _, r := range regions {
		if r.Kind == "script" || r.Kind == "expression" {
			if !strings.EqualFold(r.Language, "jscript") && !strings.EqualFold(r.Language, "javascript") {
				return nil, fmt.Errorf("unsupported server language %q (declare Language=JScript)", r.Language)
			}
			if r.Kind == "expression" {
				out.WriteString("__aspWrite(")
			}
			copyRegion(r.CodeStart, r.CodeEnd)
			if r.Kind == "expression" {
				out.WriteString(");")
			}
			// Newline ends line comments; no extra semicolon between script blocks
			// because control-flow constructs can span HTML regions.
			out.WriteByte('\n')
		} else {
			for _, c := range text[r.Start:r.End] {
				if c == '\r' || c == '\n' {
					out.WriteRune(c)
				} else {
					out.WriteByte(' ')
				}
			}
		}
	}
	m.Text = out.String()
	m.normalizeStringContinuations()
	m.rewriteMemberFunctions()
	m.rewriteIndexedAssignments()
	m.rewriteDeleteIdentifiers()
	m.rewriteCollectionCountCalls()
	m.rewriteReservedIdentifiers()
	return m, nil
}

func resolve(file, path string, virtual bool, root string) (string, error) {
	path = strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	if path == "" {
		return "", fmt.Errorf("empty include path")
	}
	var candidate string
	if virtual || strings.HasPrefix(path, "/") {
		if root == "" {
			return "", fmt.Errorf("virtual include requires --root")
		}
		candidate = filepath.Join(root, filepath.FromSlash(strings.TrimLeft(path, "/")))
		rel, err := filepath.Rel(root, candidate)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("virtual include escapes root")
		}
	} else {
		if filepath.IsAbs(path) || strings.Contains(path, ":") {
			return "", fmt.Errorf("absolute file include is not allowed")
		}
		candidate = filepath.Join(filepath.Dir(file), filepath.FromSlash(path))
	}
	// Classic ASP paths are case-insensitive even when checking under Linux.
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	volume := filepath.VolumeName(candidate)
	current := volume + string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(candidate, current), string(filepath.Separator)) {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", err
		}
		match := ""
		for _, e := range entries {
			if strings.EqualFold(e.Name(), part) {
				if match != "" {
					return "", fmt.Errorf("ambiguous include case")
				}
				match = e.Name()
			}
		}
		if match == "" {
			return "", fmt.Errorf("include not found: %s", candidate)
		}
		current = filepath.Join(current, match)
	}
	return current, nil
}

// Position returns the original filename and one-based line/UTF-16 column.
// Synthetic punctuation is anchored to the nearest preceding source region.
func (m *MappedFile) Position(pos int) (string, int, int) {
	var chosen *Span
	for i := range m.Spans {
		s := &m.Spans[i]
		if s.Start > pos {
			break
		}
		chosen = s
		if pos < s.End {
			break
		}
	}
	if chosen == nil && len(m.Spans) > 0 {
		chosen = &m.Spans[0]
	}
	if chosen == nil {
		return "", 0, 0
	}
	offset := chosen.OriginalStart + max(0, min(pos-chosen.Start, chosen.End-chosen.Start))
	line, col := LineColumn(m.Sources[chosen.File], offset)
	return chosen.File, line, col
}

func LineColumn(text string, pos int) (int, int) {
	line, col := 1, 1
	for i := 0; i < min(pos, len(text)); {
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size
		if r == '\r' {
			if i < len(text) && text[i] == '\n' {
				i++
			}
			line++
			col = 1
		} else if r == '\n' {
			line++
			col = 1
		} else {
			col += utf16.RuneLen(r)
		}
	}
	return line, col
}
