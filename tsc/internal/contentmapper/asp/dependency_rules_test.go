package asp

import (
	"context"
	"testing"
)

func TestConditionalIncludeRuleScope(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "shared.inc", `<% function validate(){ parseBody(); } function other(){ parseBody(); } %>`)
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="shared.inc" -->`)
	rules := writeFixture(t, root, "rules.json", `[{"source":"shared.inc","consumer":"validate","names":["parseBody"],"provider":"upload.inc","condition":"multipart POST","requiredEntries":["upload.asp"]}]`)
	d, err := CheckWithDependencyRules(context.Background(), page, root, nil, rules)
	if err != nil || len(d) != 2 || d[0].Warning || !d[1].Warning {
		t.Fatalf("%v %+v", err, d)
	}
	page = writeFixture(t, root, "upload.asp", `<%@ Language=JScript %><!-- #include file="shared.inc" -->`)
	d, err = CheckWithDependencyRules(context.Background(), page, root, nil, rules)
	if err != nil || len(d) != 2 || d[0].Warning || d[1].Warning {
		t.Fatalf("required entry: %v %+v", err, d)
	}
	writeFixture(t, root, "upload.inc", `<% function parseBody(){return true;} %>`)
	page = writeFixture(t, root, "provided.asp", `<%@ Language=JScript %><!-- #include file="shared.inc" --><!-- #include file="upload.inc" -->`)
	d, err = CheckWithDependencyRules(context.Background(), page, root, nil, rules)
	if err != nil || len(d) != 0 {
		t.Fatalf("provider present: %v %+v", err, d)
	}
}

func TestDependencyRulesRequireValidConfig(t *testing.T) {
	root := t.TempDir()
	for _, text := range []string{`[{"source":"../outside","consumer":"f","names":["x"],"provider":"p.inc","condition":"post"}]`, `[{"source":"shared.inc"}]`, `[{"unknown":true}]`} {
		file := writeFixture(t, root, "invalid.json", text)
		if _, err := loadDependencyRules(file, root); err == nil {
			t.Fatal(text)
		}
	}
}
