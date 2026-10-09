package asp

import (
	"strings"
	"testing"
)

func TestErasureRemovesTypePadding(t *testing.T) {
	out, r := eraseFixture(t, `<%@ Language=JScript %><% const customer: {Name:string} = {Name:'Alex'}; function greet(name: string): string { return name; } %>`)
	if r.Error != "" || len(r.Diagnostics) != 0 {
		t.Fatalf("%+v", r)
	}
	if !strings.Contains(out, "const customer =") || !strings.Contains(out, "function greet(name) {") {
		t.Fatal(out)
	}
}

func TestErasureRemovesDeclarationOnlyLines(t *testing.T) {
	input := "<%@ Language=JScript %><%\r\ninterface Item {\r\n  value: number;\r\n}\r\ntype Count = number;\r\nconst count: Count = 1;\r\nResponse.Write(count);\r\n%>"
	out, r := eraseFixture(t, input)
	if r.Error != "" || len(r.Diagnostics) != 0 {
		t.Fatalf("%+v", r)
	}
	want := "<%@ Language=JScript %><%\r\nconst count = 1;\r\nResponse.Write(count);\r\n%>"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestInlineDeclarationPreservesSharedLine(t *testing.T) {
	out, r := eraseFixture(t, `<%@ Language=JScript %><% interface Item { value:number; } const count:number=1; %><p>unchanged</p>`)
	if r.Error != "" || len(r.Diagnostics) != 0 {
		t.Fatalf("%+v", r)
	}
	if !strings.Contains(out, "const count=1;") || !strings.Contains(out, "%><p>unchanged</p>") {
		t.Fatal(out)
	}
}
