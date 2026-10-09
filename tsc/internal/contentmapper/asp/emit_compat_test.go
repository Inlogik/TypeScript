package asp

import (
	"context"
	"testing"
)

func TestErasurePreservesVerifiedLegacySyntax(t *testing.T) {
	root := t.TempDir()
	input := "<%@ Language=JScript %>\r\n<%\r\nfunction Object.example(value) { return value; }\r\nvar private=1, public=2;\r\nvar text=\"one \\ \r\ntwo\";\r\nSession('key')=text;\r\nvar count=Request.QueryString.Count();\r\ndelete text;\r\nResponse.Write(private+public);\r\n%><h1>unchanged</h1>"
	page := writeFixture(t, root, "legacy.asp.ts", input)
	types := writeFixture(t, root, "types.d.ts", "interface ObjectConstructor {example(value?:any):any;}")
	out, r := EraseASPWithOptions(context.Background(), page, root, []string{types}, true)
	if r.Error != "" || len(r.Diagnostics) != 0 || out != input {
		t.Fatalf("%+v output=%q", r, out)
	}
}
