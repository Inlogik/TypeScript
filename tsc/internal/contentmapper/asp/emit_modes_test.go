package asp

import (
	"context"
	"testing"
)

func TestEmitAlwaysChecksSemanticsUnlessEraseOnly(t *testing.T) {
	root := t.TempDir()
	input := `<%@ Language=JScript %><% Response.YouWrite('bad'); %>`
	page := writeFixture(t, root, "page.asp.ts", input)
	out, r := EraseASP(context.Background(), page, root, nil)
	if out != "" || len(r.Diagnostics) == 0 {
		t.Fatalf("semantic error ignored: %+v", r)
	}
	out, r = EraseASPWithOptions(context.Background(), page, root, nil, true)
	if r.Error != "" || len(r.Diagnostics) != 0 || out != input {
		t.Fatalf("erase-only failed: %+v", r)
	}
	page = writeFixture(t, root, "broken.asp.ts", `<%@ Language=JScript %><% var x=; %>`)
	out, r = EraseASPWithOptions(context.Background(), page, root, nil, true)
	if out != "" || len(r.Diagnostics) == 0 {
		t.Fatalf("syntax guard removed: %+v", r)
	}
}
