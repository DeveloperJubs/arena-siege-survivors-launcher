package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// A shortcuts.vdf as Steam writes it, with one game already in it (and a uint64 Steam may add).
func steamFile() []byte {
	other := &node{Type: typeMap, Key: "0", Children: []*node{
		intNode("appid", 0x81234567),
		stringNode("AppName", "Some Emulator"),
		stringNode("Exe", `"C:\Emu\emu.exe"`),
		stringNode("StartDir", `"C:\Emu\"`),
		intNode("LastPlayTime", 1700000000),
		{Type: typeUint64, Key: "Extra", Raw: []byte{1, 2, 3, 4, 5, 6, 7, 8}},
		{Type: typeMap, Key: "tags", Children: []*node{stringNode("0", "Favorites")}},
	}}
	return encode([]*node{{Type: typeMap, Key: "shortcuts", Children: []*node{other}}})
}

func TestRoundTripKeepsEveryByte(t *testing.T) {
	data := steamFile()
	nodes, err := decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encode(nodes), data) {
		t.Fatal("decode then encode changed the file")
	}
}

func TestAddOnceKeepsOtherGames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "shortcuts.vdf")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, steamFile(), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Shortcut{Name: "Arena Siege Survivors", Exe: `C:\Games\ASS\ArenaSiegeSurvivorsLauncher.exe`, StartDir: `C:\Games\ASS`}
	added, err := AddTo(path, s)
	if err != nil || !added {
		t.Fatalf("first add: added=%v err=%v", added, err)
	}
	s.LaunchOptions = "-x"
	if !listedIn(t, path, s.Exe) {
		t.Fatal("not listed after adding")
	}
	if added, err = AddTo(path, s); err != nil || added {
		t.Fatalf("second add must update, not add: added=%v err=%v", added, err)
	}
	data, _ := os.ReadFile(path)
	nodes, err := decode(data)
	if err != nil {
		t.Fatal(err)
	}
	list := (&node{Children: nodes}).child("shortcuts").Children
	if len(list) != 2 {
		t.Fatalf("want the old game and ours, got %d entries", len(list))
	}
	if list[0].str("AppName") != "Some Emulator" || list[0].child("tags").Children[0].Str != "Favorites" {
		t.Fatal("the other game changed")
	}
	ours := list[1]
	if ours.Key != "1" || ours.str("AppName") != s.Name || ours.str("Exe") != `"`+s.Exe+`"` || ours.str("LaunchOptions") != "-x" {
		t.Fatalf("our entry: key %q name %q exe %q opts %q", ours.Key, ours.str("AppName"), ours.str("Exe"), ours.str("LaunchOptions"))
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatal("no backup of Steam's file")
	}
}

func TestAddToANewAccount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "userdata", "42", "config", "shortcuts.vdf")
	if added, err := AddTo(path, Shortcut{Name: "Arena Siege Survivors", Exe: `C:\a.exe`}); err != nil || !added {
		t.Fatalf("added=%v err=%v", added, err)
	}
	data, _ := os.ReadFile(path)
	if _, err := decode(data); err != nil {
		t.Fatal(err)
	}
}

func TestAccountsPutTheLastSignedInFirst(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"0", "111", "222"} {
		_ = os.MkdirAll(filepath.Join(dir, "userdata", id), 0o755)
	}
	_ = os.MkdirAll(filepath.Join(dir, "config"), 0o755)
	// 76561197960265728 + 222
	login := "\"users\"\n{\n\t\"76561197960265950\"\n\t{\n\t\t\"AccountName\"\t\t\"x\"\n\t\t\"MostRecent\"\t\t\"1\"\n\t}\n}\n"
	_ = os.WriteFile(filepath.Join(dir, "config", "loginusers.vdf"), []byte(login), 0o644)
	got := Accounts(dir)
	if len(got) != 2 || got[0] != "222" || got[1] != "111" {
		t.Fatalf("accounts %v", got)
	}
}

func TestAppIDIsSteamsNonSteamID(t *testing.T) {
	if AppID(`C:\a.exe`, "A")&0x80000000 == 0 {
		t.Fatal("non-Steam ids have the top bit set")
	}
}

// listedIn is Listed for a shortcuts.vdf at path, laid out as Steam's userdata/<id>/config.
func listedIn(t *testing.T, path, exe string) bool {
	t.Helper()
	dir := t.TempDir()
	dst := ShortcutsFile(dir, "7")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return Listed(dir, exe) && !Listed(dir, `C:\elsewhere.exe`)
}
