//go:build !windows

package steam

import (
	"os"
	"path/filepath"
	"runtime"
)

const steamProcess = "steam"

func candidates() []string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return []string{filepath.Join(home, "Library", "Application Support", "Steam")}
	}
	return []string{
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".local", "share", "Steam"),
	}
}

func steamExe(string) string { return "steam" }
