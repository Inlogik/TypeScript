package asp

import (
	"context"
	"testing"
)

func TestImplicitGlobalAssignmentsAreWarnings(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "globals.asp", `<%@ Language=JScript %><%
function f(){ currentChar=1; return currentChar; }
function g(){ return currentChar; }
Response.Write(neverAssigned);
function strict(){ 'use strict'; currentChar=2; }
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 5 {
		t.Fatalf("%v %+v", err, d)
	}
	for _, diag := range d {
		wantWarning := diag.Line == 2 || diag.Line == 3
		if diag.Warning != wantWarning {
			t.Fatalf("unexpected severity: %+v", diag)
		}
	}
}

func TestDeclaredVariablesAndCompoundAssignmentsNotDowngraded(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "other.asp", `<%@ Language=JScript %><%
var declared=1; declared='wrong';
missing+=1;
other++;
class C { f(){ classGlobal=1; } }
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) == 0 {
		t.Fatalf("%v %+v", err, d)
	}
	for _, diag := range d {
		if diag.Warning {
			t.Fatalf("incorrect warning: %+v", diag)
		}
	}
}

func TestImplicitForInAndForOfTargets(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "loops.asp", `<%@ Language=JScript %><%
function f(object){ for(item in object){ Response.Write(object[item]); } }
function g(values){ for(value of values){ Response.Write(value); } }
function strict(object){ 'use strict'; for(strictItem in object){ Response.Write(strictItem); } }
for(var local in {}) { Response.Write(local); }
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 6 {
		t.Fatalf("%v %+v", err, d)
	}
	for _, diag := range d {
		if diag.Warning != (diag.Line == 2 || diag.Line == 3) {
			t.Fatalf("wrong severity: %+v", diag)
		}
	}
}

func TestOptionalPageHooksThroughProjectTypes(t *testing.T) {
	root := t.TempDir()
	types := writeFixture(t, root, "hooks.d.ts", `declare function get(viewdata: any): any;
declare function post(viewdata: any): any;`)
	for _, body := range []string{
		`var data={}; if(typeof(get)==='function') get.apply(data,[data]); if(typeof(post)==='function') post.apply(data,[data]);`,
		`function get(data){return data;} function post(data){return data;} get({}); post({});`,
	} {
		page := writeFixture(t, root, "hooks.asp", "<%@ Language=JScript %><% "+body+" %>")
		d, err := CheckWithTypes(context.Background(), page, root, []string{types})
		if err != nil || len(d) != 0 {
			t.Fatalf("%v %+v", err, d)
		}
	}
	page := writeFixture(t, root, "typo.asp", "<%@ Language=JScript %><% if(typeof(get)==='function') gett({}); %>")
	d, err := CheckWithTypes(context.Background(), page, root, []string{types})
	if err != nil || len(d) != 1 || d[0].Warning {
		t.Fatalf("%v %+v", err, d)
	}
}
