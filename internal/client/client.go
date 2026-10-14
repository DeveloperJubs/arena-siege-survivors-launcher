// Package client drives an ArenaSiegeSurvivorsClient folder the way its launcher.ps1 does: updates, the
// first-start setup (character and WoW files), and running the game server and the game.
package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ServerPort is where siege-server listens and benilla connects, on 127.0.0.1.
const ServerPort = 3725

// Client is one ArenaSiegeSurvivorsClient folder.
type Client struct {
	Root string
	// Log receives progress messages meant for the player. It may be nil.
	Log func(string)
}

func New(root string) *Client {
	return &Client{Root: root}
}

func (c *Client) path(elem ...string) string {
	return filepath.Join(append([]string{c.Root}, elem...)...)
}

func (c *Client) logf(format string, args ...any) {
	if c.Log != nil {
		c.Log(fmt.Sprintf(format, args...))
	}
}

// Check reports whether Root looks like an ArenaSiegeSurvivorsClient folder, or a
// RogueArenaClient folder from before the rename (see NeedsUpdate).
func (c *Client) Check() error {
	if !c.exists(gameExe) || !(c.exists(serverExe) || c.exists(oldServerExe)) {
		return fmt.Errorf("%s is not an Arena Siege Survivors folder (no %s and %s)", c.Root, serverExe, gameExe)
	}
	return nil
}

// NeedsUpdate reports whether the folder still has Rogue Arena, the game before it was renamed,
// which must be updated before it can be played.
func (c *Client) NeedsUpdate() bool {
	return !c.exists(serverExe) && c.exists(oldServerExe)
}

func (c *Client) exists(rel string) bool {
	_, err := os.Stat(c.path(rel))
	return err == nil
}

// Release is the release number from release.txt (0.01, 0.02 ...), or "" if unknown.
func (c *Client) Release() string {
	s, _ := readTrimmed(c.path("release.txt"))
	return s
}

// LocalVersion is the build number from version.txt, or 0 if unknown.
func (c *Client) LocalVersion() int64 {
	s, err := readTrimmed(c.path("version.txt"))
	if err != nil {
		return 0
	}
	return parseVersion(s)
}

func parseVersion(s string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func readTrimmed(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
