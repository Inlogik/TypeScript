package asp

import (
	"context"
	"testing"
)

func TestLegacyRedeclarationMismatchIsWarning(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "redeclaration.asp", `<%@ Language=JScript %><%
var value=1;
var value='changed';
Response.YouWrite(value);
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 2 {
		t.Fatalf("%v %+v", err, d)
	}
	for _, diag := range d {
		if diag.Warning != (diag.Code == 2403) {
			t.Fatalf("wrong severity: %+v", diag)
		}
	}
}
