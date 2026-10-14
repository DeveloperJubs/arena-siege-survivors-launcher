package client

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The game was called Rogue Arena; launcher.ps1's Move-OldNames carries its settings over to the
// new names.

var oldNameRe = regexp.MustCompile(`\bRogue([A-Z_])`)

// moveOldNames carries a Rogue Arena folder's settings over to Arena Siege Survivors' names, once:
// the addons' saved settings (RogueMenu became SiegeMenu, RogueUI SiegeUI, RogueMeter SiegeMeter),
// the per-realm settings (the realm is now "Arena Siege Survivors"), the old game server and the
// desktop shortcut.
func (c *Client) moveOldNames() {
	cfg := c.path("benilla-config")
	for _, name := range []string{"Menu", "UI", "Meter"} {
		old := filepath.Join(cfg, "saved", "Rogue"+name+".lua")
		new := filepath.Join(cfg, "saved", "Siege"+name+".lua")
		if b, err := os.ReadFile(old); err == nil && !exists(new) {
			_ = os.WriteFile(new, oldNameRe.ReplaceAll(b, []byte("Siege$1")), 0o644)
		}
		// The old addon goes, so the menu does not show twice.
		_ = os.RemoveAll(filepath.Join(cfg, "AddOns", "Rogue"+name))
	}

	_ = filepath.WalkDir(cfg, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasPrefix(d.Name(), "Rogue_Arena-") {
			return nil
		}
		new := filepath.Join(filepath.Dir(path), "Arena_Siege_Survivors-"+strings.TrimPrefix(d.Name(), "Rogue_Arena-"))
		if !exists(new) {
			if b, err := os.ReadFile(path); err == nil {
				_ = os.WriteFile(new, b, 0o644)
			}
		}
		return nil
	})

	_ = os.Remove(c.path(oldServerExe))
	if err := renameShortcut("Rogue Arena", shortcutName, c.Root); err != nil {
		c.logf("(Could not rename the desktop shortcut: %v)", err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}
