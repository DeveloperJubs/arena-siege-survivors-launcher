package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverExistingInstallations(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "ArenaSiegeSurvivorsClient")
	second := filepath.Join(root, "RogueArena", "RogueArenaClient")
	for _, dir := range []string{first, second} {
		if err := os.MkdirAll(filepath.Join(dir, "bin"), 0755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"benilla.exe", "siege-server.exe"} {
			if err := os.WriteFile(filepath.Join(dir, "bin", name), []byte("fixture"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	os.MkdirAll(filepath.Join(root, "Incomplete", "bin"), 0755)
	os.WriteFile(filepath.Join(root, "Incomplete", "bin", "benilla.exe"), []byte("fixture"), 0644)
	os.MkdirAll(filepath.Join(first, "game", "data"), 0755)
	save := filepath.Join(first, "game", "data", "save.txt")
	os.WriteFile(save, []byte("keep me"), 0600)
	found := Discover([]string{root, first, root, filepath.Join(root, "missing")})
	if len(found) != 2 {
		t.Fatalf("found %v, want two unique complete installs", found)
	}
	b, err := os.ReadFile(save)
	if err != nil || string(b) != "keep me" {
		t.Fatal("discovery changed saved progress")
	}
}
