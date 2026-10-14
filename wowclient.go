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

	"github.com/danotchuuy/RogueArenaLauncher/internal/stonetavern"
)

// installedClient returns the folder of the WoW client the game uses and what this launcher
// installed there, or nil if the player's own client is used.
func (l *launcher) installedClient() (string, *stonetavern.Installed) {
	data := l.client.WoWData()
	if data == "" {
		return "", nil
	}
	dir := filepath.Dir(data)
	in, err := stonetavern.ReadInstalled(dir)
	if err != nil {
		return dir, nil
	}
	return dir, in
}

// downloadClient lets the player pick a client edition and a folder, downloads it and makes it
// the game's WoW files. It reports whether a client was installed. Not on the UI goroutine.
func (l *launcher) downloadClient() bool {
	l.say("Looking up the WoW clients on stonetavern.app...")
	eds, err := stonetavern.FetchEditions(context.Background())
	if err != nil {
		l.say("Error: " + err.Error())
		l.showError(fmt.Errorf("could not get the list of clients: %w", err))
		return false
	}

	ed, parent, ok := l.pickEdition(eds)
	if !ok {
		return false
	}
	dir := filepath.Join(parent, stonetavern.FolderName(ed))
	if err := l.installClient(ed, dir); err != nil {
		return false
	}
	if err := l.client.SetWoWData(stonetavern.DataDir(dir)); err != nil {
		l.say("Error: " + err.Error())
		l.showError(fmt.Errorf("the client was installed in %s, but it has no usable Data folder: %w", dir, err))
		return false
	}
	l.say("Game files: " + l.client.WoWData())
	l.checkClientUpdate(true)
	return true
}

// pickEdition shows the editions on offer and where to put the chosen one.
func (l *launcher) pickEdition(eds []stonetavern.Edition) (ed stonetavern.Edition, parent string, ok bool) {
	type choice struct {
		index  int
		parent string
	}
	answer := make(chan *choice, 1)
	installedDir, installed := l.installedClient()

	fyne.Do(func() {
		labels := make([]string, len(eds))
		selected := 0
		for i, e := range eds {
			labels[i] = fmt.Sprintf("%s (build %s, %s)", e.Name, e.Build, e.Size)
			if e.Recommended {
				labels[i] += " - recommended"
				if installed == nil {
					selected = i
				}
			}
			if installed != nil && e.Name == installed.Edition {
				labels[i] += " - installed"
				selected = i
			}
		}
		about := widget.NewLabel("")
		about.Wrapping = fyne.TextWrapWord
		radio := widget.NewRadioGroup(labels, nil)
		radio.OnChanged = func(s string) {
			for i, label := range labels {
				if label == s {
					e := eds[i]
					about.SetText(fmt.Sprintf("%s\nReleased %s.", e.Tagline, e.Released))
				}
			}
		}
		radio.Required = true
		radio.SetSelected(labels[selected])

		folder := widget.NewEntry()
		folder.SetText(l.client.Root)
		if installed != nil {
			folder.SetText(filepath.Dir(installedDir))
		}
		browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
			fd := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
				if u != nil && err == nil {
					folder.SetText(filepath.Clean(u.Path()))
				}
			}, l.win)
			fd.Resize(fyne.NewSize(600, 400))
			fd.Show()
		})
		note := widget.NewLabel("The download is about 5 GB, and unpacking it needs about as much again. " +
			"The client goes into its own folder (Stonetavern-<edition>) inside the folder you pick. " +
			"An unfinished download carries on where it stopped.")
		note.Wrapping = fyne.TextWrapWord

		items := []*widget.FormItem{
			widget.NewFormItem("Edition", radio),
			widget.NewFormItem("", about),
			widget.NewFormItem("Install in", container.NewBorder(nil, nil, nil, browse, folder)),
			widget.NewFormItem("", note),
		}
		d := dialog.NewForm("Download a WoW 1.12.1 client", "Download", "Cancel", items, func(ok bool) {
			if !ok || folder.Text == "" {
				answer <- nil
				return
			}
			for i, label := range labels {
				if label == radio.Selected {
					answer <- &choice{index: i, parent: filepath.Clean(folder.Text)}
					return
				}
			}
			answer <- nil
		}, l.win)
		d.Resize(fyne.NewSize(620, d.MinSize().Height))
		d.Show()
	})

	c := <-answer
	if c == nil {
		return stonetavern.Edition{}, "", false
	}
	return eds[c.index], c.parent, true
}

// installClient downloads ed into dir, showing progress, with a Cancel button.
func (l *launcher) installClient(ed stonetavern.Edition, dir string) error {
	ctx, finish := l.startJob("A download is running. It carries on where it stopped the next time. Close the launcher?")
	defer finish()

	l.say(fmt.Sprintf("Downloading %s build %s (%s) into %s...", ed.Title, ed.Build, ed.Size, dir))
	l.showProgress("Starting the download...", true)
	report := l.progressReporter()
	err := stonetavern.Install(ctx, ed, dir, func(stage stonetavern.Stage, done, total int64) {
		report(fmt.Sprintf("%s %s", stage, ed.Name), done, total)
	})
	l.hideProgress()

	switch {
	case errors.Is(err, context.Canceled):
		l.say("Download stopped. It carries on where it stopped the next time.")
	case err != nil:
		l.say("Error: " + err.Error())
		l.showError(err)
	default:
		l.say(fmt.Sprintf("%s build %s is installed.", ed.Title, ed.Build))
	}
	return err
}

// checkClientUpdate offers a newer build of the downloaded WoW client, if there is one. With
// announce, it also says when there is nothing to update. Not on the UI goroutine.
func (l *launcher) checkClientUpdate(announce bool) {
	dir, in := l.installedClient()
	if in == nil {
		if announce {
			l.say("The WoW client was not downloaded by this launcher, so it has no updates to check.")
		}
		return
	}
	l.say("Checking for WoW client updates...")
	eds, err := stonetavern.FetchEditions(context.Background())
	if err != nil {
		l.say("(Could not check for WoW client updates: " + err.Error() + ")")
		return
	}
	ed := stonetavern.Find(eds, in.Edition)
	if ed == nil {
		l.say(fmt.Sprintf("stonetavern.app no longer offers the %s client; keeping the one you have.", in.Edition))
		return
	}
	if !ed.IsNewerThan(in) {
		l.say(fmt.Sprintf("The WoW client (%s build %s) is up to date.", ed.Title, in.Build))
		return
	}

	question := fmt.Sprintf("A new build of %s is out (%s, %s). Download it now?", ed.Title, ed.Build, ed.Size)
	if in.Build == "" {
		question = fmt.Sprintf("The download of %s did not finish. Carry on with it now?", ed.Title)
	}
	if !l.confirm("WoW client update", question) {
		l.say("WoW client update skipped.")
		return
	}
	_ = l.installClient(*ed, dir)
}

func (l *launcher) showError(err error) {
	fyne.Do(func() { dialog.ShowError(err, l.win) })
}
