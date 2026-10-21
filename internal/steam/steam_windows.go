package steam

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const steamProcess = "steam.exe"

// candidates: where Steam says it is, then its usual folders.
func candidates() []string {
	var dirs []string
	for _, k := range []struct {
		root registry.Key
		path string
		name string
	}{
		{registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, "InstallPath"},
		{registry.LOCAL_MACHINE, `SOFTWARE\Valve\Steam`, "InstallPath"},
	} {
		key, err := registry.OpenKey(k.root, k.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		if v, _, err := key.GetStringValue(k.name); err == nil {
			dirs = append(dirs, filepath.Clean(v))
		}
		key.Close()
	}
	return append(dirs,
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Steam"),
		filepath.Join(os.Getenv("ProgramFiles"), "Steam"))
}

func steamExe(dir string) string { return filepath.Join(dir, "steam.exe") }
