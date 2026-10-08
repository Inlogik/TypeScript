package asp

// normalizeStringContinuations matches AxonASP's pre-parser compatibility rule:
// horizontal whitespace may follow a continuation backslash inside quoted
// strings. Only that whitespace is removed; original source spans are composed
// through applyEdits. Comments and escaped quotes are left intact.
func (m *MappedFile) normalizeStringContinuations() {
	const (
		code = iota
		single
		double
		lineComment
		blockComment
	)
	state := code
	var edits []sourceEdit
	for i := 0; i < len(m.Text); i++ {
		ch := m.Text[i]
		switch state {
		case code:
			if ch == '\'' {
				state = single
			} else if ch == '"' {
				state = double
			} else if ch == '/' && i+1 < len(m.Text) {
				if m.Text[i+1] == '/' {
					state = lineComment
					i++
				} else if m.Text[i+1] == '*' {
					state = blockComment
					i++
				}
			}
		case single, double:
			quote := byte('\'')
			if state == double {
				quote = '"'
			}
			if ch == quote {
				state = code
			} else if ch == '\\' {
				j := i + 1
				for j < len(m.Text) && (m.Text[j] == ' ' || m.Text[j] == '\t' || m.Text[j] == '\f' || m.Text[j] == '\v') {
					j++
				}
				if j < len(m.Text) && (m.Text[j] == '\r' || m.Text[j] == '\n') {
					if j > i+1 {
						edits = append(edits, sourceEdit{i + 1, j, ""})
					}
					i = j - 1
				} else {
					i++
				}
			}
		case lineComment:
			if ch == '\r' || ch == '\n' {
				state = code
			}
		case blockComment:
			if ch == '*' && i+1 < len(m.Text) && m.Text[i+1] == '/' {
				state = code
				i++
			}
		}
	}
	m.applyEdits(edits)
}
