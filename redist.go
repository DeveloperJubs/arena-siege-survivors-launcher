package main

import (
	"fyne.io/fyne/v2"
	"github.com/danotchuuy/arena-siege-survivors-launcher/internal/client"
	"net/url"
)

// vcRedistURL is Microsoft's download of the latest Visual C++ Redistributable (x64).
const vcRedistURL = "https://aka.ms/vs/17/release/vc_redist.x64.exe"

// checkVCRedist offers to download the Microsoft Visual C++ Redistributable (x64), which the game
// needs, if it is not installed. Not on the UI goroutine.
func (l *launcher) checkVCRedist() {
	if client.VCRedistInstalled() {
		return
	}
	l.say("The Microsoft Visual C++ Redistributable (x64) is not installed; the game needs it to start.")
	question := "Arena Siege Survivors needs the Microsoft Visual C++ Redistributable (x64), which is not installed. Download it from Microsoft now?"
	if !l.confirm("Microsoft Visual C++ Redistributable", question) {
		l.say("(Without it the game may not start. It can be downloaded from " + vcRedistURL + ")")
		return
	}
	u, _ := url.Parse(vcRedistURL)
	fyne.Do(func() {
		if err := l.app.OpenURL(u); err != nil {
			l.say("Could not open the download (" + err.Error() + "); get it from " + vcRedistURL)
		}
	})
	l.say("Run the downloaded vc_redist.x64.exe to install it, then press Play.")
}
