package asp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func eraseFixture(t *testing.T, code string) (string, Report) {
	t.Helper()
	entry := filepath.Join(t.TempDir(), "page.asp.ts")
	if err := os.WriteFile(entry, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	return EraseASP(context.Background(), entry, "", nil)
}

func TestEraseASPSyntax(t *testing.T) {
	input := "\uFEFF<%@ Language=JScript %>\r\n<h1>😀</h1><%\ninterface Item { value: number }\ntype Alias = Item;\nfunction identity<T>(value: T): T { return value; }\nvar count: number = identity<number>(2);\nvar item = { value: count } satisfies Item;\nvar value = (<Item>item).value!;\nif ((count as number) > 0) { %><b>yes</b><%= count as number %><% } else { %>no<% } %>\n<script language=JScript runat=server>var other: number = count;</script>"
	output, report := eraseFixture(t, input)
	if report.Error != "" || len(report.Diagnostics) != 0 {
		t.Fatalf("%+v", report)
	}
	for _, preserved := range []string{"\uFEFF<%@ Language=JScript %>\r\n<h1>😀</h1>", "<b>yes</b><%=", "%><% } else { %>no<% } %>", "<script language=JScript runat=server>", "</script>", "return value;", "identity(2)"} {
		if !strings.Contains(output, preserved) {
			t.Errorf("missing %q in %s", preserved, output)
		}
	}
	for _, removed := range []string{"interface Item", "type Alias", ": number", " as number", "satisfies Item", "<Item>", "<T>"} {
		if strings.Contains(output, removed) {
			t.Errorf("retained %q", removed)
		}
	}
}

func TestEraseASPRejects(t *testing.T) {
	for _, code := range []string{
		"enum State { Ready }", "const enum State { Ready }", "namespace N { export var value = 1; }", "import { value } from 'module';", "import type { Item } from 'module';", "export var value = 1;", "class C { constructor(public value: number) {} }", "var before = 1; @decorator class C {}", "@decorator class C {}", "var value: number = 'wrong';", "var value: = 1;", "function f(this: object) {}",
	} {
		t.Run(code, func(t *testing.T) {
			output, report := eraseFixture(t, "<%@ Language=JScript %><% "+code+" %>")
			if output != "" || (report.Error == "" && len(report.Diagnostics) == 0) {
				t.Fatalf("unexpected success: %q %+v", output, report)
			}
		})
	}
}

func TestEraseASPFunctions(t *testing.T) {
	output, report := eraseFixture(t, "<%@ Language=JScript %><% function f<T>(x: T, y?: T): T; function f<T>(x: T, y?: T): T { return x; } var g = f<number>; var value: number = g(1); var pending!: number; %>")
	if report.Error != "" || len(report.Diagnostics) != 0 {
		t.Fatalf("%+v", report)
	}
	if strings.Count(output, "function f") != 1 || strings.Contains(output, "y?") || strings.Contains(output, "pending!") {
		t.Fatal(output)
	}
}

func TestEraseASPCommentsAndNestedGenerics(t *testing.T) {
	_, report := eraseFixture(t, "<%@ Language=JScript %><% type Box<T> = { value: T }; function id<T>(x: T): T { return x; } var box = id<Box<number>>({ value: 1 }); var f = (x /* : not a type */: number): number => x; var obj = { method(x: number): number { return x; } }; function unpack({value}: Box<number>): number { return value; } var n = f(obj.method(box.value)); %>")
	if report.Error != "" || len(report.Diagnostics) != 0 {
		t.Fatalf("%+v", report)
	}
}

func TestEraseASPRejectsASIChanges(t *testing.T) {
	_, report := eraseFixture(t, "<%@ Language=JScript %><% var count = 1; count\ninterface I {}\n[0]; %>")
	if len(report.Diagnostics) == 0 {
		t.Fatal("changed runtime AST accepted")
	}
}

func TestEraseASPIncludes(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "page.asp.ts")
	include := "<!-- #include file='shared.asp' -->"
	input := "<%@ Language=JScript %>" + include + "<% var value: number = shared; %>"
	os.WriteFile(entry, []byte(input), 0600)
	os.WriteFile(filepath.Join(dir, "shared.asp"), []byte("<% var shared = 2; %>"), 0600)
	output, report := EraseASP(context.Background(), entry, "", nil)
	if report.Error != "" || len(report.Diagnostics) != 0 {
		t.Fatalf("%+v", report)
	}
	if !strings.Contains(output, include) || strings.Contains(output, "var shared") {
		t.Fatalf("include changed: %s", output)
	}
	os.WriteFile(filepath.Join(dir, "shared.asp"), []byte("<% var shared: number = 2; %>"), 0600)
	_, report = EraseASP(context.Background(), entry, "", nil)
	if len(report.Diagnostics) == 0 {
		t.Fatal("typed include accepted")
	}
}

func TestEraseASPPositionsAndBoundaries(t *testing.T) {
	_, report := eraseFixture(t, "<%@ Language=JScript %>\r\n😀<% var x: number = 'bad'; %>")
	if len(report.Diagnostics) == 0 || report.Diagnostics[0].Line != 2 || report.Diagnostics[0].Column != 10 {
		t.Fatalf("%+v", report)
	}
	_, report = eraseFixture(t, "<%@ Language=JScript %><% var x: %><% number = 1; %>")
	if len(report.Diagnostics) == 0 {
		t.Fatal("type spanning delimiter accepted")
	}
}

func TestWriteErasedASP(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "page.asp.ts")
	os.WriteFile(entry, []byte("source"), 0600)
	dest, err := WriteErasedASP(entry, "", "output")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dest) != "page.asp" {
		t.Fatal(dest)
	}
	if _, err := WriteErasedASP(entry, dir, "again"); err == nil {
		t.Fatal("existing output overwritten")
	}
	output, _ := os.ReadFile(dest)
	if string(output) != "output" {
		t.Fatal("existing output changed")
	}
	data, _ := os.ReadFile(entry)
	if string(data) != "source" {
		t.Fatal("input overwritten")
	}
	os.Remove(dest)
	if err := os.Link(entry, dest); err != nil {
		t.Skip(err)
	}
	if _, err := WriteErasedASP(entry, dir, "bad"); err == nil {
		t.Fatal("hardlink overwrite accepted")
	}
}
