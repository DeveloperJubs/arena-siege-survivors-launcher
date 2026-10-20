//go:build ci

package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/danotchuuy/arena-siege-survivors-launcher/internal/client"
	"image/png"
	"os"
	"testing"
	"time"
)

func TestLauncherLayout(t *testing.T) {
	a := app.NewWithID("launcher-layout-test")
	defer a.Quit()
	a.Settings().SetTheme(siegeTheme{})
	l := newLauncher(a, client.New(t.TempDir()))
	l.win.Show()
	if l.win.Content().MinSize().Width > 740 {
		t.Fatalf("launcher too wide: %v", l.win.Content().MinSize())
	}
	if os.Getenv("LAUNCHER_PREVIEW") != "" {
		f, err := os.Create(os.Getenv("LAUNCHER_PREVIEW"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err = png.Encode(f, l.win.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
}

func findSetupButton(o fyne.CanvasObject, label string) *widget.Button {
	if b, ok := o.(*widget.Button); ok {
		if b.Text == label {
			return b
		}
		return nil
	}
	if c, ok := o.(*fyne.Container); ok {
		for _, child := range c.Objects {
			if b := findSetupButton(child, label); b != nil {
				return b
			}
		}
	}
	if w, ok := o.(fyne.Widget); ok {
		for _, child := range test.WidgetRenderer(w).Objects() {
			if b := findSetupButton(child, label); b != nil {
				return b
			}
		}
	}
	return nil
}
func TestInstallConfirmation(t *testing.T) {
	a := app.NewWithID("launcher-install-test")
	defer a.Quit()
	a.Settings().SetTheme(siegeTheme{})
	root := t.TempDir()
	l := newLauncher(a, client.New(root))
	l.win.Show()
	answer := make(chan bool, 1)
	go func() { answer <- l.installWelcome(root) }()
	var install *widget.Button
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, o := range l.win.Canvas().Overlays().List() {
			if b := findSetupButton(o, "Install"); b != nil {
				install = b
			}
		}
		if install != nil {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}
	if install == nil {
		t.Fatal("single Install action not found")
	}
	if path := os.Getenv("LAUNCHER_SETUP_PREVIEW"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err = png.Encode(f, l.win.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
	test.Tap(install)
	select {
	case ok := <-answer:
		if !ok || !l.setupApproved || l.client.Root != root {
			t.Fatal("one click did not approve full install")
		}
	case <-time.After(time.Second):
		t.Fatal("install action did not finish")
	}
}
