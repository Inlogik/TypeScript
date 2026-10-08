package asp

import (
	"context"
	"testing"
)

func TestRepeatedIncludeDiagnosticsDeduplicatedAtOriginalSource(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "repeat.inc", "<% Response.YouWrite('bad'); %>")
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="repeat.inc" --><!-- #include file="repeat.inc" -->`)
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 1 || d[0].Code != 2339 {
		t.Fatalf("%v %+v", err, d)
	}
	m, err := Map(page, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Spans) != 2 {
		t.Fatalf("source expansion deduplicated: %+v", m.Spans)
	}
}

func TestScopedCoercionOverloads(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "coercion.asp", `<%@ Language=JScript %><%
parseInt(123,10);
parseInt(new String('123'),10);
Date.parse(new Date());
parseFloat('12',10);
%>`)
	types := writeFixture(t, root, "legacy.d.ts", `declare function parseInt(value: number | String, radix?: number): number;
interface DateConstructor { parse(value: Date): number; }`)
	d, err := CheckWithTypes(context.Background(), page, root, []string{types})
	if err != nil || len(d) != 1 || d[0].Code != 2554 || d[0].Line != 5 {
		t.Fatalf("%v %+v", err, d)
	}
	d, err = Check(context.Background(), page, root)
	if err != nil || len(d) != 4 {
		t.Fatalf("overloads leaked: %v %+v", err, d)
	}
}
