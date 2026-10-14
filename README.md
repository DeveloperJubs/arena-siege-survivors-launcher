# arena-siege-survivors-launcher
Launcher for Arena Siege Survivors (formerly Rogue Arena), published at
<https://github.com/DeveloperJubs/ArenaSiegeSurvivors-Releases>.

A [Fyne](https://fyne.io) window that does what the client's `Play Arena Siege Survivors.bat` /
`launcher.ps1` do:

1. **Updates**: compares `version.txt` with the one at the URL in `update.txt`. If a newer one is
   out, it downloads `ArenaSiegeSurvivorsClient-update.zip` and unpacks it over the folder,
   keeping `login.txt`, `wow-data.txt`, `game\data` and `Play Arena Siege Survivors.bat`.
2. **First start**: asks for a character name (`siege-server.exe add-friend <Name>` writes
   `login.txt`) and the WoW 1.12.1 `Data` folder (`wow-data.txt`), and puts a desktop shortcut.
3. **Play**: starts `bin\siege-server.exe` hidden (port 3725), waits for it, then runs
   `bin\benilla.exe` with `WOW_DATA`, `WOW_HOST`, `WOW_USER`, `WOW_PASS`, `WOW_CHAR`,
   `BENILLA_HOME` and `BENILLA_SIEGE_PATCH` set. When the game closes, the server is stopped.
   Logs go to `game\server-log.txt` and `game\benilla-log.txt`.

### From Rogue Arena

A `RogueArenaClient` folder from before the rename still works with the launcher. Its
`update.txt` names the old release site, so the launcher updates it from the new one instead.
The update keeps the character, saves and WoW folder. On the first Play after that, the
launcher carries over the old names, like `launcher.ps1` does:
- The addons' saved settings move from `RogueMenu`/`RogueUI`/`RogueMeter` to `Siege*`, and the
  old addon folders are removed.
- The per-realm `Rogue_Arena-*` settings are copied to `Arena_Siege_Survivors-*`.
- `bin\rogue-server.exe` and the old `rogue` patch folder are deleted.
- A "Rogue Arena" desktop shortcut made for that folder is renamed.

### WoW client downloads

The game needs a WoW 1.12.1 `Data` folder. If you don't have one, **Download WoW client...** (or
the button in the first-start WoW folder dialog) offers the editions published at
<https://stonetavern.app/clients> (Cinematic, Enhanced, Classic). The chosen one is downloaded
(about 5 GB, resumable), checked against the SHA-256 on that page, unpacked into
`<folder>/Stonetavern-<Edition>` and used as the game's WoW folder. The installed edition and
build are recorded in `stonetavern-client.json` there, so at every start (and with
**Check for updates**) the launcher looks for a newer build of it and offers to install it.

## Use

Put `ArenaSiegeSurvivorsLauncher.exe` in the `ArenaSiegeSurvivorsClient` folder and run it, or
point it at the folder: `ArenaSiegeSurvivorsLauncher.exe -root C:\Games\ArenaSiegeSurvivorsClient`.

If the launcher finds no game files there, it offers to download the latest release (into an
`ArenaSiegeSurvivorsClient` folder next to the launcher, or a folder you pick) or to use a folder
where the game already is. The folder is remembered for the next start.

The launcher also updates itself: at every start (and with **Check for updates**) it looks at
this repository's latest GitHub release and, if it is newer, offers to download it (checked
against the release's `SHA256SUMS`), swap it in and restart. Development builds (any version
that is not a plain `vX.Y.Z`) never update themselves.

The launcher and this repository used to be called RogueArenaLauncher. Each release also has
the exe as `RogueArenaLauncher.exe`, so launchers from then still find their updates. An
updated launcher keeps its file name.

## Build and release

    make build                  # dist/ArenaSiegeSurvivorsLauncher.exe (and its old-name copy), SHA256SUMS
    make push VERSION=v0.1.0    # build, tag v0.1.0, push the tag, upload the GitHub release
    make test                   # tests of the game logic (no window needed)

Fyne needs cgo, so the Windows build uses MinGW-w64 (`x86_64-w64-mingw32-gcc`) when it is
installed, and otherwise builds inside a Docker image that has it (`build/windows.Dockerfile`).
`make push` needs the [GitHub CLI](https://cli.github.com) logged in (`gh auth login`), a
committed tree, and a `vX.Y.Z` version. Running it again for the same version replaces the
release's files. The release's file names never change, so the latest launcher is always at
`https://github.com/danotchuuy/arena-siege-survivors-launcher/releases/latest/download/ArenaSiegeSurvivorsLauncher.exe`.
