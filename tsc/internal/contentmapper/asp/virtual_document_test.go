package asp

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestVirtualDocumentUTF16MappingAndInference(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "page.asp", "<%@ Language=JScript %><% var value='😀'; Response.Write(value); %>")
	r := PrepareVirtual(page, root, nil, "")
	if r.Error != "" || strings.Contains(r.Text, "@type {any}") || !strings.Contains(r.Declarations, "interface ASPResponse") {
		t.Fatalf("%+v", r)
	}
	pos := strings.Index(r.Text, "Response.Write")
	original := strings.Index(r.Sources[page], "Response.Write")
	found := false
	for _, s := range r.Spans {
		p := utf16Length(r.Text[:pos])
		if s.Start <= p && p < s.End {
			if s.OriginalStart+p-s.Start != utf16Length(r.Sources[page][:original]) {
				t.Fatal(s)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing mapping")
	}
}

func TestVirtualDocumentNormalizesCRLFBeforeMapping(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "crlf.asp", "<%@ Language=JScript %>\r\n<%\r\nvar text='😀';\r\nResponse.Write(text);\r\n%>\r\n")
	r := PrepareVirtual(page, root, nil, "")
	if r.Error != "" || strings.Contains(r.Text, "\r") {
		t.Fatalf("not uniform LF: %+v", r)
	}
	pos := utf16Length(r.Text[:strings.Index(r.Text, "Response.Write")])
	original := utf16Length(r.Sources[page][:strings.Index(r.Sources[page], "Response.Write")])
	found := false
	for _, s := range r.Spans {
		if s.Start <= pos && pos < s.End {
			if s.OriginalStart+pos-s.Start != original {
				t.Fatalf("wrong normalized mapping: %+v", s)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("no Response span")
	}
}

func TestVirtualOverlayUsesUnsavedEntryAndIncludes(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="part.inc" --><% Response.Write('disk'); %>`)
	part := writeFixture(t, root, "part.inc", "<% var value='disk'; %>")
	overlay := writeFixture(t, root, "overlay.json", "{}")
	data, err := json.Marshal(map[string]string{page: `<%@ Language=JScript %><!-- #include file="part.inc" --><% value. %>`, part: "<% var value={Name:'unsaved'}; %>"})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(overlay, data, 0600); err != nil {
		t.Fatal(err)
	}
	r := PrepareVirtualWithOverlay(page, root, nil, "", overlay)
	if r.Error != "" || !strings.Contains(r.Text, "value.") || !strings.Contains(r.Text, "Name:'unsaved'") || strings.Contains(r.Text, "'disk'") {
		t.Fatalf("%+v", r)
	}
}
