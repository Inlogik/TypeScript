package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExitCodes(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name, text string
		code       int
	}{
		{"valid", "<%@ Language=JScript %><% Response.Write('ok'); %>", 0},
		{"typed", "<%@ Language=JScript %><% /** @type {number} */ var n='wrong'; %>", 1},
		{"broken", "<%@ Language=JScript %><%", 2},
	} {
		file := filepath.Join(root, tc.name+".asp")
		if err := os.WriteFile(file, []byte(tc.text), 0600); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if code := run([]string{"--noEmit", file}, &out); code != tc.code {
			t.Fatalf("%s: code %d: %s", tc.name, code, out.String())
		}
		if _, err := os.Stat(file + ".__asp_check.js"); !os.IsNotExist(err) {
			t.Fatalf("emitted virtual JS: %v", err)
		}
	}
	var out bytes.Buffer
	if code := run(nil, &out); code != 2 {
		t.Fatal(code)
	}
	if code := run([]string{"--noEmit=false", "page.asp"}, &out); code != 2 {
		t.Fatal(code)
	}
}

func TestWarningsDoNotFailExitCode(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "globals.asp")
	if err := os.WriteFile(page, []byte("<%@ Language=JScript %><% implicit=1; Response.Write(implicit); %>"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := run([]string{page}, &out); code != 0 || !strings.Contains(out.String(), "warning TS") || strings.Contains(out.String(), "error TS") {
		t.Fatalf("%d: %s", code, out.String())
	}
	if err := os.WriteFile(page, []byte("<%@ Language=JScript %><% implicit=1; Response.YouWrite(implicit); %>"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{page}, &out); code != 1 {
		t.Fatalf("%d: %s", code, out.String())
	}
}

func TestLooseVariablesFlag(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "loose.asp")
	if err := os.WriteFile(page, []byte("<%@ Language=JScript %><% var value='text'; value=1; Response.Write(value); %>"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := run([]string{"--loose-variables", page}, &out); code != 0 {
		t.Fatalf("%d: %s", code, out.String())
	}
	out.Reset()
	if code := run([]string{page}, &out); code != 1 {
		t.Fatalf("default changed: %d: %s", code, out.String())
	}
}

func TestErrorsOnlyHidesWarningsWithoutChangingExitCodes(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "warnings.asp")
	for _, tc := range []struct {
		source   string
		exit     int
		hasError bool
	}{
		{"<%@ Language=JScript %><% implicit=1; %>", 0, false},
		{"<%@ Language=JScript %><% implicit=1; Response.YouWrite(implicit); %>", 1, true},
	} {
		if err := os.WriteFile(page, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		code := run([]string{"--errors-only", page}, &output)
		if code != tc.exit || strings.Contains(output.String(), "warning TS") || strings.Contains(output.String(), "error TS") != tc.hasError {
			t.Fatalf("%d: %s", code, output.String())
		}
	}
	var output bytes.Buffer
	if code := run([]string{"--errors-only", filepath.Join(root, "missing.asp")}, &output); code != 2 || !strings.Contains(output.String(), "ASP:") {
		t.Fatalf("%d: %s", code, output.String())
	}
}

func TestRepeatedTypesFlags(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "page.asp")
	first := filepath.Join(root, "first.d.ts")
	second := filepath.Join(root, "second.d.ts")
	for file, text := range map[string]string{page: "<%@ Language=JScript %><% Response.Write(First + Second); %>", first: "declare const First: string;", second: "declare const Second: string;"} {
		if err := os.WriteFile(file, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if code := run([]string{"--types", first, "--types", second, page}, &out); code != 0 {
		t.Fatalf("%d: %s", code, out.String())
	}
}
