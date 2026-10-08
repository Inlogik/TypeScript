package asp

import (
	"context"
	"testing"
)

func TestActiveXObjectHostDeclaration(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "activex.asp", `<%@ Language=JScript %><%
var fso=new ActiveXObject("Scripting.FileSystemObject");
fso.FileExists("example.txt");
new ActiveXObject(123);
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || d[0].Code != 2345 || d[0].Line != 4 {
		t.Fatalf("%v %+v", err, d)
	}
}
