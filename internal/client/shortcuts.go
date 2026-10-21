package client

// The desktop shortcut's makers. Tests replace them, so a test run never touches the real
// desktop (it once pointed the player's "Arena Siege Survivors" shortcut at the test binary).
var (
	createShortcut = createDesktopShortcut
	renameShortcut = renameDesktopShortcut
)
