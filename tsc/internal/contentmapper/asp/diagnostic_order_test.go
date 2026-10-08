package asp

import (
	"context"
	"reflect"
	"testing"
)

func TestMappedDiagnosticOrder(t *testing.T) {
	want := []Diagnostic{
		{Code: 1, Message: "global"},
		{File: "a.asp", Line: 1, Column: 2, Code: 1, Message: "a"},
		{File: "a.asp", Line: 1, Column: 2, Code: 1, Message: "b"},
		{File: "a.asp", Line: 1, Column: 2, Code: 2},
		{File: "a.asp", Line: 1, Column: 3, Code: 1},
		{File: "a.asp", Line: 2, Column: 1, Code: 1},
		{File: "z.inc", Line: 1, Column: 1, Code: 1},
		{File: "a.asp", Line: 1, Column: 1, Code: 1, Warning: true},
		{File: "z.inc", Line: 1, Column: 1, Code: 1, Warning: true},
	}
	got := append([]Diagnostic(nil), want...)
	for i, j := 0, len(got)-1; i < j; i, j = i+1, j-1 {
		got[i], got[j] = got[j], got[i]
	}
	sortMappedDiagnostics(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestIncludeDiagnosticsSortedByOriginalFileAndSeverity(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "z.inc", "<% Response.YouWrite('z'); implicit=1; %>")
	writeFixture(t, root, "a.inc", "<% Response.YouWrite('a'); %>")
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="z.inc" --><!-- #include file="a.inc" -->`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 3 {
		t.Fatalf("%v %+v", err, d)
	}
	if d[0].File >= d[1].File || d[0].Warning || d[1].Warning || !d[2].Warning {
		t.Fatalf("wrong ordering: %+v", d)
	}
}
