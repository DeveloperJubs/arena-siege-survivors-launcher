package client

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestMain lets the test binary stand in for bin/siege-server.exe and bin/benilla.exe: the tests
// link those names to it, and it acts by the name it was started as.
func TestMain(m *testing.M) {
	switch filepath.Base(os.Args[0]) {
	case "siege-server.exe":
		fakeServer()
	case "benilla.exe":
		fakeGame()
	default:
		// Never the real desktop.
		createShortcut = func(name, target, workDir string) error { return nil }
		renameShortcut = func(oldName, newName, workDir string) error { return nil }
		os.Exit(m.Run())
	}
}

func fakeServer() {
	if len(os.Args) == 2 && os.Args[1] == "new-player" {
		fmt.Println("siege-server 0.01\nOK Heroabcdefgh s3cret")
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "add-friend" {
		if os.Args[2] == "Taken" {
			fmt.Println("siege-server 0.01\nERROR That name is taken.")
			os.Exit(1)
		}
		fmt.Println("siege-server 0.01\nOK s3cret")
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
	for _, k := range []string{"WOW_DATA", "WOW_HOST", "WOW_USER", "WOW_PASS", "WOW_CHAR", "BENILLA_HOME", "BENILLA_SIEGE_PATCH"} {
		fmt.Printf("%s=%s\n", k, os.Getenv(k))
	}
	fmt.Printf("CWD=%s\n", cwd)
	if !listening(ServerPort) {
		fmt.Fprintln(os.Stderr, "server not up")
		os.Exit(3)
	}
	if os.Getenv("FAKE_GAME_STAYS_OPEN") != "" {
		closing := make(chan os.Signal, 1)
		signal.Notify(closing, syscall.SIGTERM)
		fmt.Println("RUNNING")
		<-closing
		fmt.Println("ASKED TO CLOSE")
	}
}

// newFolder makes an ArenaSiegeSurvivorsClient folder whose programs are the fakes above.
func newFolder(t *testing.T) *Client {
	t.Helper()
	root := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, exe := range []string{serverExe, gameExe} {
		// Windows: a copy under the fake's name (a symlink needs Developer Mode there); it acts
		// by the name it was started as all the same.
		if runtime.GOOS == "windows" {
			b, err := os.ReadFile(self)
			if err == nil {
				err = os.WriteFile(filepath.Join(root, exe), b, 0o755)
			}
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
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

func TestNewPlayer(t *testing.T) {
	c := newFolder(t)
	if err := c.NewPlayer(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(c.Root, "login.txt"))
	if string(b) != "Heroabcdefgh\r\ns3cret" {
		t.Errorf("login.txt = %q", b)
	}
	if !c.HasLogin() {
		t.Error("no login after NewPlayer")
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
		`ArenaSiegeSurvivorsClient\version.txt`:                    "2",
		`ArenaSiegeSurvivorsClient\login.txt`:                      "theirs",
		`ArenaSiegeSurvivorsClient\game\data\save.db`:              "theirs",
		`ArenaSiegeSurvivorsClient\game\content\x.txt`:             "new",
		`ArenaSiegeSurvivorsClient\Play Arena Siege Survivors.bat`: "theirs",
		`ArenaSiegeSurvivorsClient\game\`:                          "",
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
		"version.txt":                    "2",
		"login.txt":                      "mine",
		"game/data/save.db":              "mine",
		"game/content/x.txt":             "new",
		"Play Arena Siege Survivors.bat": "",
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
		"BENILLA_SIEGE_PATCH=" + filepath.Join(c.Root, "siege"),
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

func TestInstall(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		`ArenaSiegeSurvivorsClient\bin\siege-server.exe`:           "server",
		`ArenaSiegeSurvivorsClient\bin\benilla.exe`:                "game",
		`ArenaSiegeSurvivorsClient\version.txt`:                    "202610140845",
		`ArenaSiegeSurvivorsClient\wow-data.txt`:                   "theirs",
		`ArenaSiegeSurvivorsClient\Play Arena Siege Survivors.bat`: "bat",
		`ArenaSiegeSurvivorsClient\game\data\starter.txt`:          "starter",
	} {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+updateZip {
			http.NotFound(w, r)
			return
		}
		w.Write(buf.Bytes())
	}))
	defer srv.Close()
	old := DefaultReleases
	DefaultReleases = srv.URL
	defer func() { DefaultReleases = old }()

	// A folder that does not exist yet, and one where the player already has a WoW folder saved.
	for _, mine := range []bool{false, true} {
		root := filepath.Join(t.TempDir(), "ArenaSiegeSurvivorsClient")
		if mine {
			os.MkdirAll(root, 0o755)
			os.WriteFile(filepath.Join(root, "wow-data.txt"), []byte("mine"), 0o644)
		}
		c := New(root)
		if err := c.Check(); err == nil {
			t.Fatal("empty folder passes Check")
		}
		var reported int64
		if err := c.Install(context.Background(), func(done, total int64) { reported = done }); err != nil {
			t.Fatal(err)
		}
		if reported != int64(buf.Len()) {
			t.Errorf("progress reached %d of %d bytes", reported, buf.Len())
		}
		if c.LocalVersion() != 202610140845 {
			t.Errorf("LocalVersion() = %d", c.LocalVersion())
		}
		want := map[string]string{"Play Arena Siege Survivors.bat": "bat", "game/data/starter.txt": "starter", "wow-data.txt": "theirs"}
		if mine {
			want["wow-data.txt"] = "mine"
		}
		for rel, w := range want {
			if b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))); string(b) != w {
				t.Errorf("mine=%v: %s = %q, want %q", mine, rel, b, w)
			}
		}
	}
}

func TestCloseGame(t *testing.T) {
	if runtime.GOOS == "windows" {
		// The game is asked to close as its window's close button would; the fake game has no
		// window on Windows, so only the forced close could be seen here.
		t.Skip("the fake game has no window to ask")
	}
	if listening(ServerPort) {
		t.Skipf("port %d is in use", ServerPort)
	}
	t.Setenv("FAKE_GAME_STAYS_OPEN", "1")
	c := newFolder(t)
	if err := c.CreateCharacter("Thrall"); err != nil {
		t.Fatal(err)
	}
	wow := t.TempDir()
	os.WriteFile(filepath.Join(wow, "dbc.MPQ"), nil, 0o644)
	os.WriteFile(filepath.Join(wow, "patch.MPQ"), nil, 0o644)
	if err := c.SetWoWData(wow); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Play(ctx) }()
	gameLog := filepath.Join(c.Root, "game", "benilla-log.txt")
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if b, _ := os.ReadFile(gameLog); strings.Contains(string(b), "RUNNING") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the game did not start")
		}
	}

	start := time.Now()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Play: %v", err)
	}
	if took := time.Since(start); took >= GameCloseTimeout {
		t.Errorf("closing took %v: the game was forced instead of asked", took)
	}
	if b, _ := os.ReadFile(gameLog); !strings.Contains(string(b), "ASKED TO CLOSE") {
		t.Errorf("the game was not asked to close; its log:\n%s", b)
	}
	if listening(ServerPort) {
		t.Error("the server is still running after the game closed")
	}
}

func TestOldName(t *testing.T) {
	root := t.TempDir()
	write := func(rel, s string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(s), 0o644)
	}
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "(missing)"
		}
		return string(b)
	}
	// A Rogue Arena folder.
	write("bin/rogue-server.exe", "")
	write("bin/benilla.exe", "")
	write("update.txt", oldReleases+"/\r\n")
	write("benilla-config/saved/RogueMenu.lua", "RogueMenuDB = { RogueUI_Scale = 1, label = \"Rogue Arena\" }")
	write("benilla-config/saved/SiegeUI.lua", "kept")
	write("benilla-config/saved/RogueUI.lua", "RogueUIDB = {}")
	write("benilla-config/AddOns/RogueMeter/RogueMeter.toc", "")
	write("benilla-config/WTF/Account/THRALL/Rogue_Arena-Thrall.lua", "layout")

	c := New(root)
	if err := c.Check(); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !c.NeedsUpdate() {
		t.Error("a Rogue Arena folder does not need an update")
	}
	if got := c.updateBase(); got != DefaultReleases {
		t.Errorf("updateBase() = %q, want %q", got, DefaultReleases)
	}
	if err := c.Play(context.Background()); err == nil || !strings.Contains(err.Error(), "update it first") {
		t.Errorf("Play = %v, want an update-first error", err)
	}

	// The update brings siege-server; the next Play moves the old names over.
	write("bin/siege-server.exe", "")
	if c.NeedsUpdate() {
		t.Error("an updated folder still needs an update")
	}
	c.moveOldNames()
	for rel, want := range map[string]string{
		"benilla-config/saved/SiegeMenu.lua":                                 "SiegeMenuDB = { SiegeUI_Scale = 1, label = \"Rogue Arena\" }",
		"benilla-config/saved/SiegeUI.lua":                                   "kept",
		"benilla-config/WTF/Account/THRALL/Arena_Siege_Survivors-Thrall.lua": "layout",
		"benilla-config/AddOns/RogueMeter/RogueMeter.toc":                    "(missing)",
		"bin/rogue-server.exe":                                               "(missing)",
	} {
		if got := read(rel); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
}
