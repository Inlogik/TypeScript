package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/contentmapper/asp"
)

func TestEmitASPCLI(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "customer.asp.ts")
	input := "<%@ Language=JScript %><h1>hello</h1><% var value: number = 2; %><%= value %>"
	if err := os.WriteFile(entry, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	generated := filepath.Join(dir, "generated")
	var out bytes.Buffer
	for _, args := range [][]string{
		{"--emit-asp", "--noEmit", entry},
		{"--emit-asp", "--noEmit=true", "--out-dir", generated, entry},
	} {
		out.Reset()
		if code := run(args, &out); code != 0 {
			t.Fatalf("%v: %d %s", args, code, &out)
		}
		if _, err := os.Stat(generated); !os.IsNotExist(err) {
			t.Fatal("check-only wrote output")
		}
	}
	out.Reset()
	if code := run([]string{"--emit-asp", "--out-dir", generated, "--json", entry}, &out); code != 0 {
		t.Fatalf("%d %s", code, &out)
	}
	var report asp.Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Version != 1 || len(report.Diagnostics) != 0 {
		t.Fatalf("%v %s", err, &out)
	}
	data, err := os.ReadFile(filepath.Join(generated, "customer.asp"))
	if err != nil || bytes.Contains(data, []byte(": number")) {
		t.Fatalf("%v %s", err, data)
	}
	data, _ = os.ReadFile(entry)
	if string(data) != input {
		t.Fatal("input modified")
	}
	for _, args := range [][]string{
		{"--erase-only", entry},
		{"--noEmit=false", entry},
		{"--emit-asp", "--noEmit", "--source-types", "types.json", entry},
	} {
		out.Reset()
		if code := run(args, &out); code != 2 {
			t.Fatalf("%v: %d %s", args, code, &out)
		}
	}
	out.Reset()
	if code := run([]string{"--emit-asp", entry}, &out); code != 0 {
		t.Fatalf("default output: %d %s", code, &out)
	}
	out.Reset()
	if code := run([]string{"--emit-asp", entry}, &out); code != 2 {
		t.Fatalf("overwrite accepted: %d %s", code, &out)
	}
	bad := filepath.Join(dir, "bad.asp.ts")
	os.WriteFile(bad, []byte("<%@ Language=JScript %><% var value: number = 'bad'; %>"), 0600)
	out.Reset()
	if code := run([]string{"--emit-asp", "--no-emit-on-error", "--out-dir", generated, bad}, &out); code != 1 {
		t.Fatalf("%d %s", code, &out)
	}
	if _, err := os.Stat(filepath.Join(generated, "bad.asp")); !os.IsNotExist(err) {
		t.Fatal("emitted despite diagnostics")
	}
}
