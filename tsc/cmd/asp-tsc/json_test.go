package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/contentmapper/asp"
)

func TestJSONDiagnosticsAndInputFailures(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "page.asp")
	if err := os.WriteFile(page, []byte("<%@ Language=JScript %>\n<% Response.YouWrite('x'); %>"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := run([]string{"--json", page}, &out); code != 1 {
		t.Fatal(code)
	}
	var r asp.Report
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Version != 1 || len(r.Dependencies) != 1 || len(r.Diagnostics) != 1 || r.Diagnostics[0].Line != 2 || r.Diagnostics[0].EndColumn <= r.Diagnostics[0].Column {
		t.Fatalf("%+v", r)
	}
	out.Reset()
	if code := run([]string{"--json", filepath.Join(root, "missing.asp")}, &out); code != 2 {
		t.Fatal(code)
	}
	if err := json.Unmarshal(out.Bytes(), &r); err != nil || r.Error == "" {
		t.Fatalf("%v %+v", err, r)
	}
}
