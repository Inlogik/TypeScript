package asp

import (
	"context"
	"testing"
)

func TestTypedASPCheckUsesTypeScriptWithoutEmitting(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "page.asp.ts", `<%@ Language=JScript %><% const value:number='wrong';function greet(name:string):string{return name;} Response.Write(greet('ok')); %>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || d[0].Code != 2322 {
		t.Fatalf("%v %+v", err, d)
	}
	r := PrepareVirtual(page, root, nil, "")
	if r.Error != "" || r.ScriptKind != "typescript" {
		t.Fatalf("%+v", r)
	}
}
