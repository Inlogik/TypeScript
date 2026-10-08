package asp

import (
	"context"
	"testing"
)

func TestOnlyLegacyTypeofComparisonsAreWarnings(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "date.asp", `<%@ Language=JScript %><%
function check(a){
    var first=typeof(a)==='date';
    var reversed='date' !== (typeof a);
    var other=typeof a==='Function';
    var plain=123==='date';
    return first;
}
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 4 {
		t.Fatalf("%v %+v", err, d)
	}
	for _, diag := range d {
		if diag.Warning != (diag.Line == 3 || diag.Line == 4 || diag.Line == 5) {
			t.Fatalf("wrong severity: %+v", diag)
		}
	}
}

func TestOtherTypeofComparisonsRemainErrors(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "other.asp", `<%@ Language=JScript %><%
function f(value) {
    return typeof value === 'null' || typeof value === 'FUNCTION';
}
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 2 {
		t.Fatalf("%v %+v", err, d)
	}
	for _, diag := range d {
		if diag.Warning {
			t.Fatal(diag)
		}
	}
}
