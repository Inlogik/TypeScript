package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMixedBuildEmitsTypedIncludeForPlainCaller(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dest := filepath.Join(root, "build")
	os.MkdirAll(filepath.Join(src, "includes"), 0700)
	page := `<%@ Language=JScript %><!-- #include file="includes/helper.inc" --><% Response.Write(label(1)); %>`
	os.WriteFile(filepath.Join(src, "page.asp"), []byte(page), 0600)
	os.WriteFile(filepath.Join(src, "includes/helper.inc.ts"), []byte(`<% function label(value:number):string{return String(value);} %>`), 0600)
	var out bytes.Buffer
	if code := run([]string{"--emit-asp", "--scan", src, "--root", src, "--out-dir", dest}, &out); code != 0 {
		t.Fatalf("%d %s", code, &out)
	}
	data, err := os.ReadFile(filepath.Join(dest, "page.asp"))
	if err != nil || string(data) != page {
		t.Fatalf("caller changed: %v %s", err, data)
	}
	data, err = os.ReadFile(filepath.Join(dest, "includes/helper.inc"))
	if err != nil || strings.Contains(string(data), ":number") || !strings.Contains(string(data), "label(value)") {
		t.Fatalf("include not erased: %v %s", err, data)
	}
}
