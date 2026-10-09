package asp

import (
	"context"
	"strings"
	"testing"
)

func TestTypedIncludeResolutionCheckingAndSeparateErasure(t *testing.T) {
	root := t.TempDir()
	include := writeFixture(t, root, "helper.inc.ts", "<% function label(value: number): string { return String(value); } %>")
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="helper.inc" --><% Response.Write(label('wrong')); %>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || d[0].Code != 2345 {
		t.Fatalf("%v %+v", err, d)
	}
	v := PrepareVirtual(page, root, nil, "")
	if v.Error != "" || v.ScriptKind != "typescript" || v.Sources[include] == "" {
		t.Fatalf("%+v", v)
	}
	out, r := EraseASPWithCache(context.Background(), include, root, nil, false, nil, true)
	if r.Error != "" || !r.EmissionSafe || strings.Contains(out, ": number") || !strings.Contains(out, "function label(value)") {
		t.Fatalf("%+v %s", r, out)
	}
	writeFixture(t, root, "helper.inc", "<% function label(value){return value;} %>")
	if _, err := Map(page, root); err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("collision accepted: %v", err)
	}
}

func TestTypedIncludeMigrationPreservesLooseSiblingsAndExplicitTypes(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "legacy.inc", `<% var mixed='text';mixed=1;function legacy(arg){return arg;} legacy(); %>`)
	writeFixture(t, root, "typed.inc.ts", `<% var alsoMixed='text';alsoMixed=2;const explicit:number='wrong';function typed(value:number):number{return value;} %>`)
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="legacy.inc" --><!-- #include file="typed.inc" --><% typed('wrong'); %>`)
	r := CheckReport(context.Background(), page, root, nil, "", "", true)
	if r.Error != "" || len(r.Diagnostics) != 2 {
		t.Fatalf("%+v", r)
	}
	for _, d := range r.Diagnostics {
		if d.Code != 2322 && d.Code != 2345 {
			t.Fatal(d)
		}
	}
}
