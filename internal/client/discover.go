package client

import (
	"os"
	"path/filepath"
	"strings"
)

// Discover checks likely install locations, without traversing entire drives or
// following directory links. It only reads paths; it never runs found programs.
func Discover(roots []string) []string {
	var found []string
	seen := map[string]bool{}
	checked := map[string]bool{}
	check := func(dir string) {
		dir = filepath.Clean(dir)
		key := strings.ToLower(dir)
		if checked[key] {
			return
		}
		checked[key] = true
		if New(dir).Check() != nil {
			return
		}
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
		key = strings.ToLower(dir)
		if !seen[key] {
			seen[key] = true
			found = append(found, dir)
		}
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		check(root)
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		if len(entries) > 512 {
			entries = entries[:512]
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			dir := filepath.Join(root, entry.Name())
			check(dir)
			name := strings.ToLower(entry.Name())
			if !strings.Contains(name, "arena") && !strings.Contains(name, "siege") && !strings.Contains(name, "rogue") {
				continue
			}
			children, _ := os.ReadDir(dir)
			for i, child := range children {
				if i >= 128 {
					break
				}
				if child.IsDir() && child.Type()&os.ModeSymlink == 0 {
					check(filepath.Join(dir, child.Name()))
				}
			}
		}
	}
	return found
}
