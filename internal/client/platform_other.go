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

// createDesktopShortcut does nothing outside Windows: the game itself only runs there.
func createDesktopShortcut(name, target, workDir string) error { return nil }

// renameDesktopShortcut does nothing outside Windows, like createShortcut.
func renameDesktopShortcut(oldName, newName, workDir string) error { return nil }

// VCRedistInstalled is true outside Windows, which has no Visual C++ Redistributable to need.
func VCRedistInstalled() bool { return true }
