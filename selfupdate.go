package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"

	"github.com/danotchuuy/RogueArenaLauncher/internal/selfupdate"
)

const (
	launcherRepo  = "danotchuuy/RogueArenaLauncher"
	launcherAsset = "RogueArenaLauncher.exe" // as "make push" uploads it
)

// launcherPath is the running launcher's file.
func launcherPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// cleanupLauncherUpdate removes the old launcher a previous update left behind.
func cleanupLauncherUpdate() {
	if exe, err := launcherPath(); err == nil {
		go selfupdate.Cleanup(exe)
	}
}

// updateLauncher offers a newer launcher if one has been released, and on yes installs it and
// restarts into it. It reports true if the launcher is restarting (the caller should stop).
// With announce, it also says when there is nothing to update. Not on the UI goroutine.
func (l *launcher) updateLauncher(announce bool) bool {
	if !selfupdate.IsRelease(version) {
		if announce {
			l.say(fmt.Sprintf("This launcher is a development build (%s), so it does not update itself.", version))
		}
		return false
	}
	if announce {
		l.say("Checking for launcher updates...")
	}
	rel, err := selfupdate.Latest(context.Background(), launcherRepo, launcherAsset)
	if err != nil {
		l.say("(Could not check for launcher updates: " + err.Error() + ")")
		return false
	}
	if !selfupdate.Newer(version, rel.Version) {
		if announce {
			l.say(fmt.Sprintf("The launcher is up to date (%s).", version))
		}
		return false
	}
	question := fmt.Sprintf("Launcher %s is out (you have %s). Update and restart the launcher now?", rel.Version, version)
	if !l.confirm("Launcher update", question) {
		l.say(fmt.Sprintf("Launcher update to %s skipped.", rel.Version))
		return false
	}

	exe, err := launcherPath()
	if err != nil {
		l.say("Error: " + err.Error())
		l.showError(err)
		return false
	}
	l.say(fmt.Sprintf("Updating the launcher to %s...", rel.Version))
	l.showProgress("Downloading the launcher...", false)
	report := l.progressReporter()
	err = selfupdate.Apply(context.Background(), rel, exe, func(done, total int64) {
		report("Downloading launcher "+rel.Version, done, total)
	})
	l.hideProgress()
	if err != nil {
		l.say("Error: " + err.Error())
		l.showError(fmt.Errorf("the launcher could not be updated: %w", err))
		return false
	}
	if err := selfupdate.Restart(exe, os.Args[1:]); err != nil {
		l.say("The new launcher is installed; close this one and start it again.")
		l.showError(fmt.Errorf("could not start the new launcher: %w", err))
		return false
	}
	l.say("Restarting the launcher...")
	fyne.Do(l.app.Quit)
	return true
}
