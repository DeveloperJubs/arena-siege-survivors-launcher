package steam

import (
	"bufio"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Shortcut is the game as Steam lists it.
type Shortcut struct {
	Name          string // what friends see you playing
	Exe           string // the program Steam starts (and watches: you are "playing" while it runs)
	StartDir      string
	Icon          string
	LaunchOptions string
}

// AppID is the id Steam gives a non-Steam game: its library art is filed under it
// (config/grid/<id>p.png and so on).
func AppID(exe, name string) uint32 {
	return crc32.ChecksumIEEE([]byte(quote(exe)+name)) | 0x80000000
}

func quote(path string) string { return `"` + strings.Trim(path, `"`) + `"` }

// AddTo adds s to the shortcuts.vdf at path (made if missing). An entry for the same program is
// updated in place instead, so pressing the button twice never lists the game twice. It reports
// whether the game was new.
func AddTo(path string, s Shortcut) (bool, error) {
	var top []*node
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if top, err = decode(data); err != nil {
			return false, err
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return false, err
	}
	list := (&node{Children: top}).child("shortcuts")
	if list == nil {
		list = &node{Type: typeMap, Key: "shortcuts"}
		top = append(top, list)
	}
	entry := func(e *node) {
		set := func(n *node) {
			if old := e.child(n.Key); old != nil {
				*old = *n
			} else {
				e.Children = append(e.Children, n)
			}
		}
		set(intNode("appid", AppID(s.Exe, s.Name)))
		set(stringNode("AppName", s.Name))
		set(stringNode("Exe", quote(s.Exe)))
		set(stringNode("StartDir", quote(s.StartDir)))
		set(stringNode("icon", s.Icon))
		set(stringNode("LaunchOptions", s.LaunchOptions))
		for _, key := range []string{"AllowDesktopConfig", "AllowOverlay"} {
			if e.child(key) == nil {
				e.Children = append(e.Children, intNode(key, 1))
			}
		}
		if e.child("tags") == nil {
			e.Children = append(e.Children, &node{Type: typeMap, Key: "tags"})
		}
	}
	for _, e := range list.Children {
		if e.Type == typeMap && strings.EqualFold(strings.Trim(e.str("Exe"), `"`), strings.Trim(s.Exe, `"`)) {
			entry(e)
			return false, save(path, top)
		}
	}
	// Steam numbers its entries "0", "1", ...: the next free number.
	next := 0
	for _, e := range list.Children {
		if i, err := strconv.Atoi(e.Key); err == nil && i >= next {
			next = i + 1
		}
	}
	e := &node{Type: typeMap, Key: strconv.Itoa(next)}
	entry(e)
	list.Children = append(list.Children, e)
	return true, save(path, top)
}

func save(path string, top []*node) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Steam's own copy stays beside it, once: shortcuts.vdf.bak.
	if old, err := os.ReadFile(path); err == nil {
		if _, err := os.Stat(path + ".bak"); errors.Is(err, os.ErrNotExist) {
			_ = os.WriteFile(path+".bak", old, 0o644)
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, encode(top), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Accounts lists the Steam accounts on this PC (userdata/<account id>), the one that signed in
// last first.
func Accounts(steamDir string) []string {
	entries, err := os.ReadDir(filepath.Join(steamDir, "userdata"))
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		if n, err := strconv.ParseUint(e.Name(), 10, 32); err == nil && n != 0 && e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	if last := lastAccount(steamDir); last != "" {
		for i, id := range ids {
			if id == last {
				ids = append([]string{id}, append(ids[:i:i], ids[i+1:]...)...)
				break
			}
		}
	}
	return ids
}

var steamID = regexp.MustCompile(`^\s*"(\d{17})"\s*$`)
var mostRecent = regexp.MustCompile(`(?i)^\s*"MostRecent"\s*"1"\s*$`)

// lastAccount is the account id of the user loginusers.vdf marks MostRecent ("" if none).
func lastAccount(steamDir string) string {
	f, err := os.Open(filepath.Join(steamDir, "config", "loginusers.vdf"))
	if err != nil {
		return ""
	}
	defer f.Close()
	current := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := steamID.FindStringSubmatch(sc.Text()); m != nil {
			current = m[1]
		} else if mostRecent.MatchString(sc.Text()) && current != "" {
			id64, err := strconv.ParseUint(current, 10, 64)
			if err != nil || id64 < 76561197960265728 {
				return ""
			}
			return strconv.FormatUint(id64-76561197960265728, 10)
		}
	}
	return ""
}

// ShortcutsFile is where account's non-Steam games are listed.
func ShortcutsFile(steamDir, account string) string {
	return filepath.Join(steamDir, "userdata", account, "config", "shortcuts.vdf")
}

// Listed reports whether exe is already in any account's Steam library on this PC.
func Listed(steamDir, exe string) bool {
	for _, account := range Accounts(steamDir) {
		data, err := os.ReadFile(ShortcutsFile(steamDir, account))
		if err != nil {
			continue
		}
		top, err := decode(data)
		if err != nil {
			continue
		}
		list := (&node{Children: top}).child("shortcuts")
		if list == nil {
			continue
		}
		for _, e := range list.Children {
			if e.Type == typeMap && strings.EqualFold(strings.Trim(e.str("Exe"), `"`), strings.Trim(exe, `"`)) {
				return true
			}
		}
	}
	return false
}
