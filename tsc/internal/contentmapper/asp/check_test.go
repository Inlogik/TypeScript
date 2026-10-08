package asp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckJSDocAndOriginalLocations(t *testing.T) {
	diags, err := Check(context.Background(), "testdata/default.asp", "testdata")
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 2 {
		t.Fatalf("expected two deliberate errors: %+v", diags)
	}
	want := map[string][2]int{"DoesNotExist": {9, 18}, "Banana": {4, 21}}
	for _, d := range diags {
		key := ""
		for name := range want {
			if strings.Contains(d.Message, "'"+name+"'") {
				key = name
				break
			}
		}
		pos, ok := want[key]
		file := "default.asp"
		if key == "Banana" {
			file = "helpers.asp"
		}
		if !ok || filepath.Base(d.File) != file || d.Code != 2339 || d.Line != pos[0] || d.Column != pos[1] {
			t.Fatalf("unexpected diagnostic: %+v", d)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatal(want)
	}
}

func writeFixture(t *testing.T, root, name, text string) string {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCleanPageAndCrossBlockControlFlow(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "valid.asp", "<%@ Language=JScript %><% var x=1; if (x) { %><h1><%= x %></h1><% } %>")
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 0 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestUntypedLegacyCodeRemainsLoose(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "loose.asp", "<%@ Language=JScript %><% function f(x) { try { return x; } catch(e) { return e.message; } } %>")
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 0 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestIncludeFragmentsAndRepeatedIncludes(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "start.inc", "<% /** @type {number} */ var n =")
	writeFixture(t, root, "end.inc", " 'wrong'; %>")
	page := writeFixture(t, root, "page.asp", "<%@ Language=JScript %><!-- #include file=\"START.INC\" --><!-- #include file=\"end.inc\" -->")
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || !strings.EqualFold(filepath.Base(d[0].File), "start.inc") || d[0].Code != 2322 {
		t.Fatalf("%v %+v", err, d)
	}
	writeFixture(t, root, "empty.inc", "<p>static</p>")
	page = writeFixture(t, root, "repeat.asp", `<!-- #include file="empty.inc" --><!-- #include file="empty.inc" -->`)
	if _, err = Map(page, root); err != nil {
		t.Fatal(err)
	}
}

func TestUnsupportedAndInvalidInputs(t *testing.T) {
	for _, tc := range []struct{ name, text, error string }{
		{"vbscript", "<% Dim x %>", "unsupported server language"},
		{"truncated", "<%@ Language=JScript %><% var x=1;", "unterminated"},
		{"missing", `<!-- #include file="missing.inc" -->`, "include not found"},
		{"escape", `<!-- #include virtual="/../outside.inc" -->`, "escapes root"},
		{"cycle", `<!-- #include file="page.asp" -->`, "circular include"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			page := writeFixture(t, root, "page.asp", tc.text)
			if _, err := Map(page, root); err == nil || !strings.Contains(err.Error(), tc.error) {
				t.Fatalf("expected %s, got %v", tc.error, err)
			}
		})
	}
}

func TestUnicodeCRLFAndBOMMapping(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "unicode.asp", "\uFEFF<%@ Language=JScript %>\r\n<% /** @type {{Name:string}} */ var c={Name:'😀'}; c.Missing; %>")
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || d[0].Line != 2 || d[0].Column != 54 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestSyntaxDiagnosticAndNoSourceMutation(t *testing.T) {
	root := t.TempDir()
	text := "<%@ Language=JScript %>\n<% var n = ; %>"
	page := writeFixture(t, root, "syntax.asp", text)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || d[0].Code != 1109 || d[0].Line != 2 || d[0].Column != 12 {
		t.Fatalf("%v %+v", err, d)
	}
	bytes, err := os.ReadFile(page)
	if err != nil || string(bytes) != text {
		t.Fatalf("source changed: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected files emitted: %v %v", entries, err)
	}
}

func TestNestedIncludeMappingAndVirtualRootRequired(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "parts/outer.inc", `<!-- #include file="inner.inc" -->`)
	writeFixture(t, root, "parts/inner.inc", "<% /** @type {number} */ var n='wrong'; %>")
	page := writeFixture(t, root, "nested.asp", "<%@ Language=JScript %><!-- #include virtual=\"/parts/outer.inc\" -->")
	if _, err := Map(page, ""); err == nil || !strings.Contains(err.Error(), "requires --root") {
		t.Fatalf("%v", err)
	}
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || filepath.Base(d[0].File) != "inner.inc" || d[0].Line != 1 || d[0].Column != 30 {
		t.Fatalf("%v %+v", err, d)
	}
}
