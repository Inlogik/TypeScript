package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestBatchEmissionKeepsPathsChecksAndExclusiveOutput(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "generated")
	for _, dir := range []string{"one", "two"} {
		os.MkdirAll(filepath.Join(root, dir), 0700)
		os.WriteFile(filepath.Join(root, dir, "page.asp.ts"), []byte(`<%@ Language=JScript %><% var value:number=1;Response.Write(value); %>`), 0600)
	}
	var out bytes.Buffer
	args := []string{"--emit-asp", "--scan", root, "--out-dir", dest, "--jobs", "2"}
	if code := run(args, &out); code != 0 {
		t.Fatalf("%d %s", code, &out)
	}
	for _, dir := range []string{"one", "two"} {
		if _, err := os.Stat(filepath.Join(dest, dir, "page.asp")); err != nil {
			t.Fatal(err)
		}
	}
	out.Reset()
	if code := run(args, &out); code != 2 {
		t.Fatalf("existing files overwritten: %d %s", code, &out)
	}
	bad := filepath.Join(root, "bad.asp.ts")
	os.WriteFile(bad, []byte(`<%@ Language=JScript %><% Response.YouWrite('bad'); %>`), 0600)
	out.Reset()
	if code := run([]string{"--emit-asp", "--scan", root, "--noEmit"}, &out); code != 1 {
		t.Fatalf("semantic checking skipped: %d %s", code, &out)
	}
}

func TestUnsupportedSyntaxBlocksEntireEmissionBatch(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "build")
	os.WriteFile(filepath.Join(root, "good.asp.ts"), []byte(`<%@ Language=JScript %><% var value:number=1; %>`), 0600)
	os.WriteFile(filepath.Join(root, "bad.asp.ts"), []byte(`<%@ Language=JScript %><% enum State { Ready } %>`), 0600)
	var out bytes.Buffer
	if code := run([]string{"--emit-asp", "--scan", root, "--out-dir", dest}, &out); code != 1 {
		t.Fatalf("%d %s", code, &out)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("output written despite unsupported syntax")
	}
}

func TestSemanticErrorsDoNotBlockDefaultEmissionBatch(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "build")
	os.WriteFile(filepath.Join(root, "good.asp.ts"), []byte(`<%@ Language=JScript %><% var value:number=1; %>`), 0600)
	os.WriteFile(filepath.Join(root, "bad.asp.ts"), []byte(`<%@ Language=JScript %><% var value:number='wrong'; %>`), 0600)
	var out bytes.Buffer
	if code := run([]string{"--emit-asp", "--scan", root, "--out-dir", dest}, &out); code != 1 {
		t.Fatalf("%d %s", code, &out)
	}
	for _, name := range []string{"good.asp", "bad.asp"} {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Fatal(err)
		}
	}
}
