package main

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/danotchuuy/arena-siege-survivors-launcher/internal/stonetavern"
	"os"
	"path/filepath"
)

func (l *launcher) installFailed() {
	fyne.Do(func() {
		l.summary.SetText("Installation stopped. Press Retry install to continue. Your saves are kept.")
		l.play.SetText("Retry install")
		l.enableButtons(true)
	})
}

// The primary path approves both downloads once; advanced choices remain optional.
func (l *launcher) installWelcome(dir string) bool {
	answer := make(chan bool, 1)
	fyne.Do(func() {
		l.summary.SetText("Install the game and its required files in one step.")
		folder := widget.NewEntry()
		folder.SetText(dir)
		browse := widget.NewButton("Change folder", func() {
			d := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
				if err == nil && u != nil {
					folder.SetText(u.Path())
				}
			}, l.win)
			d.Show()
		})
		advanced := widget.NewAccordion(widget.NewAccordionItem("Change install location", container.NewVBox(folder, browse)))
		info := widget.NewLabel("Downloads Arena Siege Survivors and the Classic 1.12.1 files from Stonetavern (about 5 GB plus the game). Allow at least 20 GB of free space.\n\nInterrupted client downloads can resume. No account or client folder to configure. Create your hero in game.")
		info.Wrapping = fyne.TextWrapWord
		d := dialog.NewCustomConfirm("Ready for your first run?", "Install", "Later", container.NewVBox(info, advanced), func(ok bool) {
			if ok {
				v := filepath.Clean(folder.Text)
				if !filepath.IsAbs(v) {
					l.showError(fmt.Errorf("choose a full install folder path"))
					answer <- false
					return
				}
				l.client.Root = v
				l.setupApproved = true
			}
			answer <- ok
		}, l.win)
		d.Resize(fyne.NewSize(590, 350))
		d.Show()
	})
	if !<-answer {
		l.installFailed()
		return false
	}
	return true
}

func (l *launcher) findClient() bool {
	dir := l.client.Root
	if !l.explicitRoot {
		if base, err := os.UserCacheDir(); err == nil {
			dir = filepath.Join(base, "ArenaSiegeSurvivors", "Game")
		}
	}
	// Keep the install location remembered even if a download is interrupted.
	if !l.explicitRoot {
		if saved := l.app.Preferences().String(prefRoot); saved != "" {
			dir = saved
		}
	}
	if !l.installWelcome(dir) {
		return false
	}
	l.rememberRoot()
	if err := l.installGame(l.client.Root); err != nil {
		l.installFailed()
		l.showError(err)
		return false
	}
	return true
}

func (l *launcher) downloadDefaultClient() bool {
	fyne.Do(func() { l.summary.SetText("Installing required files · Step 2 of 3") })
	eds, err := stonetavern.FetchEditions(context.Background())
	if err != nil {
		l.showError(err)
		return false
	}
	ed := stonetavern.Find(eds, "Classic")
	if ed == nil || ed.SHA256 == "" {
		l.showError(fmt.Errorf("Stonetavern's verified Classic 1.12.1 download is unavailable. Try again later or choose an existing client in Options"))
		return false
	}
	dir := filepath.Join(l.client.Root, stonetavern.FolderName(*ed))
	if err = l.installClient(*ed, dir); err != nil {
		return false
	}
	if err = l.client.SetWoWData(stonetavern.DataDir(dir)); err != nil {
		l.showError(err)
		return false
	}
	return true
}
