# Arena Siege Survivors Launcher

## For players

Download and open the launcher, then click **Install**. It downloads the latest Arena Siege Survivors and the required Classic WoW **1.12.1** data from Stonetavern, checks the client checksum, and configures the data folder automatically. Press **Play** when ready and create your hero in game.

- Default install: `%LOCALAPPDATA%/ArenaSiegeSurvivors/Game`. Change it under **Change install location**.
- Allow at least **20 GB free**. The Stonetavern download is about 5 GB, plus the game and temporary extraction space.
- Interrupted client downloads resume on retry. A failed install shows **Retry install**.
- Every launcher start checks for game updates and installs newer versions automatically. Saves, login and the selected client folder are preserved. Without internet, an installed game can still be played.
- **Options & troubleshooting** contains the existing-client folder picker, alternate client editions, manual update check and diagnostic log.
- Download source: https://stonetavern.app/clients . The launcher reads the published Classic URL and SHA-256 rather than bundling or running Stonetavern's launcher. It uses the data with our own game executable.

The game stays free. No microtransactions; cosmetics and player power are earned by playing.

## Build

`go test ./internal/...` tests downloads, updates, extraction, checksums and save preservation.
`go test -tags ci .` tests the interface and the one-click install confirmation with a software renderer.

Windows release builds need Go and MinGW-w64. Build using the existing Makefile. Keep the app ID and legacy executable alias for existing-player self-updates. Preview binaries use `dev-one-click` and deliberately do not self-update. Public signing and release remain a separate step.

Existing installs and Rogue Arena migrations continue to use the existing updater and local save paths.