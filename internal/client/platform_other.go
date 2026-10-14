//go:build !windows

package client

import (
	"os"
	"os/exec"
	"syscall"
)

func hideWindow(*exec.Cmd) {}

// askToClose asks a program to close.
func askToClose(p *os.Process) error { return p.Signal(syscall.SIGTERM) }

// createShortcut does nothing outside Windows: the game itself only runs there.
func createShortcut(name, target, workDir string) error { return nil }
