package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/danotchuuy/arena-siege-survivors-launcher/internal/client"
)

// prefRoot remembers the Arena Siege Survivors folder when the launcher is not inside it.
const prefRoot = "clientRoot"

// locateClient makes sure l.client points at an Arena Siege Survivors folder: the launcher's own folder, the
// one used last time, or one the player downloads or picks. It reports false if there is none.
// Not on the UI goroutine.
func (l *launcher) locateClient() bool {
	if l.client.Check() == nil {
		l.rememberRoot()
		return true
	}
	if !l.explicitRoot {
		if saved := l.app.Preferences().String(prefRoot); saved != "" && client.New(saved).Check() == nil {
			l.client.Root = saved
			return true
		}
	}
	if !l.findClient() {
		return false
	}
	l.rememberRoot()
	return true
}

func (l *launcher) rememberRoot() {
	if !l.explicitRoot {
		l.app.Preferences().SetString(prefRoot, l.client.Root)
	}
}

// findClient asks the player to download Arena Siege Survivors or show where it is, until one works.
func (l *launcher) findClient() bool {
	const (
		optDownload = "Download the latest Arena Siege Survivors release into this folder"
		optExisting = "Arena Siege Survivors is already in this folder"
	)
	type choice struct {
		download bool
		dir      string
	}

	// Download into the folder named with -root, or a new folder next to the launcher.
	defaultDir := l.client.Root
	if !l.explicitRoot {
		defaultDir = filepath.Join(l.client.Root, "ArenaSiegeSurvivorsClient")
	}
	l.say("Arena Siege Survivors's game files were not found in " + l.client.Root + ".")

	for problem := ""; ; {
		answer := make(chan *choice, 1)
		fyne.Do(func() {
			help := widget.NewLabel("The launcher could not find Arena Siege Survivors's game files (bin\\siege-server.exe and bin\\benilla.exe). " +
				"Download the latest release, or show the launcher where Arena Siege Survivors is.")
			help.Wrapping = fyne.TextWrapWord
			radio := widget.NewRadioGroup([]string{optDownload, optExisting}, nil)
			radio.Required = true
			radio.SetSelected(optDownload)
			folder := widget.NewEntry()
			folder.SetText(defaultDir)
			browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
				fd := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
					if u != nil && err == nil {
						folder.SetText(filepath.Clean(u.Path()))
					}
				}, l.win)
				fd.Resize(fyne.NewSize(600, 400))
				fd.Show()
			})
			items := []*widget.FormItem{
				widget.NewFormItem("", help),
				widget.NewFormItem("", radio),
				widget.NewFormItem("Folder", container.NewBorder(nil, nil, nil, browse, folder)),
			}
			if problem != "" {
				p := widget.NewLabel(problem)
				p.Wrapping = fyne.TextWrapWord
				items = append(items, widget.NewFormItem("", p))
			}
			d := dialog.NewForm("Arena Siege Survivors not found", "Continue", "Quit", items, func(ok bool) {
				if !ok {
					answer <- nil
					return
				}
				answer <- &choice{download: radio.Selected == optDownload, dir: filepath.Clean(folder.Text)}
			}, l.win)
			d.Resize(fyne.NewSize(620, d.MinSize().Height))
			d.Show()
		})

		c := <-answer
		if c == nil {
			fyne.Do(l.app.Quit)
			return false
		}
		if c.dir == "" || c.dir == "." {
			problem = "Choose a folder."
			continue
		}
		if !c.download {
			if err := client.New(c.dir).Check(); err != nil {
				problem = err.Error() + "."
				continue
			}
			l.client.Root = c.dir
			l.say("Using Arena Siege Survivors in " + c.dir)
			return true
		}
		if err := l.installGame(c.dir); err != nil {
			if errors.Is(err, context.Canceled) {
				problem = "The download was stopped."
			} else {
				problem = "The download failed: " + err.Error()
			}
			continue
		}
		return true
	}
}

// installGame downloads the latest release into dir and points the launcher at it.
func (l *launcher) installGame(dir string) error {
	ctx, finish := l.startJob("Arena Siege Survivors is being downloaded. Stop and close the launcher?")
	defer finish()

	l.say("Downloading the latest Arena Siege Survivors into " + dir + "...")
	l.showProgress("Downloading Arena Siege Survivors...", true)
	report := l.progressReporter()
	c := client.New(dir)
	c.Log = l.say
	err := c.Install(ctx, func(done, total int64) { report("Downloading Arena Siege Survivors", done, total) })
	l.hideProgress()
	if err != nil {
		l.say("Error: " + err.Error())
		return err
	}
	l.client.Root = dir
	l.say(fmt.Sprintf("Arena Siege Survivors v%s is installed in %s.", l.client.Release(), dir))
	l.showRelease()
	return nil
}
