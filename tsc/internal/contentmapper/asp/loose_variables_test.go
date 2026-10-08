package asp

import (
	"context"
	"testing"
)

func TestLooseVariablesPreservesExplicitContracts(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "loose.asp", `<%@ Language=JScript %><%
var value='text'; value=123;
/** @type {number} */ var typed='wrong';
/** @type {{Name:string}} */ var customer={Name:'ok'};
Response.YouWrite(value);
Response.Write(customer.Banana);
%>`)
	d, err := CheckWithOptions(context.Background(), page, root, nil, "", "", true)
	if err != nil || len(d) != 3 {
		t.Fatalf("%v %+v", err, d)
	}
	for _, diag := range d {
		if diag.Line != 3 && diag.Line != 5 && diag.Line != 6 {
			t.Fatal(diag)
		}
	}
	d, err = Check(context.Background(), page, root)
	if err != nil || len(d) != 4 {
		t.Fatalf("default changed: %v %+v", err, d)
	}
}

func TestLooseVariablesKeepsProjectAnnotationAndParameters(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "project.asp", `<%@ Language=JScript %><%
var value='wrong';
/** @param {number} n */ function f(n){return n;}
f('wrong');
%>`)
	rules := writeFixture(t, root, "rules.json", `[{"source":"project.asp","variable":"value","type":"number"}]`)
	d, err := CheckWithOptions(context.Background(), page, root, nil, "", rules, true)
	if err != nil || len(d) != 2 || d[0].Code != 2322 || d[1].Code != 2345 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestLooseVariablesLoopsMultipleBindingsAndTypedAliasTradeoff(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "bindings.asp", `<%@ Language=JScript %><%
var a='', b=1; a=2; b='text';
for(var i=0;i<2;i++){ i='text'; }
for(var key in {x:1}){ Response.Write(key); }
/** @returns {{Name:string}} */ function make(){ return {Name:'ok'}; }
var inferred=make(); inferred.Banana;
/** @type {{Name:string}} */ var explicit=make(); explicit.Banana;
%>`)
	d, err := CheckWithOptions(context.Background(), page, root, nil, "", "", true)
	if err != nil || len(d) != 1 || d[0].Code != 2339 || d[0].Line != 7 {
		t.Fatalf("%v %+v", err, d)
	}
}
