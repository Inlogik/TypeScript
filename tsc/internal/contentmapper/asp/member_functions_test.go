package asp

import (
	"context"
	"strings"
	"testing"
)

func TestMemberFunctionsWithProjectDeclarations(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "members.asp", `<%@ Language=JScript %><%
/** @param {number} value @returns {number} */
function Object.example(value) { return value; }
function String.example() { return 'ok'; }
Object.example('wrong');
String.example();
%>`)
	types := writeFixture(t, root, "project.d.ts", `interface ObjectConstructor { example(value: number): number; }
interface StringConstructor { example(): string; }`)
	d, err := CheckWithTypes(context.Background(), page, root, []string{types})
	if err != nil || len(d) != 1 || d[0].Code != 2345 || d[0].Line != 5 || d[0].Column != 16 {
		t.Fatalf("%v %+v", err, d)
	}
	m, err := Map(page, root)
	if err != nil || !strings.Contains(m.Text, "Object.example = function") {
		t.Fatalf("%v %+v", err, m)
	}
}

func TestTypesInputValidation(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "page.asp", "<%@ Language=JScript %><% Response.Write(ProjectValue); %>")
	types := writeFixture(t, root, "project.d.ts", "declare const ProjectValue: string;")
	d, err := CheckWithTypes(context.Background(), page, root, []string{types})
	if err != nil || len(d) != 0 {
		t.Fatalf("%v %+v", err, d)
	}
	for _, name := range []string{"missing.d.ts", "not-a-declaration.js"} {
		if _, err := CheckWithTypes(context.Background(), page, root, []string{name}); err == nil {
			t.Fatal(name)
		}
	}
}

func TestMemberDeclarationMappingAndNonCode(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><%
var text='function Object.fake() {}';
// function Object.comment() {}
function Object.example() { return missing; }
%>`)
	types := writeFixture(t, root, "project.d.ts", "interface ObjectConstructor { example(): any; }")
	d, err := CheckWithTypes(context.Background(), page, root, []string{types})
	if err != nil || len(d) != 1 || d[0].Code != 2304 || d[0].Line != 4 || d[0].Column != 36 {
		t.Fatalf("%v %+v", err, d)
	}
	m, _ := Map(page, root)
	if !strings.Contains(m.Text, "'function Object.fake() {}'") || !strings.Contains(m.Text, "// function Object.comment() {}") {
		t.Fatal(m.Text)
	}
}

func TestMemberFunctionFollowedByIIFE(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "iife.asp", `<%@ Language=JScript %><%
function Object.example() { return 1; }
(function(){ Response.Write('ok'); })();
Object.original = function(){ return 2; };
%>`)
	types := writeFixture(t, root, "project.d.ts", "interface ObjectConstructor { example(): number; original(): number; }")
	d, err := CheckWithTypes(context.Background(), page, root, []string{types})
	if err != nil || len(d) != 0 {
		t.Fatalf("%v %+v", err, d)
	}
	m, _ := Map(page, root)
	if !strings.Contains(m.Text, "return 1; };") || strings.Contains(m.Text, "return 2; };;") {
		t.Fatal(m.Text)
	}
}
