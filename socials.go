package main

import (
	_ "embed"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// The round YouTube, X and Discord badges, the same ones the in-game menu shows
// (SiegeMenu/Textures), and where they go (the game's own `.arena youtube|twitter|discord`).
var (
	//go:embed assets/youtube.png
	youtubePNG []byte
	//go:embed assets/x.png
	xPNG []byte
	//go:embed assets/discord.png
	discordPNG []byte
)

var socials = []struct {
	name, url string
	icon      []byte
}{
	{"YouTube", "https://www.youtube.com/@ArenaSiegeSurvivors", youtubePNG},
	{"X", "https://x.com/ArenaSiegeGame", xPNG},
	{"Discord", "https://discord.gg/QFnT5jFmC4", discordPNG},
}

// socialSize is a badge's size, as in the game's menu.
const socialSize = 34

// linkBadge is a round image that opens a page when clicked.
type linkBadge struct {
	widget.BaseWidget
	img  *canvas.Image
	open func()
}

func newLinkBadge(name string, icon []byte, open func()) *linkBadge {
	img := canvas.NewImageFromResource(fyne.NewStaticResource(name+".png", icon))
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(socialSize, socialSize))
	b := &linkBadge{img: img, open: open}
	b.ExtendBaseWidget(b)
	return b
}

func (b *linkBadge) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(b.img) }
func (b *linkBadge) Tapped(*fyne.PointEvent)             { b.open() }
func (b *linkBadge) Cursor() desktop.Cursor              { return desktop.PointerCursor }

// socialBadges is the row of badges.
func (l *launcher) socialBadges() []fyne.CanvasObject {
	var out []fyne.CanvasObject
	for _, s := range socials {
		link := s.url
		out = append(out, newLinkBadge(s.name, s.icon, func() {
			u, err := url.Parse(link)
			if err == nil {
				err = l.app.OpenURL(u)
			}
			if err != nil {
				l.say("Could not open " + link + ": " + err.Error())
			}
		}))
	}
	return out
}
