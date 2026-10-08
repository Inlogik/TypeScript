package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBatchKeepsPageScopesAndAggregatesSharedDiagnostics(t *testing.T) {
	root := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("shared.inc", `<% Response.YouWrite('bad'); %>`)
	write("a.asp", `<%@ Language=JScript %><!-- #include file="shared.inc" --><% function onlyA(){} %>`)
	write("b.ASP", `<%@ Language=JScript %><!-- #include file="shared.inc" --><% onlyA(); %>`)
	write("clean.asp", `<%@ Language=JScript %><% Response.Write('ok'); %>`)
	var out bytes.Buffer
	if code := run([]string{"--scan", root, "--jobs", "2", "--json"}, &out); code != 1 {
		t.Fatalf("%d %s", code, out.String())
	}
	var r batchResult
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Pages) != 3 || r.Clean != 1 || r.WithErrors != 2 || r.Failed != 0 || r.Errors != 2 {
		t.Fatalf("%+v", r)
	}
	shared := false
	for _, d := range r.Diagnostics {
		if d.Code == 2339 {
			shared = len(d.Entries) == 2
		}
	}
	if !shared {
		t.Fatal("shared diagnostic lost affected entries")
	}
}

func TestBatchFailureAndWarningsOnlyExitCodes(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "page.asp")
	if err := os.WriteFile(page, []byte(`<%@ Language=JScript %><% implicit=1; %>`), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := run([]string{"--scan", root, "--errors-only"}, &out); code != 0 {
		t.Fatalf("%d %s", code, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("1 clean")) || !bytes.Contains(out.Bytes(), []byte("1 warnings")) {
		t.Fatal(out.String())
	}
	if err := os.WriteFile(page, []byte(`<%@ Language=JScript %><!-- #include file="missing.inc" -->`), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"--scan", root, "--json"}, &out); code != 2 {
		t.Fatalf("%d %s", code, out.String())
	}
	var r batchResult
	if err := json.Unmarshal(out.Bytes(), &r); err != nil || r.Failed != 1 {
		t.Fatalf("%v %+v", err, r)
	}
}

func TestProjectDefaultEntryAndExplicitOverrides(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "page.asp")
	if err := os.WriteFile(page, []byte(`<%@ Language=JScript %><% var value='text';value=1; %>`), 0600); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "project.json")
	if err := os.WriteFile(project, []byte(`{"version":1,"root":".","entry":"page.asp","types":[],"looseVariables":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := run([]string{"--project", project, "--errors-only"}, &out); code != 0 {
		t.Fatalf("%d %s", code, out.String())
	}
	out.Reset()
	if code := run([]string{"--project", project, "--loose-variables=false"}, &out); code != 1 {
		t.Fatalf("%d %s", code, out.String())
	}
}
