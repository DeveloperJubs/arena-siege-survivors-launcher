package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// GameCloseTimeout is how long the game gets to close when Play's ctx is cancelled.
const GameCloseTimeout = 5 * time.Second

var (
	serverExe = filepath.Join("bin", "rogue-server.exe")
	gameExe   = filepath.Join("bin", "benilla.exe")
)

// Play runs the game server in the background and then the game, logged straight in as the
// player's character, and returns when the game closes (or ctx is cancelled, which closes it).
// The server is always stopped on return.
func (c *Client) Play(ctx context.Context) error {
	user, pass, err := c.Login()
	if err != nil {
		return errors.New("no character yet: name one first")
	}
	wow := c.WoWData()
	if wow == "" {
		return errors.New("no WoW 1.12.1 Data folder chosen yet")
	}
	game := c.path("game")
	if err := os.MkdirAll(filepath.Join(game, "data"), 0o755); err != nil {
		return err
	}
	c.removeLeftovers()
	if err := c.stopOldServers(ctx); err != nil {
		return err
	}

	c.logf("Game files: %s", wow)
	c.logf("Starting... the game opens in its own window in a moment and goes straight into the start menu.")
	c.logf(`Close the game to stop. (Messages go to game\server-log.txt and game\benilla-log.txt.)`)

	stopServer, err := c.startServer(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil // stopped before the game started
		}
		return err
	}
	defer stopServer()

	// The game: the player's WoW files (read only), this game's ability changes mounted on top
	// of them (rogue\), its addons and settings in benilla-config\.
	// Cancelling ctx asks the game to close, as its window's close button would, and forces it
	// if it is still open GameCloseTimeout later.
	cmd := exec.CommandContext(ctx, c.path(gameExe))
	cmd.Cancel = func() error { return askToClose(cmd.Process) }
	cmd.WaitDelay = GameCloseTimeout
	cmd.Dir = game
	cmd.Env = append(os.Environ(),
		"WOW_DATA="+wow,
		"WOW_HOST=127.0.0.1:"+strconv.Itoa(ServerPort),
		"WOW_USER="+user,
		"WOW_PASS="+pass,
		"WOW_CHAR="+user,
		"BENILLA_HOME="+c.path("benilla-config"),
		"BENILLA_ROGUE_PATCH="+c.path("rogue"),
	)
	closeLogs, err := redirect(cmd, filepath.Join(game, "benilla-log.txt"), filepath.Join(game, "benilla-errors.txt"))
	if err != nil {
		return err
	}
	defer closeLogs()
	hideWindow(cmd)

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf(`the game closed with an error (%v). game\benilla-errors.txt says what went wrong; send it to whoever gave you the game`, err)
	}
	return nil
}

// removeLeftovers deletes files from the old browser version of the game.
func (c *Client) removeLeftovers() {
	_ = os.Remove(c.path("bin", "wenilla-host.exe"))
	_ = os.RemoveAll(c.path("web"))
	_ = os.RemoveAll(c.path("rogue", "AddOns"))
}

// stopOldServers makes sure nothing else holds the game's port. A rogue-server left running from
// this folder is stopped; one from another copy of Rogue Arena would not know this character, so
// the player is asked to close it.
func (c *Client) stopOldServers(ctx context.Context) error {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return fmt.Errorf("could not list running programs: %w", err)
	}
	server := c.path(serverExe)
	for _, p := range procs {
		name, err := p.NameWithContext(ctx)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(strings.ToLower(name), ".exe") {
		case "benilla":
			return errors.New("Rogue Arena is already running. Close the other game window first, then try again")
		case "rogue-server":
			exe, _ := p.ExeWithContext(ctx)
			if exe != "" && samePath(exe, server) {
				_ = p.KillWithContext(ctx)
				continue
			}
			return fmt.Errorf("another copy of Rogue Arena is still running in the background (%s). Close its window (or restart the PC), then try again", exe)
		}
	}
	time.Sleep(300 * time.Millisecond)
	if listening(ServerPort) {
		return fmt.Errorf("something else on this PC is using port %d. Close other Rogue Arena or WoW server windows, then try again", ServerPort)
	}
	return nil
}

// startServer runs rogue-server with its saves in game\data and waits until it takes connections.
// The returned func stops it.
func (c *Client) startServer(ctx context.Context) (stop func(), err error) {
	game := c.path("game")
	cmd := exec.Command(c.path(serverExe))
	cmd.Dir = game
	closeLogs, err := redirect(cmd, filepath.Join(game, "server-log.txt"), filepath.Join(game, "server-errors.txt"))
	if err != nil {
		return nil, err
	}
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		closeLogs()
		return nil, fmt.Errorf("the game server could not start: %w", err)
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		closeLogs()
		close(exited)
	}()
	stop = func() {
		_ = cmd.Process.Kill()
		<-exited
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if listening(ServerPort) {
			return stop, nil
		}
		select {
		case <-exited:
			return nil, errors.New(`the game server could not start (see game\server-errors.txt)`)
		case <-ctx.Done():
			stop()
			return nil, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	stop()
	return nil, errors.New(`the game server did not start in time (see game\server-errors.txt)`)
}

// redirect sends cmd's output and errors to the two files, replacing what they held before.
func redirect(cmd *exec.Cmd, stdout, stderr string) (closeFn func(), err error) {
	out, err := os.Create(stdout)
	if err != nil {
		return nil, err
	}
	errf, err := os.Create(stderr)
	if err != nil {
		out.Close()
		return nil, err
	}
	cmd.Stdout, cmd.Stderr = out, errf
	return func() { out.Close(); errf.Close() }, nil
}

func listening(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
