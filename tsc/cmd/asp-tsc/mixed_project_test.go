package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMixedBuildUsesProjectSourceContracts(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "site")
	dest := filepath.Join(root, "build")
	os.MkdirAll(src, 0700)
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("site/page.asp", `<%@ Language=JScript %><% var formatter=function(value){return String(value);};formatter.masks={default:'x'};Response.Write(formatter.masks.default); %>`)
	write("types.d.ts", `interface Formatter {(value?:any):string;masks?:{[key:string]:string};}`)
	write("source.json", `[{"source":"page.asp","variable":"formatter","type":"Formatter"}]`)
	write("project.json", `{"version":1,"root":"site","types":["types.d.ts"],"sourceTypes":"source.json","looseVariables":true}`)
	var out bytes.Buffer
	if code := run([]string{"--emit-asp", "--project", filepath.Join(root, "project.json"), "--scan", src, "--out-dir", dest}, &out); code != 0 {
		t.Fatalf("%d %s", code, &out)
	}
	if _, err := os.Stat(filepath.Join(dest, "page.asp")); err != nil {
		t.Fatal(err)
	}
}
