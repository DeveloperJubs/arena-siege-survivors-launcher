package client

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const createNoWindow = 0x08000000

// hideWindow keeps a console program from opening a console window of its own.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// createShortcut puts "<name>.lnk" on the desktop, pointing at target.
func createShortcut(name, target, workDir string) error {
	// The paths go in through the environment, so no quoting can break the script.
	const script = `$desk = [Environment]::GetFolderPath("Desktop")
$lnk = (New-Object -ComObject WScript.Shell).CreateShortcut((Join-Path $desk "$env:LNK_NAME.lnk"))
$lnk.TargetPath = $env:LNK_TARGET
$lnk.WorkingDirectory = $env:LNK_DIR
$lnk.Save()`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(),
		"LNK_NAME="+name,
		"LNK_TARGET="+filepath.Clean(target),
		"LNK_DIR="+filepath.Clean(workDir),
	)
	hideWindow(cmd)
	return cmd.Run()
}
