package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/danotchuuy/arena-siege-survivors-launcher/internal/client"
	"os"
	"path/filepath"
)

func (l *launcher) detectExisting() int {
	home, _ := os.UserHomeDir()
	local, _ := os.UserCacheDir()
	roots := []string{l.client.Root, filepath.Dir(l.client.Root), filepath.Join(home, "Downloads"), filepath.Join(home, "Desktop"), filepath.Join(home, "Documents"), filepath.Join(home, "Games"), filepath.Join(home, "OneDrive", "Desktop"), filepath.Join(local, "ArenaSiegeSurvivors"), `C:\Games`, `D:\Games`, `D:\ArenaSiege`}
	found := client.Discover(roots)
	if len(found) == 0 {
		return 0
	}
	selected := found[0]
	if len(found) > 1 {
		selected = l.ask(func(done func(string)) {
			picker := widget.NewSelect(found, nil)
			picker.SetSelected(found[0])
			d := dialog.NewForm("Choose your existing game", "Use this folder", "Cancel", []*widget.FormItem{widget.NewFormItem("Installation", picker)}, func(ok bool) {
				if ok {
					done(picker.Selected)
				} else {
					done("")
				}
			}, l.win)
			d.Resize(fyne.NewSize(650, 260))
			d.Show()
		})
		if selected == "" {
			return -1
		}
	}
	l.client.Root = selected
	l.rememberRoot()
	l.say("Found your existing game: " + selected + ". Your saves stay in this folder.")
	return 1
}
