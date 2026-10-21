package main

import (
	_ "embed"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"image/color"
)

//go:embed assets/logo.png
var launcherLogo []byte

// The launcher's icon: the game's, with an "L" badge. The window and taskbar use this PNG; the
// .exe uses the same art from assets/launcher.ico, compiled into rsrc_windows_amd64.syso (rc.exe,
// then cvtres.exe, with its two section-less symbols moved to section 1 for the Go linker).
//
//go:embed assets/launcher-icon.png
var launcherIcon []byte

type siegeTheme struct{}

func (siegeTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 16, G: 13, B: 10, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 242, G: 228, B: 195, A: 255}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 210, G: 166, B: 65, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 64, G: 22, B: 15, A: 255}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 25, G: 20, B: 15, A: 255}
	}
	return theme.DefaultTheme().Color(n, theme.VariantDark)
}
func (siegeTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (siegeTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }
func (siegeTheme) Size(n fyne.ThemeSizeName) float32       { return theme.DefaultTheme().Size(n) }
