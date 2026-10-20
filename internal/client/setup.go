package client

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	nameRe   = regexp.MustCompile(`^[A-Za-z]{2,12}$`)
	okRe     = regexp.MustCompile(`(?m)^OK (\S+)`)
	errorRe  = regexp.MustCompile(`(?s)^.*ERROR `)
	wowFiles = []string{"dbc.MPQ", "patch.MPQ"}
)

// shortcutName is the desktop shortcut to the launcher.
const shortcutName = "Arena Siege Survivors"

// CharacterName checks a typed character name (2-12 letters) and capitalizes it: "tHRALL" is
// "Thrall".
func CharacterName(typed string) (string, error) {
	typed = strings.TrimSpace(typed)
	if !nameRe.MatchString(typed) {
		return "", errors.New("use 2 to 12 letters, no spaces, digits or symbols")
	}
	return strings.ToUpper(typed[:1]) + strings.ToLower(typed[1:]), nil
}

// HasLogin reports whether a character has been made (login.txt).
func (c *Client) HasLogin() bool {
	_, _, err := c.Login()
	return err == nil
}

// Login is the character name (also the account) and password from login.txt.
func (c *Client) Login() (user, pass string, err error) {
	b, err := os.ReadFile(c.path("login.txt"))
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) == "" || strings.TrimSpace(lines[1]) == "" {
		return "", "", errors.New("login.txt is incomplete")
	}
	return strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1]), nil
}

// CreateCharacter makes the account and character in the game server's saves and writes
// login.txt. name must come from CharacterName.
func (c *Client) CreateCharacter(name string) error {
	if err := os.MkdirAll(c.path("game", "data"), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(c.path(serverExe), "add-friend", name)
	cmd.Dir = c.path("game")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	hideWindow(cmd)
	out, runErr := cmd.Output()

	m := okRe.FindSubmatch(out)
	if m == nil {
		msg := strings.TrimSpace(errorRe.ReplaceAllString(string(out), ""))
		if msg == "" {
			msg = strings.TrimSpace(stderr.String())
		}
		if msg == "" && runErr != nil {
			msg = runErr.Error()
		}
		if msg == "" {
			msg = "the game server did not make the character"
		}
		return errors.New(msg)
	}
	login := name + "\r\n" + string(m[1])
	if err := os.WriteFile(c.path("login.txt"), []byte(login), 0o600); err != nil {
		return err
	}
	_ = os.Remove(c.path("game", "data", "local-login.txt"))
	return nil
}

// newPlayerRe is siege-server new-player's answer: the generated account and its password.
var newPlayerRe = regexp.MustCompile(`(?m)^OK (\S+) (\S+)`)

// NewPlayer makes an account with a hidden, generated name (siege-server new-player) and writes
// login.txt. The player names their hero in game, step by step, the first time it opens.
func (c *Client) NewPlayer() error {
	if err := os.MkdirAll(c.path("game", "data"), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(c.path(serverExe), "new-player")
	cmd.Dir = c.path("game")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	hideWindow(cmd)
	out, runErr := cmd.Output()

	m := newPlayerRe.FindSubmatch(out)
	if m == nil {
		msg := strings.TrimSpace(errorRe.ReplaceAllString(string(out), ""))
		if msg == "" {
			msg = strings.TrimSpace(stderr.String())
		}
		if msg == "" && runErr != nil {
			msg = runErr.Error()
		}
		if msg == "" {
			msg = "the game server did not make the account"
		}
		return errors.New(msg)
	}
	login := string(m[1]) + "\r\n" + string(m[2])
	if err := os.WriteFile(c.path("login.txt"), []byte(login), 0o600); err != nil {
		return err
	}
	_ = os.Remove(c.path("game", "data", "local-login.txt"))
	return nil
}

// ValidWoWData checks that dir is a WoW 1.12.1 "Data" folder.
func ValidWoWData(dir string) error {
	if dir == "" {
		return errors.New("no folder chosen")
	}
	for _, f := range wowFiles {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return fmt.Errorf("that folder has no %s", f)
		}
	}
	return nil
}

// WoWData is the saved WoW "Data" folder from wow-data.txt, or "" if none is saved or it is no
// longer valid.
func (c *Client) WoWData() string {
	dir, _ := readTrimmed(c.path("wow-data.txt"))
	if ValidWoWData(dir) != nil {
		return ""
	}
	return dir
}

// SetWoWData checks and saves the WoW "Data" folder. The first time, it also puts a
// "Arena Siege Survivors" shortcut to this launcher on the desktop.
func (c *Client) SetWoWData(dir string) error {
	dir = strings.Trim(strings.TrimSpace(dir), `"`)
	if dir != "" {
		dir = filepath.Clean(dir)
	}
	if err := ValidWoWData(dir); err != nil {
		return err
	}
	_, statErr := os.Stat(c.path("wow-data.txt"))
	first := errors.Is(statErr, os.ErrNotExist)
	if err := os.WriteFile(c.path("wow-data.txt"), []byte(dir+"\r\n"), 0o644); err != nil {
		return err
	}
	if first {
		if exe, err := os.Executable(); err == nil {
			if err := createShortcut(shortcutName, exe, c.Root); err != nil {
				c.logf("(Could not put a shortcut on the desktop: %v)", err)
			}
		}
	}
	return nil
}
