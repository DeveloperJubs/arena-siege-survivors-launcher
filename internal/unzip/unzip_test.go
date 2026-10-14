package unzip

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractRejectsEscapes(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "u.zip")
	f, _ := os.Create(zipPath)
	zw := zip.NewWriter(f)
	zw.Create(`RogueArenaClient\..\..\evil.txt`)
	zw.Close()
	f.Close()
	if err := Extract(context.Background(), zipPath, t.TempDir(), Options{}); err == nil {
		t.Fatal("installed a file outside the folder")
	}
}

func TestCommonTopDir(t *testing.T) {
	for _, tc := range []struct {
		names []string
		want  string
	}{
		{[]string{"A/x", "A/b/y", "A/"}, "A/"},
		{[]string{"A/x", "B/y"}, ""},
		{[]string{"A/x", "top.txt"}, ""},
		{nil, ""},
	} {
		if got := commonTopDir(tc.names); got != tc.want {
			t.Errorf("commonTopDir(%q) = %q, want %q", tc.names, got, tc.want)
		}
	}
}
