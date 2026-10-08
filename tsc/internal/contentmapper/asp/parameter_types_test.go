package asp

import (
	"context"
	"testing"
)

func TestUnannotatedParametersDefaultToAny(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "parameters.asp", `<%@ Language=JScript %><%
function f(value) { value='text'; value=123; return value; }
f(); f(123); f('text'); f({});
var api={method:function(value){value='text';value=123;return value;}};
api.method({});
function defaulted(value='text') { value=123; return value; }
defaulted(123);
/** @param {number} value */ function typed(value){return value;}
typed('wrong');
f(1,2);
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 2 || d[0].Code != 2345 || d[0].Line != 9 || d[1].Code != 2554 || d[1].Line != 10 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestExplicitParameterTypePreserved(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "explicit.asp", `<%@ Language=JScript %><%
function f(/** @type {number} */ value) { return value; }
f('wrong');
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || d[0].Code != 2345 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestPartialJSDocLeavesOtherParametersAny(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "partial.asp", `<%@ Language=JScript %><%
/** @param {number} typed @returns {number} */
function f(typed, loose='text') { loose=123; return typed; }
f(1,123);
f('wrong',123);
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || d[0].Code != 2345 || d[0].Line != 5 {
		t.Fatalf("%v %+v", err, d)
	}
}
