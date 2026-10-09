package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEmitOnErrorKeepsSemanticErrorsAndBlocksUnsafeOutput(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name, source string
		written      bool
	}{
		{"semantic", `<%@ Language=JScript %><% var value:number='wrong'; %>`, true},
		{"syntax", `<%@ Language=JScript %><% var value: = ; %>`, false},
		{"enum", `<%@ Language=JScript %><% enum E { A } %>`, false},
	} {
		page := filepath.Join(root, tc.name+".asp.ts")
		if err := os.WriteFile(page, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if code := run([]string{"--emit-asp", page}, &out); code != 1 {
			t.Fatalf("%s: %d %s", tc.name, code, &out)
		}
		_, err := os.Stat(filepath.Join(root, tc.name+".asp"))
		if (err == nil) != tc.written {
			t.Fatalf("%s output: %v", tc.name, err)
		}
	}
}
