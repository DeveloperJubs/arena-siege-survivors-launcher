package client

import (
	"archive/zip"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestMain lets the test binary stand in for bin/rogue-server.exe and bin/benilla.exe: the tests
// link those names to it, and it acts by the name it was started as.
func TestMain(m *testing.M) {
	switch filepath.Base(os.Args[0]) {
	case "rogue-server.exe":
		fakeServer()
	case "benilla.exe":
		fakeGame()
	default:
		os.Exit(m.Run())
	}
}

func fakeServer() {
	if len(os.Args) == 3 && os.Args[1] == "add-friend" {
		if os.Args[2] == "Taken" {
			fmt.Println("rogue-server 0.01\nERROR That name is taken.")
			os.Exit(1)
		}
		fmt.Println("rogue-server 0.01\nOK s3cret")
		return
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", ServerPort))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("listening")
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}
}

func fakeGame() {
	cwd, _ := os.Getwd()
	for _, k := range []string{"WOW_DATA", "WOW_HOST", "WOW_USER", "WOW_PASS", "WOW_CHAR", "BENILLA_HOME", "BENILLA_ROGUE_PATCH"} {
		fmt.Printf("%s=%s\n", k, os.Getenv(k))
	}
	fmt.Printf("CWD=%s\n", cwd)
	if !listening(ServerPort) {
		fmt.Fprintln(os.Stderr, "server not up")
		os.Exit(3)
	}
}

// newFolder makes a RogueArenaClient folder whose programs are the fakes above.
func newFolder(t *testing.T) *Client {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake programs are symlinks")
	}
	root := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, exe := range []string{serverExe, gameExe} {
		if err := os.Symlink(self, filepath.Join(root, exe)); err != nil {
			t.Fatal(err)
		}
	}
	return New(root)
}

func TestCharacterName(t *testing.T) {
	for typed, want := range map[string]string{"tHRALL": "Thrall", " jaina ": "Jaina", "Ab": "Ab"} {
		if got, err := CharacterName(typed); err != nil || got != want {
			t.Errorf("CharacterName(%q) = %q, %v; want %q", typed, got, err, want)
		}
	}
	for _, typed := range []string{"", "A", "Thirteenchars", "Bad Name", "Abc1", "Élan"} {
		if _, err := CharacterName(typed); err == nil {
			t.Errorf("CharacterName(%q) accepted", typed)
		}
	}
}

func TestCreateCharacter(t *testing.T) {
	c := newFolder(t)
	if c.HasLogin() {
		t.Fatal("new folder has a login")
	}
	if err := c.CreateCharacter("Taken"); err == nil || err.Error() != "That name is taken." {
		t.Fatalf("taken name: err = %v", err)
	}
	if err := c.CreateCharacter("Thrall"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(c.Root, "login.txt"))
	if string(b) != "Thrall\r\ns3cret" {
		t.Errorf("login.txt = %q", b)
	}
	user, pass, err := c.Login()
	if user != "Thrall" || pass != "s3cret" || err != nil {
		t.Errorf("Login() = %q, %q, %v", user, pass, err)
	}
}

func TestWoWData(t *testing.T) {
	c := New(t.TempDir())
	wow := t.TempDir()
	os.WriteFile(filepath.Join(wow, "dbc.MPQ"), nil, 0o644)
	if err := c.SetWoWData(wow); err == nil || !strings.Contains(err.Error(), "patch.MPQ") {
		t.Fatalf("missing patch.MPQ: err = %v", err)
	}
	os.WriteFile(filepath.Join(wow, "patch.MPQ"), nil, 0o644)
	if err := c.SetWoWData(`"` + wow + `"`); err != nil {
		t.Fatal(err)
	}
	if got := c.WoWData(); got != wow {
		t.Errorf("WoWData() = %q, want %q", got, wow)
	}
	os.Remove(filepath.Join(wow, "dbc.MPQ"))
	if got := c.WoWData(); got != "" {
		t.Errorf("WoWData() with dbc.MPQ gone = %q", got)
	}
}

func TestInstallZip(t *testing.T) {
	root := t.TempDir()
	write := func(rel, s string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(s), 0o644)
	}
	write("login.txt", "mine")
	write("game/data/save.db", "mine")
	write("version.txt", "1")

	zipPath := filepath.Join(t.TempDir(), "u.zip")
	f, _ := os.Create(zipPath)
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		`RogueArenaClient\version.txt`:          "2",
		`RogueArenaClient\login.txt`:            "theirs",
		`RogueArenaClient\game\data\save.db`:    "theirs",
		`RogueArenaClient\game\content\x.txt`:   "new",
		`RogueArenaClient\Play Rogue Arena.bat`: "theirs",
		`RogueArenaClient\game\`:                "",
	} {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	f.Close()

	if err := installZip(zipPath, root); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{
		"version.txt":          "2",
		"login.txt":            "mine",
		"game/data/save.db":    "mine",
		"game/content/x.txt":   "new",
		"Play Rogue Arena.bat": "",
	} {
		b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if string(b) != want {
			t.Errorf("%s = %q, want %q", rel, b, want)
		}
	}
}

func TestPlay(t *testing.T) {
	if listening(ServerPort) {
		t.Skipf("port %d is in use", ServerPort)
	}
	c := newFolder(t)
	if err := c.CreateCharacter("thrall"); err != nil {
		t.Fatal(err)
	}
	wow := t.TempDir()
	os.WriteFile(filepath.Join(wow, "dbc.MPQ"), nil, 0o644)
	os.WriteFile(filepath.Join(wow, "patch.MPQ"), nil, 0o644)
	if err := c.SetWoWData(wow); err != nil {
		t.Fatal(err)
	}

	if err := c.Play(context.Background()); err != nil {
		errs, _ := os.ReadFile(filepath.Join(c.Root, "game", "benilla-errors.txt"))
		t.Fatalf("Play: %v\n%s", err, errs)
	}
	got, _ := os.ReadFile(filepath.Join(c.Root, "game", "benilla-log.txt"))
	for _, want := range []string{
		"WOW_DATA=" + wow,
		"WOW_HOST=127.0.0.1:3725",
		"WOW_USER=thrall",
		"WOW_PASS=s3cret",
		"WOW_CHAR=thrall",
		"BENILLA_HOME=" + filepath.Join(c.Root, "benilla-config"),
		"BENILLA_ROGUE_PATCH=" + filepath.Join(c.Root, "rogue"),
		"CWD=" + filepath.Join(c.Root, "game"),
	} {
		if !strings.Contains(string(got), want+"\n") {
			t.Errorf("game did not get %s; its log:\n%s", want, got)
		}
	}
	if listening(ServerPort) {
		t.Error("the server is still running after the game closed")
	}
}
