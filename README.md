# RogueArenaLauncher
Launcher for Rogue Arena

A [Fyne](https://fyne.io) window that does what the client's `Play Rogue Arena.bat` /
`launcher.ps1` do:

1. **Updates**: compares `version.txt` with the one at the URL in `update.txt`. If a newer one is
   out, it downloads `RogueArenaClient-update.zip` and unpacks it over the folder, keeping
   `login.txt`, `wow-data.txt`, `game\data` and `Play Rogue Arena.bat`.
2. **First start**: asks for a character name (`rogue-server.exe add-friend <Name>` writes
   `login.txt`) and the WoW 1.12.1 `Data` folder (`wow-data.txt`), and puts a desktop shortcut.
3. **Play**: starts `bin\rogue-server.exe` hidden (port 3725), waits for it, then runs
   `bin\benilla.exe` with `WOW_DATA`, `WOW_HOST`, `WOW_USER`, `WOW_PASS`, `WOW_CHAR`,
   `BENILLA_HOME` and `BENILLA_ROGUE_PATCH` set. When the game closes, the server is stopped.
   Logs go to `game\server-log.txt` and `game\benilla-log.txt`.

### WoW client downloads

The game needs a WoW 1.12.1 `Data` folder. If you don't have one, **Download WoW client...** (or
the button in the first-start WoW folder dialog) offers the editions published at
<https://stonetavern.app/clients> (Cinematic, Enhanced, Classic). The chosen one is downloaded
(about 5 GB, resumable), checked against the SHA-256 on that page, unpacked into
`<folder>/Stonetavern-<Edition>` and used as the game's WoW folder. The installed edition and
build are recorded in `stonetavern-client.json` there, so at every start (and with
**Check for updates**) the launcher looks for a newer build of it and offers to install it.

## Use

Put `RogueArenaLauncher.exe` in the `RogueArenaClient` folder and run it, or point it at the
folder: `RogueArenaLauncher.exe -root C:\Games\RogueArenaClient`.

If the launcher finds no Rogue Arena files there, it offers to download the latest release
(into a `RogueArenaClient` folder next to the launcher, or a folder you pick) or to use a folder
where Rogue Arena already is. The folder is remembered for the next start.

## Build and release

    make build                  # dist/RogueArenaLauncher.exe and dist/SHA256SUMS
    make push VERSION=v0.1.0    # build, tag v0.1.0, push the tag, upload the GitHub release
    make test                   # tests of the game logic (no window needed)

Fyne needs cgo, so the Windows build uses MinGW-w64 (`x86_64-w64-mingw32-gcc`) when it is
installed, and otherwise builds inside a Docker image that has it (`build/windows.Dockerfile`).
`make push` needs the [GitHub CLI](https://cli.github.com) logged in (`gh auth login`), a
committed tree, and a `vX.Y.Z` version. Running it again for the same version replaces the
release's files. The release's file names never change, so the latest launcher is always at
`https://github.com/danotchuuy/RogueArenaLauncher/releases/latest/download/RogueArenaLauncher.exe`.
