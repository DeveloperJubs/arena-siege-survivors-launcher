package stonetavern

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestDefaultEdition(t *testing.T) {
	eds := []Edition{{Name: "Enhanced", Recommended: true, SHA256: strings.Repeat("a", 64)}, {Name: "Classic", SHA256: strings.Repeat("b", 64)}}
	ed, err := DefaultEdition(eds)
	if err != nil || ed.Name != "Classic" {
		t.Fatalf("default = %v, %v", ed, err)
	}
	for _, bad := range [][]Edition{nil, eds[:1], {{Name: "Classic"}}, {{Name: "Classic", SHA256: "not-a-checksum"}}} {
		if _, err := DefaultEdition(bad); err == nil {
			t.Fatal("unverified default accepted")
		}
	}
}

func TestLiveDefaultEdition(t *testing.T) {
	if os.Getenv("STONETAVERN_LIVE_TEST") == "" {
		t.Skip("optional live metadata check")
	}
	eds, err := FetchEditions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ed, err := DefaultEdition(eds)
	if err != nil {
		t.Fatal(err)
	}
	if ed.URL == "" || ed.Build == "" {
		t.Fatal("live client metadata incomplete")
	}
	t.Logf("Verified live Classic build %s, %s", ed.Build, ed.Size)
}
