package main

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/danotchuuy/arena-siege-survivors-launcher/internal/steam"
)

//go:embed assets/steam.svg
var steamSVG []byte

var steamIcon = fyne.NewStaticResource("steam.svg", steamSVG)

// steamName is what Steam lists, and what friends see you playing.
const steamName = "Arena Siege Survivors"

// addToSteam lists this launcher in the player's Steam library as a non-Steam game. Started from
// Steam, it shows their friends "Arena Siege Survivors" for as long as they play (the launcher
// stays open while the game runs). Steam only reads its list when it starts, so a running Steam
// is closed and opened again, after asking.
func (l *launcher) addToSteam() {
	info := func(msg string) {
		fyne.Do(func() { dialog.ShowInformation("Add to Steam", msg, l.win) })
	}
	dir, err := steam.Find()
	if err != nil {
		l.say("Add to Steam: " + err.Error())
		info("Steam isn't installed on this PC. Install Steam, sign in once, then press Add to Steam again.")
		return
	}
	accounts := steam.Accounts(dir)
	if len(accounts) == 0 {
		info("Sign in to Steam once, then press Add to Steam again.")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		l.say("Add to Steam: " + err.Error())
		return
	}
	s := steam.Shortcut{Name: steamName, Exe: exe, StartDir: filepath.Dir(exe), Icon: exe}
	// Steam shows the logo beside the name; it reads it from a file.
	icon := filepath.Join(filepath.Dir(exe), "ArenaSiegeSurvivors-steam.png")
	if err := os.WriteFile(icon, launcherLogo, 0o644); err == nil {
		s.Icon = icon
	}
	if l.explicitRoot {
		s.LaunchOptions = `-root "` + strings.Trim(l.client.Root, `"`) + `"`
	}

	ctx := context.Background()
	wasRunning := steam.Running(ctx)
	if wasRunning && !l.confirm("Add to Steam", "Steam has to restart to show Arena Siege Survivors in your library.\n\nRestart Steam now?") {
		return
	}
	if wasRunning {
		l.say("Closing Steam...")
		if err := steam.Shutdown(ctx, dir); err != nil {
			l.say("Add to Steam: " + err.Error())
			info(err.Error() + ".")
			return
		}
	}
	added := false
	for _, account := range accounts {
		isNew, err := steam.AddTo(steam.ShortcutsFile(dir, account), s)
		if err != nil {
			l.say("Add to Steam: " + err.Error())
			info("Could not add the game to Steam: " + err.Error())
			if wasRunning {
				_ = steam.Start(dir)
			}
			return
		}
		added = added || isNew
	}
	if wasRunning {
		l.say("Starting Steam...")
		if err := steam.Start(dir); err != nil {
			l.say("Could not start Steam: " + err.Error())
		}
	}
	l.showSteamState()
	if added {
		l.say("Added Arena Siege Survivors to Steam.")
	} else {
		l.say("Arena Siege Survivors was already in Steam; its entry is up to date.")
	}
	info("Arena Siege Survivors is in your Steam library.\n\nStart it from Steam (Library > Arena Siege Survivors) and your friends see you playing it.")
}

// showSteamState labels the button by whether this launcher is already in Steam: it only ever
// needs pressing once. Safe from any goroutine.
func (l *launcher) showSteamState() {
	text := "Add to Steam"
	if dir, err := steam.Find(); err == nil {
		if exe, err := os.Executable(); err == nil && steam.Listed(dir, exe) {
			text = "In your Steam library"
		}
	}
	fyne.Do(func() { l.steamBtn.SetText(text) })
}
