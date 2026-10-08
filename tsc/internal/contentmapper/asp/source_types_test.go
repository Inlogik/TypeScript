package asp

import (
	"context"
	"testing"
)

func TestVirtualVariableTypesAndMapping(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "part.inc", `<% var formatter=(function(){return function(value){return String(value);};})(); formatter.masks={default:'x'}; formatter.Missing=1; %>`)
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="part.inc" -->`)
	types := writeFixture(t, root, "types.d.ts", `interface Formatter { (value?: any): string; masks?: {[key: string]: string}; }`)
	rules := writeFixture(t, root, "source-types.json", `[{"source":"part.inc","variable":"formatter","type":"Formatter"}]`)
	d, err := CheckWithProject(context.Background(), page, root, []string{types}, "", rules)
	if err != nil || len(d) != 1 || d[0].Code != 2339 || d[0].Column != 123 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestScopedVirtualRestContracts(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><%
var db={row:function(sql){return arguments.length;}};
db.row('query',1,2);
function debug(){return arguments.length;}
debug('first','second');
function ordinary(value){return value;}
ordinary(1,2);
%>`)
	rules := writeFixture(t, root, "rest.json", `[{"source":"page.asp","function":"row","variadic":true},{"source":"page.asp","function":"debug","variadic":true}]`)
	d, err := CheckWithProject(context.Background(), page, root, nil, "", rules)
	if err != nil || len(d) != 1 || d[0].Code != 2554 || d[0].Line != 7 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestSourceTypeDoesNotHideInitializerMismatch(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "page.asp", "<%@ Language=JScript %><% var value='wrong'; %>")
	rules := writeFixture(t, root, "types.json", `[{"source":"page.asp","variable":"value","type":"number"}]`)
	d, err := CheckWithProject(context.Background(), page, root, nil, "", rules)
	if err != nil || len(d) != 1 || d[0].Code != 2322 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestVirtualMethodCallableJSDoc(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "helper.inc", `<% var api={html:function(value){if(typeof value==='undefined')return '';return this;}}; %>`)
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="helper.inc" --><%
api.html('x').html();
/** @type {number} */ var bad=api.html();
api.html('x').missing();
%>`)
	types := writeFixture(t, root, "types.d.ts", `interface Tag { html(): string; html(value:any): Tag; }`)
	rules := writeFixture(t, root, "rules.json", `[
{"source":"helper.inc","variable":"api","type":"Tag"},
{"source":"helper.inc","function":"html","jsdoc":"/** @type {{(): string; (value:any): Tag}}\n * @this {Tag}\n * @param {*} [value]\n * @returns {*}\n */"}
]`)
	d, err := CheckWithProject(context.Background(), page, root, []string{types}, "", rules)
	if err != nil || len(d) != 2 || d[0].Code != 2322 || d[0].Line != 3 || d[1].Code != 2339 || d[1].Line != 4 {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestSourceJSDocRejectsExecutableOrTruncatedText(t *testing.T) {
	for _, text := range []string{"/** @returns {*} */ Response.End();", "/** unterminated", "// not JSDoc", "/* ordinary comment */"} {
		if validSourceJSDoc(text) {
			t.Fatalf("accepted %q", text)
		}
	}
	if !validSourceJSDoc("/** @returns {*} */\n/** @this {Object} */") {
		t.Fatal("rejected complete comments")
	}
}

func TestVirtualPrototypeOptionalParameters(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "index.asp", `<%@ Language=JScript %><%
Array.prototype.indexOf=function(value,begin,strict){return -1;};
[1,2].indexOf(1);
[1,2].indexOf(1,0,true);
[1,2].indexOf(1,0,'wrong');
%>`)
	types := writeFixture(t, root, "types.d.ts", `interface Array<T> { indexOf(value:T,begin?:number,strict?:boolean):number; }`)
	rules := writeFixture(t, root, "rules.json", `[{"source":"index.asp","function":"Array.prototype.indexOf","jsdoc":"/**\n * @param {*} value\n * @param {number} [begin]\n * @param {boolean} [strict]\n * @returns {number}\n */"}]`)
	d, err := CheckWithProject(context.Background(), page, root, []string{types}, "", rules)
	if err != nil || len(d) != 1 || d[0].Code != 2345 || d[0].Line != 5 {
		t.Fatalf("%v %+v", err, d)
	}
}
