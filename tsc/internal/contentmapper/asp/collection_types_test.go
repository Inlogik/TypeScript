package asp

import (
	"context"
	"testing"
)

func TestCallableASPCollectionTypes(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "collections.asp", `<%@ Language=JScript %><%
for(var i=1;i<=Request.QueryString.Count;i++) {
    Response.Write(Request.QueryString.Key(i));
    Response.Write(Request.QueryString(i));
}
Response.Write(Request.Form.Item('name'));
Response.Write(Request.ServerVariables('REQUEST_METHOD'));
Response.Write(Request.Cookies('name'));
Response.Write(Response.Cookies.Count);
Response.Write(Request.QueryString.Count());
Response.Write(Session.Contents(1));
Response.Write(Application.Contents(1));
Response.CharSet = 'UTF-8'; Response.Charset = Response.CharSet;
Response.Write(Request.QueryString.Missing);
Response.Write(Request.Form.Key('wrong'));
%>`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 2 || d[0].Code != 2339 || d[0].Line != 14 || d[1].Code != 2345 || d[1].Line != 15 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestProjectPrototypeDeclarationsStayScoped(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "extensions.asp", `<%@ Language=JScript %><%
var value='abc'.left(2);
[1,2].each(function(index,item){ Response.Write(item); });
'abc'.lleft(2);
%>`)
	types := writeFixture(t, root, "extensions.d.ts", `interface String { left(count: number): string; }
interface Array<T> { each(callback: (index: number,item: T)=>any): this; }`)
	d, err := CheckWithTypes(context.Background(), page, root, []string{types})
	if err != nil || len(d) != 1 || d[0].Line != 4 || d[0].Warning {
		t.Fatalf("%v %+v", err, d)
	}
	d, err = Check(context.Background(), page, root)
	if err != nil || len(d) != 3 {
		t.Fatalf("declarations leaked: %v %+v", err, d)
	}
}
