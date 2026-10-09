package asp

import (
	"context"
	"strings"
	"testing"
)

func TestMissingIncludeReportsLocationAndChecksRemainingSources(t *testing.T) {
	root := t.TempDir()
	helper := writeFixture(t, root, "helper.inc", `<% function helper(value){return value;} Response.YouWrite("include"); %>`)
	page := writeFixture(t, root, "page.asp", "<%@ Language=JScript %>\n<!--#include file=\"missing.inc\" -->\n<!--#include file=\"helper.inc\" -->\n<% helper(); Response.YouWrite('page'); unknownMethod(); %>")
	r := CheckReport(context.Background(), page, root, nil, "", "", true)
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	includeError, pageError, helperError, missingName := false, false, false, false
	for _, d := range r.Diagnostics {
		if d.Code == 95002 && d.File == page && d.Line == 2 && !d.Warning {
			includeError = true
		}
		if d.Code == 2339 && d.File == page {
			pageError = true
		}
		if d.Code == 2339 && d.File == helper {
			helperError = true
		}
		if d.Code == 2304 {
			missingName = true
		}
	}
	if !includeError || !pageError || !helperError || !missingName {
		t.Fatalf("missing recovery diagnostics: %+v", r)
	}
	if len(r.Dependencies) != 2 {
		t.Fatalf("dependencies: %+v", r.Dependencies)
	}
	if _, err := Map(page, root); err == nil {
		t.Fatal("strict mapping accepted missing include")
	}
	v := PrepareVirtual(page, root, nil, "")
	if v.Error != "" || len(v.Sources) != 2 {
		t.Fatalf("virtual recovery: %+v", v)
	}
}

func TestMissingIncludeStillBlocksEmitOnError(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "page.asp.ts", `<%@ Language=JScript %><!--#include file="missing.inc" --><% const value:number=1; %>`)
	text, report := EraseASPWithCache(context.Background(), page, root, nil, false, nil, true)
	if text != "" || report.Error == "" || report.EmissionSafe {
		t.Fatalf("unsafe emission: %q %+v", text, report)
	}
}

func TestPlainIncludeStandaloneEditorContext(t *testing.T) {
	root := t.TempDir()
	file := writeFixture(t, root, "helper.inc", `<% function helper(value){return value;} helper(1); %>`)
	v := PrepareVirtual(file, root, nil, "")
	if v.Error != "" || !strings.Contains(v.Text, "function helper") {
		t.Fatalf("standalone include: %+v", v)
	}
}
