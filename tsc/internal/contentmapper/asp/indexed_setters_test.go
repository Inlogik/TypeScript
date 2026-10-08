package asp

import (
	"context"
	"strings"
	"testing"
)

func TestGeneralCOMIndexedSetters(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "com.asp", `<%@ Language=JScript %><%
command('value') = 'ok';
record.Fields('value') = 42;
matrix(1,2) = command('other') = 3;
(command('parenthesized')) = 4;
/** @type {number} */ var n = (record.Fields('value') = 5);
/** @type {string} */ var bad = (command('value') = 6);
%>`)
	types := writeFixture(t, root, "com.d.ts", `declare function command(key: string): any;
declare const record: { Fields(key: string): any };
declare function matrix(row: number, column: number): any;`)
	d, err := CheckWithTypes(context.Background(), page, root, []string{types})
	if err != nil || len(d) != 1 || d[0].Code != 2322 || d[0].Line != 7 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestGeneralSetterStillChecksTargetsAndIndices(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "arguments.asp", `<%@ Language=JScript %><%
command(123) = 'value';
unknownTarget('key') = 'value';
record.Missing('key') = 'value';
%>`)
	types := writeFixture(t, root, "com.d.ts", `declare function command(key: string): any;
declare const record: { Fields(key: string): any };`)
	d, err := CheckWithTypes(context.Background(), page, root, []string{types})
	if err != nil || len(d) != 3 {
		t.Fatalf("%v %+v", err, d)
	}
	want := map[int32]int{2345: 2, 2304: 3, 2339: 4}
	for _, diag := range d {
		if diag.Line != want[diag.Code] {
			t.Fatal(diag)
		}
	}
}

func TestGeneralSetterDoesNotRewriteOrdinaryAssignments(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "ordinary.asp", `<%@ Language=JScript %><%
var a={}; a.key=1; a['key']=2;
var s="command('x') = 1";
// command('x') = 1;
%>`)
	m, err := Map(page, root)
	if err != nil || strings.Contains(m.Text, "__aspSetIndexed(") {
		t.Fatalf("%v %+v", err, m)
	}
}
