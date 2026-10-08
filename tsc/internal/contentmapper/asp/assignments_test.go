package asp

import (
	"context"
	"strings"
	"testing"
)

func TestResponseMembersAreChecked(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "response.asp", "<%@ Language=JScript %>\n<% Response.Write('ok'); Response.YouWrite('bad'); Response.ContentType='text/plain'; Response.AddHeader('X-Test','ok'); Response.Flush(); %>")
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || d[0].Code != 2339 || !strings.Contains(d[0].Message, "YouWrite") || d[0].Line != 2 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestIndexedAssignmentsAndExpressionTypes(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "setters.asp", `<%@ Language=JScript %><%
Application("key") = 1;
Session("key") = Application("key");
Application("nested") = Session("other") = 3;
/** @type {number} */ var n = (Application("number") = 4);
/** @type {string} */ var s = (Session("number") = 5);
Application("comma") = (Response.Write('ok'), 2);
Application("comment") /* ) = */ = /* comment */ 6;
var text = 'Application("not-code") = missing';
var equal = Application("key") == 1;
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || d[0].Code != 2322 || d[0].Line != 6 {
		t.Fatalf("%v %+v", err, d)
	}
	m, err := Map(page, root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.Text, `'Application("not-code") = missing'`) || !strings.Contains(m.Text, `Application("key") == 1`) {
		t.Fatal(m.Text)
	}
}

func TestIndexedAssignmentMappingAcrossIncludes(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "part.inc", "<% Application('key') = missingValue; Response.YouWrite('bad'); %>")
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="part.inc" -->`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 2 {
		t.Fatalf("%v %+v", err, d)
	}
	want := map[int32]int{2304: 25, 2339: 48}
	for _, diag := range d {
		if diag.Line != 1 || diag.Column != want[diag.Code] {
			t.Fatalf("%+v", diag)
		}
	}
}

func TestCompoundIndexedAssignmentNotSilentlyRewritten(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "compound.asp", `<%@ Language=JScript %><% Application("key") += 1; Session("key")++; %>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) < 2 {
		t.Fatalf("%v %+v", err, d)
	}
}
