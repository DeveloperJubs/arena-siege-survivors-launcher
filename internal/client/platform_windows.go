package client

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

// hideWindow keeps a console program from opening a console window of its own.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// askToClose asks a program to close its windows, as their close buttons would.
func askToClose(p *os.Process) error {
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(p.Pid))
	hideWindow(cmd)
	return cmd.Run()
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

// renameShortcut renames the desktop shortcut "<oldName>.lnk" to "<newName>.lnk", if there is one
// that starts in workDir (one made for this folder).
func renameShortcut(oldName, newName, workDir string) error {
	desk, err := desktop()
	if err != nil || !exists(filepath.Join(desk, oldName+".lnk")) {
		return err
	}
	const script = `$desk = [Environment]::GetFolderPath("Desktop")
$old = Join-Path $desk "$env:LNK_OLD.lnk"
$shell = New-Object -ComObject WScript.Shell
$was = $shell.CreateShortcut($old)
if ($was.WorkingDirectory -ne $env:LNK_DIR) { exit 0 }
$lnk = $shell.CreateShortcut((Join-Path $desk "$env:LNK_NEW.lnk"))
$lnk.TargetPath = $was.TargetPath
$lnk.Arguments = $was.Arguments
$lnk.WorkingDirectory = $was.WorkingDirectory
$lnk.Save()
Remove-Item -Force $old`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(),
		"LNK_OLD="+oldName,
		"LNK_NEW="+newName,
		"LNK_DIR="+filepath.Clean(workDir),
	)
	hideWindow(cmd)
	return cmd.Run()
}

// desktop is the current user's desktop folder.
func desktop() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Desktop, 0)
}
