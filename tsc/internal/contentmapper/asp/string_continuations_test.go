package asp

import (
	"context"
	"strings"
	"testing"
)

func TestStringContinuationsAndMapping(t *testing.T) {
	for _, separator := range []string{"\\ \r\n", "\\\t\n", "\\\f\v \r\n"} {
		root := t.TempDir()
		page := writeFixture(t, root, "continued.asp", "<%@ Language=JScript %>\n<% var s=\"first "+separator+"second\"; Response.YouWrite(s); %>")
		d, err := Check(context.Background(), page, root)
		if err != nil || len(d) != 1 || d[0].Code != 2339 || d[0].Line != 3 || d[0].Column != 19 {
			t.Fatalf("%q: %v %+v", separator, err, d)
		}
	}
}

func TestContinuationCommentsAndEscapesUnchanged(t *testing.T) {
	text := "// 'comment \\ \n/* \"comment \\ \n */\nvar s='escaped \\\\ ';"
	m := &MappedFile{Text: text}
	m.normalizeStringContinuations()
	if m.Text != text {
		t.Fatalf("changed non-continuations: %q", m.Text)
	}
	if strings.Contains(m.Text, "__asp") {
		t.Fatal(m.Text)
	}
}
