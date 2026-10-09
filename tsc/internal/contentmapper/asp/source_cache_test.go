package asp

import (
	"os"
	"testing"
)

func TestSourceCacheRefreshesAndOverlaysDoNotContaminate(t *testing.T) {
	root := t.TempDir()
	page := writeFixture(t, root, "page.asp", "<%@ Language=JScript %><% var value=1; %>")
	cache := NewSourceCache()
	first, err := mapSources(page, root, nil, false, cache)
	if err != nil {
		t.Fatal(err)
	}
	overlay := map[string]string{page: "<%@ Language=JScript %><% var value='overlay'; %>"}
	changed, err := mapSources(page, root, overlay, false, cache)
	if err != nil || changed.Text == first.Text {
		t.Fatalf("overlay missing: %v", err)
	}
	again, err := mapSources(page, root, nil, false, cache)
	if err != nil || again.Text != first.Text {
		t.Fatal("overlay contaminated cache")
	}
	if err = os.WriteFile(page, []byte("<%@ Language=JScript %><% var value='changed on disk'; %>"), 0600); err != nil {
		t.Fatal(err)
	}
	again, err = mapSources(page, root, nil, false, cache)
	if err != nil || again.Text == first.Text {
		t.Fatal("disk change missed")
	}
}
