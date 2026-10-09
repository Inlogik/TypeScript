package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigScanAndOutDirImplyMixedBuild(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "site")
	dest := filepath.Join(root, "build")
	os.MkdirAll(src, 0700)
	os.WriteFile(filepath.Join(src, "plain.asp"), []byte(`<%@ Language=JScript %><% Response.Write('ok'); %>`), 0600)
	os.WriteFile(filepath.Join(src, "typed.asp.ts"), []byte(`<%@ Language=JScript %><% var value:number=1; %>`), 0600)
	project := filepath.Join(root, "asp-check.json")
	os.WriteFile(project, []byte(`{"version":1,"root":"site","scan":"site","types":[]}`), 0600)
	var out bytes.Buffer
	if code := run([]string{"--project", project, "--out-dir", dest}, &out); code != 0 {
		t.Fatalf("%d %s", code, &out)
	}
	for _, name := range []string{"plain.asp", "typed.asp"} {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Fatal(err)
		}
	}
	out.Reset()
	if code := run([]string{"--project", project, "--json", filepath.Join(src, "plain.asp")}, &out); code != 0 {
		t.Fatalf("explicit entry became scan: %d %s", code, &out)
	}
	if bytes.Contains(out.Bytes(), []byte(`"pages"`)) {
		t.Fatal("unexpected batch result")
	}
}
