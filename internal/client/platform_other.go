//go:build !windows

package client

import "os/exec"

func hideWindow(*exec.Cmd) {}

// createShortcut does nothing outside Windows: the game itself only runs there.
func createShortcut(name, target, workDir string) error { return nil }
