package asp

import (
	"context"
	"strings"
	"testing"
)

func TestNonStrictDeleteIdentifier(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "delete.asp", `<%@ Language=JScript %><%
var local=1;
var result=delete local;
delete (local);
delete missing;
var obj={value:1}; delete obj.value;
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || d[0].Code != 2304 || d[0].Line != 5 || d[0].Column != 8 {
		t.Fatalf("%v %+v", err, d)
	}
	m, _ := Map(page, root)
	if !strings.Contains(m.Text, "delete obj.value") {
		t.Fatal(m.Text)
	}
}

func TestStrictDeleteNotNormalized(t *testing.T) {
	for _, body := range []string{
		`"use strict"; var local=1; delete local;`,
		`function f(){ "use strict"; var local=1; delete local; }`,
		`class C { f(){ var local=1; delete local; } }`,
	} {
		root := t.TempDir()
		page := writeFixture(t, root, "strict.asp", "<%@ Language=JScript %><% "+body+" %>")
		m, err := Map(page, root)
		if err != nil || strings.Contains(m.Text, "__aspDeleteIdentifier(") {
			t.Fatalf("%v %+v", err, m)
		}
		d, err := Check(context.Background(), page, root)
		if err != nil || len(d) == 0 {
			t.Fatalf("%v %+v", err, d)
		}
	}
}
