package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMixedBuildCopiesRuntimeFilesAndExcludesDeclarations(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dest := filepath.Join(root, "build")
	os.MkdirAll(filepath.Join(src, "includes"), 0700)
	files := map[string]string{"plain.asp": "<%@ Language=JScript %><% Response.Write('ok'); %>", "includes/header.inc": "<h1>Hello</h1>", "includes/header.inc.d.ts": "interface Header {}", "asset.bin": "\x00\xff\x01", "typed.asp.ts": "<%@ Language=JScript %><% var x:number=1; %>"}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(src, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if code := run([]string{"--emit-asp", "--scan", src, "--out-dir", dest}, &out); code != 0 {
		t.Fatalf("%d %s", code, &out)
	}
	for _, name := range []string{"plain.asp", "includes/header.inc", "asset.bin"} {
		data, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil || string(data) != files[name] {
			t.Fatalf("copy changed %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "includes/header.inc.d.ts")); !os.IsNotExist(err) {
		t.Fatal("declaration deployed")
	}
	if _, err := os.Stat(filepath.Join(dest, "typed.asp")); err != nil {
		t.Fatal(err)
	}
}

func TestMixedBuildReportsExistingPageErrorsButCopies(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dest := filepath.Join(root, "build")
	os.MkdirAll(src, 0700)
	input := `<%@ Language=JScript %><% Response.YouWrite('bad'); %>`
	os.WriteFile(filepath.Join(src, "page.asp"), []byte(input), 0600)
	var out bytes.Buffer
	if code := run([]string{"--emit-asp", "--scan", src, "--out-dir", dest, "--errors-only"}, &out); code != 1 {
		t.Fatalf("%d %s", code, &out)
	}
	if !bytes.Contains(out.Bytes(), []byte("YouWrite")) {
		t.Fatal("missing diagnostics")
	}
	data, err := os.ReadFile(filepath.Join(dest, "page.asp"))
	if err != nil || string(data) != input {
		t.Fatalf("copy missing/changed: %v", err)
	}
}

func TestMixedBuildCollisionAndEnumBlockAllFiles(t *testing.T) {
	for _, collision := range []bool{false, true} {
		root := t.TempDir()
		src := filepath.Join(root, "src")
		dest := filepath.Join(root, "build")
		os.MkdirAll(src, 0700)
		os.WriteFile(filepath.Join(src, "plain.asp"), []byte("plain"), 0600)
		codeText := "<%@ Language=JScript %><% enum E { A } %>"
		if collision {
			codeText = "<%@ Language=JScript %><% var x:number=1; %>"
			os.WriteFile(filepath.Join(src, "bad.asp"), []byte("existing"), 0600)
		}
		os.WriteFile(filepath.Join(src, "bad.asp.ts"), []byte(codeText), 0600)
		var out bytes.Buffer
		code := run([]string{"--emit-asp", "--scan", src, "--out-dir", dest}, &out)
		if code == 0 {
			t.Fatal("invalid build passed")
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Fatalf("partial output: %s", out.String())
		}
	}
}
