package asp

import (
	"context"
	"strings"
	"testing"
)

func TestEraseLegacySettersWithoutTypesIsIdentical(t *testing.T) {
	root := t.TempDir()
	input := "<%@ Language=JScript %>\r\n<h1>same</h1><%\r\nSession(\"asdf\") = \"asdfasdf\";\r\nApplication('nested') = Session('other') = 3;\r\nvar command=Server.CreateObject('Example.Command');\r\ncommand('value') = 42;\r\n%>"
	page := writeFixture(t, root, "setters.asp.ts", input)
	out, r := EraseASP(context.Background(), page, root, nil)
	if r.Error != "" || len(r.Diagnostics) != 0 || out != input {
		t.Fatalf("report=%+v output=%q", r, out)
	}
}

func TestEraseTypesAroundLegacyIndexedSetters(t *testing.T) {
	root := t.TempDir()
	input := `<%@ Language=JScript %><%
var value: string = 'text';
Session('key') = (value as string);
Application('key') = Session('other') = (123 as number);
%>`
	page := writeFixture(t, root, "typed.asp.ts", input)
	out, r := EraseASP(context.Background(), page, root, nil)
	if r.Error != "" || len(r.Diagnostics) != 0 {
		t.Fatalf("%+v", r)
	}
	if strings.Contains(out, "as string") || strings.Contains(out, ": string") || strings.Contains(out, "__aspSetIndexed") {
		t.Fatal(out)
	}
	for _, text := range []string{"Session('key') =", "Application('key') = Session('other') ="} {
		if !strings.Contains(out, text) {
			t.Fatal(out)
		}
	}
}

func TestLegacySetterStillChecksTypedRHS(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "invalid.asp.ts", `<%@ Language=JScript %><% var value: number='wrong'; Session('key')=value; %>`)
	out, r := EraseASP(context.Background(), page, root, nil)
	if out != "" || len(r.Diagnostics) == 0 {
		t.Fatalf("%+v %q", r, out)
	}
}
