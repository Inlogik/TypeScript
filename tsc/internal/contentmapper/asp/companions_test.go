package asp

import (
	"context"
	"strings"
	"testing"
)

func TestCompanionExternalGlobalIsTypedAndContextual(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "helper.inc", "<% Response.Write(details[0].Name); Response.Write(details[0].Banana); %>")
	companion := writeFixture(t, root, "helper.inc.d.ts", "declare var details: {Name:string}[];")
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="helper.inc" -->`)
	r := CheckReport(context.Background(), page, root, nil, "", "", false)
	if r.Error != "" || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != 2339 {
		t.Fatalf("%+v", r)
	}
	found := false
	for _, f := range r.Dependencies {
		if f == companion {
			found = true
		}
	}
	if !found {
		t.Fatal("companion not tracked")
	}
	other := writeFixture(t, root, "other.asp", `<%@ Language=JScript %><% Response.Write(details); %>`)
	d, err := Check(context.Background(), other, root)
	if err != nil || len(d) != 1 || d[0].Code != 2304 {
		t.Fatalf("companion leaked: %v %+v", err, d)
	}
	v := PrepareVirtual(page, root, nil, "")
	if v.Error != "" || !strings.Contains(v.Declarations, "declare var details") {
		t.Fatalf("IntelliSense companion missing: %+v", v)
	}
}

func TestSharedGlobalsUseResolvedVariablesAndSourceScopedContracts(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "provider.inc", "<% var shared=1; function provided(){} %>")
	writeFixture(t, root, "consumer.inc", "<% function consume(local){var own=1; shared=own; return shared+local;} provided(); Response.Write(shared); %>")
	writeFixture(t, root, "sibling.inc", "<% Response.Write(shared); %>")
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="provider.inc" --><!-- #include file="consumer.inc" --><!-- #include file="sibling.inc" -->`)
	r := CheckReportWithOverlay(context.Background(), page, root, nil, "", "", false, "", true)
	if r.Error != "" || len(r.Diagnostics) != 4 {
		t.Fatalf("%+v", r)
	}
	for _, d := range r.Diagnostics {
		if d.Code != 90001 || !d.Warning {
			t.Fatal(d)
		}
	}
	writeFixture(t, root, "consumer.inc.d.ts", "declare var shared: number;")
	r = CheckReportWithOverlay(context.Background(), page, root, nil, "", "", false, "", true)
	sharedWarnings := 0
	for _, d := range r.Diagnostics {
		if d.Code == 90001 {
			sharedWarnings++
			if !strings.HasSuffix(d.File, "sibling.inc") {
				t.Fatal(d)
			}
		}
	}
	if r.Error != "" || sharedWarnings != 1 {
		t.Fatalf("contract scope incorrect: %+v", r)
	}
}

func TestSharedGlobalLintDoesNotFlagShadowedLocals(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "provider.inc", "<% var shared=1; %>")
	writeFixture(t, root, "consumer.inc", "<% function f(shared){return shared;} function g(){var shared=2; return shared;} %>")
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="provider.inc" --><!-- #include file="consumer.inc" -->`)
	r := CheckReportWithOverlay(context.Background(), page, root, nil, "", "", false, "", true)
	if r.Error != "" || len(r.Diagnostics) != 0 {
		t.Fatalf("%+v", r)
	}
}
