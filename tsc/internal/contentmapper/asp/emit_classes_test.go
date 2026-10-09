package asp

import (
	"context"
	"strings"
	"testing"
)

func TestEraseClassesPreservesRuntimeJavaScript(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "classes.asp.ts", `<%@ Language=JScript %><%
interface Named { name: string; }
class Base<T> { constructor(value: T) {} }
class Customer extends Base<string> implements Named {
    declare external: string;
    public name: string = 'Example';
    optional?: number;
    definite!: string;
    #secret = 1;
    static count: number = 0;
    constructor(name: string) { super(name); this.name = name; }
    label(prefix: string): string { return prefix + this.name; }
    get secret(): number { return this.#secret; }
}
const Local = class { value: number = 3; };
Response.Write(new Customer('Name').label('Hello '));
%><h1>unchanged</h1>`)
	out, r := EraseASP(context.Background(), page, root, nil)
	if r.Error != "" || len(r.Diagnostics) != 0 {
		t.Fatalf("%+v", r)
	}
	for _, text := range []string{"class Customer extends Base", "name", "= 'Example'", "optional", "definite", "#secret = 1", "static count", "super(name)", "class { value", "<h1>unchanged</h1>"} {
		if !strings.Contains(out, text) {
			t.Fatalf("missing %q: %s", text, out)
		}
	}
	for _, text := range []string{"interface Named", "implements Named", "declare external", ": string", ": number", "public name", "optional?", "definite!", "Base<string>"} {
		if strings.Contains(out, text) {
			t.Fatalf("not erased %q: %s", text, out)
		}
	}
}

func TestClassRuntimeGenerationFeaturesRemainRejected(t *testing.T) {
	for _, code := range []string{`class C { constructor(public value: number) {} }`, `class C { accessor value: number = 1; }`, `declare function dec(value:any):any; @dec class C {}`} {
		root := t.TempDir()
		page := writeFixture(t, root, "unsupported.asp.ts", "<%@ Language=JScript %><% "+code+" %>")
		out, r := EraseASP(context.Background(), page, root, nil)
		if out != "" || (r.Error == "" && len(r.Diagnostics) == 0) {
			t.Fatalf("accepted runtime transform: %+v", r)
		}
	}
}
