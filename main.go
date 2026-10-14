// Command RogueArenaLauncher is a window for playing Rogue Arena: it does what the client's
// "Play Rogue Arena.bat" and launcher.ps1 do (updates, first-start setup, starting the game), and
// can download the WoW 1.12.1 client the game needs from stonetavern.app.
// Put it in the RogueArenaClient folder, or point it there with -root.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/danotchuuy/RogueArenaLauncher/internal/client"
)

// version is set at build time (see the Makefile).
var version = "dev"

func main() {
	root := flag.String("root", "", "the RogueArenaClient folder (default: the folder this launcher is in)")
	flag.Parse()
	if *root == "" {
		if exe, err := os.Executable(); err == nil {
			*root = filepath.Dir(exe)
		}
	}

	a := app.NewWithID("io.github.danotchuuy.roguearenalauncher")
	l := newLauncher(a, client.New(*root))
	l.win.Show()
	go l.start()
	a.Run()
}

type launcher struct {
	app    fyne.App
	win    fyne.Window
	client *client.Client

	log      *widget.Label
	logLines []string
	logBox   *container.Scroll

	status    *widget.Label
	progress  *widget.ProgressBar
	cancelBtn *widget.Button
	busyBox   *fyne.Container // status, progress and cancel; shown during long jobs

	play        *widget.Button
	wowBtn      *widget.Button
	downloadBtn *widget.Button
	updatesBtn  *widget.Button

	mu  sync.Mutex
	job *job // the game or a download, while one runs
}

// job is something long-running that closing the launcher has to stop.
type job struct {
	cancel   context.CancelFunc
	done     chan struct{}
	question string // asked before closing the launcher stops it
}

func newLauncher(a fyne.App, c *client.Client) *launcher {
	l := &launcher{app: a, client: c, win: a.NewWindow("Rogue Arena")}
	c.Log = l.say

	l.log = widget.NewLabel("")
	l.log.Wrapping = fyne.TextWrapWord
	l.logBox = container.NewVScroll(l.log)

	l.status = widget.NewLabel("")
	l.progress = widget.NewProgressBar()
	l.cancelBtn = widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), l.cancelJob)
	l.busyBox = container.NewVBox(l.status, container.NewBorder(nil, nil, nil, l.cancelBtn, l.progress))
	l.busyBox.Hide()

	l.play = widget.NewButtonWithIcon("Play", theme.MediaPlayIcon(), l.onPlay)
	l.play.Importance = widget.HighImportance
	l.wowBtn = widget.NewButtonWithIcon("WoW folder...", theme.FolderOpenIcon(), func() {
		l.task(func() { l.askWoWData("") })
	})
	l.downloadBtn = widget.NewButtonWithIcon("Download WoW client...", theme.DownloadIcon(), func() {
		l.task(func() { l.downloadClient() })
	})
	l.updatesBtn = widget.NewButtonWithIcon("Check for updates", theme.ViewRefreshIcon(), func() {
		l.task(l.checkForUpdates)
	})
	l.setButtons(false)

	title := widget.NewLabelWithStyle("Rogue Arena", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	buttons := container.NewHBox(l.wowBtn, l.downloadBtn, l.updatesBtn, layout.NewSpacer(), l.play)
	l.win.SetContent(container.NewBorder(title, container.NewVBox(l.busyBox, buttons), nil, nil, l.logBox))
	l.win.Resize(fyne.NewSize(760, 460))
	l.win.SetCloseIntercept(l.onClose)
	return l
}

// say adds a line to the message area. Safe from any goroutine.
func (l *launcher) say(text string) {
	fyne.Do(func() {
		l.logLines = append(l.logLines, text)
		l.log.SetText(strings.Join(l.logLines, "\n"))
		l.logBox.ScrollToBottom()
	})
}

func (l *launcher) showRelease() {
	title := "Rogue Arena"
	if rel := l.client.Release(); rel != "" {
		title += " v" + rel
	}
	fyne.Do(func() { l.win.SetTitle(title) })
}

// setButtons turns all the buttons on or off. Safe from any goroutine.
func (l *launcher) setButtons(on bool) {
	fyne.Do(func() {
		for _, b := range []*widget.Button{l.play, l.wowBtn, l.downloadBtn, l.updatesBtn} {
			if on {
				b.Enable()
			} else {
				b.Disable()
			}
		}
	})
}

// task runs fn in the background with the buttons off. Called from the UI goroutine.
func (l *launcher) task(fn func()) {
	l.setButtons(false)
	go func() {
		defer l.setButtons(true)
		fn()
	}()
}

// start runs the steps before the game can be played: updates, then the first-start setup.
func (l *launcher) start() {
	if err := l.client.Check(); err != nil {
		l.fatal(fmt.Errorf("%v.\nPut this launcher in the RogueArenaClient folder, or start it with -root <folder>", err))
		return
	}
	l.say("Rogue Arena Launcher " + version)
	l.showRelease()
	l.update()

	if !l.client.HasLogin() {
		l.say("Welcome to Rogue Arena!")
		l.askCharacter()
	}
	if user, _, err := l.client.Login(); err == nil {
		l.say("Character: " + user)
	}
	if l.client.WoWData() == "" {
		l.askWoWData("")
	} else {
		l.checkClientUpdate(false)
	}
	l.say("Game files: " + l.client.WoWData())
	l.say("Ready. Press Play.")
	l.setButtons(true)
}

// checkForUpdates looks for a new Rogue Arena and a new build of the downloaded WoW client.
func (l *launcher) checkForUpdates() {
	l.update()
	l.checkClientUpdate(true)
}

// update installs a newer version of Rogue Arena if there is one. Without internet the player
// plays the version they have.
func (l *launcher) update() {
	l.say("Checking for Rogue Arena updates...")
	newer, err := l.client.CheckUpdate(context.Background())
	if err != nil {
		l.say("(Could not check for updates; playing the version you have.)")
		return
	}
	if !newer {
		l.say("Rogue Arena is up to date.")
		return
	}
	l.say("A new version of Rogue Arena is out: updating (your saves are kept)...")
	l.showProgress("Updating Rogue Arena...", false)
	p := l.progressReporter()
	err = l.client.Update(context.Background(), func(done, total int64) {
		p("Downloading the Rogue Arena update", done, total)
	})
	l.hideProgress()
	if err != nil {
		l.say(fmt.Sprintf("The update could not be installed (%v); playing the version you have.", err))
		return
	}
	l.showRelease()
	l.say("Updated.")
}

// askCharacter asks for a character name until the game server accepts one.
func (l *launcher) askCharacter() {
	for problem := ""; ; {
		name := l.ask(func(done func(string)) {
			entry := widget.NewEntry()
			entry.SetPlaceHolder("2-12 letters")
			entry.Validator = func(s string) error { _, err := client.CharacterName(s); return err }
			items := []*widget.FormItem{widget.NewFormItem("Name", entry)}
			if problem != "" {
				items = append(items, widget.NewFormItem("", widget.NewLabel(problem)))
			}
			d := dialog.NewForm("Name your character", "Create", "Quit", items, func(ok bool) {
				if !ok {
					l.app.Quit()
					return
				}
				done(entry.Text)
			}, l.win)
			d.Resize(fyne.NewSize(400, d.MinSize().Height))
			d.Show()
			l.win.Canvas().Focus(entry)
		})
		name, err := client.CharacterName(name)
		if err == nil {
			err = l.client.CreateCharacter(name)
		}
		if err == nil {
			return
		}
		problem = err.Error()
	}
}

// chooseDownload is what askWoWData's dialog answers when the player picks "Download one".
const chooseDownload = "\x00download"

// askWoWData asks for the WoW 1.12.1 "Data" folder until a valid one is given, or one is
// downloaded. From the "WoW folder..." button it can be cancelled; at the first start it must be
// answered.
func (l *launcher) askWoWData(problem string) {
	required := l.client.WoWData() == ""
	dismiss := "Cancel"
	if required {
		dismiss = "Quit"
	}
	for {
		dir := l.ask(func(done func(string)) {
			var d *dialog.FormDialog
			wantsDownload := false
			entry := widget.NewEntry()
			entry.SetPlaceHolder(`C:\Games\WoW 1.12\Data`)
			entry.SetText(l.client.WoWData())
			browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
				fd := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
					if u != nil && err == nil {
						entry.SetText(u.Path())
					}
				}, l.win)
				fd.Resize(fyne.NewSize(600, 400))
				fd.Show()
			})
			download := widget.NewButtonWithIcon("Download one from stonetavern.app...", theme.DownloadIcon(), func() {
				wantsDownload = true
				d.Hide() // answers the dialog with false; the callback sees wantsDownload
			})
			help := widget.NewLabel(`Rogue Arena needs the "Data" folder of a World of Warcraft 1.12.1 client (the one with dbc.MPQ, model.MPQ, patch.MPQ ...). Pick yours, or download a client.`)
			help.Wrapping = fyne.TextWrapWord
			items := []*widget.FormItem{
				widget.NewFormItem("", help),
				widget.NewFormItem("Data folder", container.NewBorder(nil, nil, nil, browse, entry)),
				widget.NewFormItem("No client?", container.NewHBox(download)),
			}
			if problem != "" {
				items = append(items, widget.NewFormItem("", widget.NewLabel(problem)))
			}
			d = dialog.NewForm("Your WoW files", "OK", dismiss, items, func(ok bool) {
				switch {
				case wantsDownload:
					done(chooseDownload)
				case ok:
					done(entry.Text)
				case required:
					l.app.Quit()
				default:
					done("")
				}
			}, l.win)
			d.Resize(fyne.NewSize(600, d.MinSize().Height))
			d.Show()
		})
		switch {
		case dir == chooseDownload:
			if l.downloadClient() {
				return
			}
			problem = ""
			continue
		case dir == "" && !required:
			return
		}
		err := l.client.SetWoWData(dir)
		if err == nil {
			if !required {
				l.say("Game files: " + l.client.WoWData())
			}
			return
		}
		problem = err.Error() + ". Try again."
	}
}

// ask shows a dialog built by show and waits for the answer it passes to done. It must not be
// called on the UI goroutine.
func (l *launcher) ask(show func(done func(string))) string {
	answer := make(chan string, 1)
	fyne.Do(func() { show(func(s string) { answer <- s }) })
	return <-answer
}

// confirm asks a yes/no question and waits for the answer. Not on the UI goroutine.
func (l *launcher) confirm(title, message string) bool {
	return l.ask(func(done func(string)) {
		dialog.ShowConfirm(title, message, func(ok bool) {
			if ok {
				done("yes")
			} else {
				done("")
			}
		}, l.win)
	}) == "yes"
}

// startJob registers a job that closing the launcher must stop first.
func (l *launcher) startJob(question string) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{cancel: cancel, done: make(chan struct{}), question: question}
	l.mu.Lock()
	l.job = j
	l.mu.Unlock()
	return ctx, func() {
		cancel()
		l.mu.Lock()
		l.job = nil
		l.mu.Unlock()
		close(j.done)
	}
}

func (l *launcher) cancelJob() {
	l.mu.Lock()
	j := l.job
	l.mu.Unlock()
	if j != nil {
		j.cancel()
	}
}

func (l *launcher) onPlay() {
	l.setButtons(false)
	ctx, finish := l.startJob("Closing the launcher closes the game too. Close it?")
	go func() {
		err := l.client.Play(ctx)
		finish()
		if err != nil {
			l.say("Error: " + err.Error())
			fyne.Do(func() { dialog.ShowError(err, l.win) })
		} else {
			l.say("The game closed.")
		}
		l.setButtons(true)
	}()
}

// onClose stops the game or download along with the launcher, after asking.
func (l *launcher) onClose() {
	l.mu.Lock()
	j := l.job
	l.mu.Unlock()
	if j == nil {
		l.app.Quit()
		return
	}
	dialog.ShowConfirm("Rogue Arena is busy", j.question, func(ok bool) {
		if !ok {
			return
		}
		j.cancel()
		go func() {
			select {
			case <-j.done:
			case <-time.After(5 * time.Second):
			}
			fyne.Do(l.app.Quit)
		}()
	}, l.win)
}

// showProgress shows the status line and progress bar, with a Cancel button if cancellable.
func (l *launcher) showProgress(status string, cancellable bool) {
	fyne.Do(func() {
		l.status.SetText(status)
		l.progress.SetValue(0)
		if cancellable {
			l.cancelBtn.Show()
		} else {
			l.cancelBtn.Hide()
		}
		l.busyBox.Show()
	})
}

func (l *launcher) hideProgress() {
	fyne.Do(l.busyBox.Hide)
}

// progressReporter returns a func that shows "<what>: 1.20 GB of 4.96 GB" and fills the bar,
// at most a few times a second.
func (l *launcher) progressReporter() func(what string, done, total int64) {
	var last time.Time
	lastWhat := ""
	return func(what string, done, total int64) {
		now := time.Now()
		if what == lastWhat && done != total && now.Sub(last) < 150*time.Millisecond {
			return
		}
		last, lastWhat = now, what
		text := fmt.Sprintf("%s: %s", what, formatBytes(done))
		value := 0.0
		if total > 0 {
			text += " of " + formatBytes(total)
			value = float64(done) / float64(total)
		}
		fyne.Do(func() {
			l.status.SetText(text)
			l.progress.SetValue(value)
		})
	}
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%d KB", n>>10)
	}
}

// fatal shows an error the launcher cannot get past, then quits.
func (l *launcher) fatal(err error) {
	l.say("Error: " + err.Error())
	fyne.Do(func() {
		d := dialog.NewError(err, l.win)
		d.SetOnClosed(l.app.Quit)
		d.Show()
	})
}
