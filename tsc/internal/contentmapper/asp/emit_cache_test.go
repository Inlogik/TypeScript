package asp

import (
	"context"
	"reflect"
	"testing"
)

func TestCachedErasurePreservesFullDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "shared.inc", "<% Response.YouWrite('bad'); %>")
	page := writeFixture(t, root, "page.asp.ts", `<%@ Language=JScript %><!-- #include file="shared.inc" -->`)
	out, want := EraseASP(context.Background(), page, root, nil)
	cache := NewSourceCache()
	for i := 0; i < 2; i++ {
		gotText, got := EraseASPWithCache(context.Background(), page, root, nil, false, cache)
		if gotText != out || !reflect.DeepEqual(got, want) {
			t.Fatalf("cached validation changed: %+v vs %+v", got, want)
		}
	}
}
