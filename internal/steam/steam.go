package steam

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// ErrNotInstalled is returned when this PC has no Steam.
var ErrNotInstalled = errors.New("Steam is not installed on this PC")

// Find returns the Steam folder.
func Find() (string, error) {
	for _, dir := range candidates() {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "userdata")); err == nil {
			return dir, nil
		}
	}
	return "", ErrNotInstalled
}

// Running reports whether Steam is running. It keeps its games list in memory and writes it out
// when it exits, so it has to be closed while the list changes.
func Running(ctx context.Context) bool {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return false
	}
	for _, p := range procs {
		if name, err := p.NameWithContext(ctx); err == nil && strings.EqualFold(name, steamProcess) {
			return true
		}
	}
	return false
}

// Shutdown asks Steam to close and waits (up to a minute) until it has.
func Shutdown(ctx context.Context, steamDir string) error {
	if err := exec.Command(steamExe(steamDir), "-shutdown").Start(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	for Running(ctx) {
		select {
		case <-ctx.Done():
			return errors.New("Steam did not close; close it yourself and try again")
		case <-time.After(500 * time.Millisecond):
		}
	}
	// Steam writes its files as it exits: a moment more so it is done with them.
	time.Sleep(time.Second)
	return nil
}

// Start opens Steam again.
func Start(steamDir string) error {
	return exec.Command(steamExe(steamDir)).Start()
}
