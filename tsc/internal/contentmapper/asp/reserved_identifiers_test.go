package asp

import (
	"context"
	"strings"
	"testing"
)

func TestNonStrictReservedIdentifiers(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "reserved.asp", `<%@ Language=JScript %><%
var private=3, public=4;
var obj={private: private, public: public};
Response.Write(private+public+obj.private);
Response.YouWrite(public);
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || d[0].Code != 2339 || d[0].Line != 5 || d[0].Column != 10 {
		t.Fatalf("%v %+v", err, d)
	}
	m, _ := Map(page, root)
	if !strings.Contains(m.Text, "obj.private") || !strings.Contains(m.Text, "{private:") {
		t.Fatal(m.Text)
	}
}

func TestStrictReservedNamesRemainErrors(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "strict.asp", `<%@ Language=JScript %><% 'use strict'; var private=1; %>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) == 0 {
		t.Fatalf("%v %+v", err, d)
	}
}
