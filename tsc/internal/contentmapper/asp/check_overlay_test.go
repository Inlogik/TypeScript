package asp

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestCheckUsesUnsavedIncludeOverlay(t *testing.T) {
	root := t.TempDir()
	part := writeFixture(t, root, "part.inc", "<% Response.Write('disk'); %>")
	page := writeFixture(t, root, "page.asp", `<%@ Language=JScript %><!-- #include file="part.inc" -->`)
	file := writeFixture(t, root, "overlay.json", "{}")
	data, _ := json.Marshal(map[string]string{part: "<% Response.YouWrite('unsaved'); %>"})
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	r := CheckReportWithOverlay(context.Background(), page, root, nil, "", "", false, file)
	if r.Error != "" || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != 2339 || r.Diagnostics[0].File != part {
		t.Fatalf("%+v", r)
	}
	d, err := Check(context.Background(), page, root)
	if err != nil || len(d) != 0 {
		t.Fatalf("disk source modified: %v %+v", err, d)
	}
}
