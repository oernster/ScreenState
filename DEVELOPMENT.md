# Development

How to build ScreenState from source, what each script does and how a release
is cut. Every command is PowerShell, run from the repository root.

## Tools

| Tool | Needed for | Get it |
|---|---|---|
| Go, at the version `go.mod` names or later | everything | [go.dev/dl](https://go.dev/dl/) |
| Wails CLI v2.12.0 | both programs, each of which has a window | `go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0` |
| WebView2 runtime | running either program | ships with Windows 11 |
| A C compiler for cgo, such as [WinLibs](https://winlibs.com/) gcc on `PATH` | the race-detector pass of the gate, which every build runs | [winlibs.com](https://winlibs.com/) |
| Python 3 | stamping the version into the site, which every build does | [python.org](https://www.python.org/downloads/) |
| Pillow | regenerating the icons only | `python -m pip install pillow` |

staticcheck is not installed: `test.ps1` fetches the latest release through
`go run`, which needs the network on first use and can move between runs.
`./test.ps1 -Quick` skips that one step while working; the build always runs it.

cgo is off for everything that ships. The only exception is the race detector,
which needs it and is therefore its own pass inside the gate; that pass is why
a build needs a C compiler although nothing shipped is compiled with one.

Allow Go's scratch directory in your anti-virus before the first test run: an
anti-virus product that quarantines Go's unsigned test binaries, as
Malwarebytes did on this project, stops the suite running. See
[TESTING.md](TESTING.md).

## Building

```powershell
./build.ps1
```

In order, it:

1. Reads `VERSION`.
2. Runs `stamp_version.py`, which writes that version into the site under
   `docs/`, since a page cannot read `VERSION` for itself.
3. Runs [`test.ps1`](TESTING.md). A failure stops the build; there is no
   switch to skip it.
4. Refuses to go on without `assets/application-icon.png` and its `.ico`,
   which are committed rather than derived at build time.
5. Copies that one icon into both programs' build directories, so the agent,
   the setup program, both shortcuts, the taskbar button and the notification
   area icon all wear it.
6. Copies `assets/theme.css` and `assets/shell.js` over the copies each page
   directory holds, which is why those two are edited in `assets/` and nowhere
   else.
7. Builds `build/bin/ScreenState.exe` with `wails build`, with the version
   passed in through `-ldflags`.
8. Copies the licence beside the agent and zips everything in `build/bin`
   into `installer/payload.zip`.
9. Builds the setup program from `installer/`, the same way.
10. Copies the result to `dist-installer/ScreenStateSetup.exe`, the one file
    that ships.
11. Puts the 22-byte empty zip back in `installer/payload.zip`, so
    `go build ./...` and the tests keep working without a full build and no
    payload can reach a commit.

To build only the agent:

```powershell
./build.ps1 -SkipInstaller
```

Everything generated is ignored by git: `build/`, `installer/build/bin/`,
`dist-installer/`, `frontend/wailsjs/` and `installer/frontend/wailsjs/`. The
rest of `installer/build/` (the setup program's icon, manifest and
`info.json`) is committed, since Wails reads it from there.

### A note on `-ldflags`

`-X` only reaches a `var`. Against a `const` it silently does nothing, which
is a whole release shipping while announcing the placeholder written in the
source, so `main.version` and the setup program's `main.appVersion` are both
declared `var` on purpose.

## Running from source

Build the agent, then start it:

```powershell
./build.ps1 -SkipInstaller
./build/bin/ScreenState.exe -quiet
```

Plain `go run .` does not work: without the Wails build tags that `wails build`
supplies, Wails shows its "will not build without the correct build tags"
message instead of the manager.

How the agent is started decides what it does. With `-quiet`, which is what
the setup program passes, it opens the manager and arranges nothing. With
`-hidden`, which is what the sign-in entry passes, it applies the default
profile and waits in the notification area without opening a window. With
neither it applies the default profile and opens the manager, so start it
with `-quiet` unless the desktop is meant to be rearranged.

Only one copy runs per user session. A second start opens the running copy's
manager rather than starting again, which the log says in as many words.

Everything the agent does is written to `%LOCALAPPDATA%\ScreenState\Log.txt`:
the version, what was restored at sign-in, what a capture read and what a
restore could not do. Read it first when something looks wrong.

The profiles sit beside it, one file each, under
`%LOCALAPPDATA%\ScreenState\profiles`, with `settings.json` alongside.

## The pages

Both windows are plain HTML, CSS and JavaScript with no build step and no
framework. `frontend/dist` is the manager, `installer/frontend/dist` is the
setup program.

Three rules follow from there being no compiler behind them; a structural test
holds each:

- **`theme.css` and `shell.js` are edited in `assets/` and nowhere else.** The
  copies in the two page directories are refreshed by `build.ps1`; the test
  fails when one has drifted.
- **No page file writes the product's name or its tagline.** Both arrive on
  the state the program hands over, so a rename cannot leave a window
  announcing a product that no longer exists.
- **Every script in `frontend/dist` is loaded by `index.html`; every tag names
  a file that is there.** The manager is spread over a script per subject,
  so a new one that nothing loads is dead weight nothing else would report.

The wire between a program and its page is stated twice, as Go structs with
json tags and as the names the page reads, with a test comparing the two. Add
a field on one side only and the test says so.

## The icons

Every piece of artwork has its master in `assets/`: `application-icon.png` for
the mark, plus one each for the light and dark toggle faces, the help, profile
and donate art. `tools/genicons.py` makes the `.ico` beside the mark and writes
each piece, reduced from its master, into the page directories that want it.
The site under `docs/` carries copies of the mark, the toggle faces and the
donate art as the manager has them.

```powershell
python tools/genicons.py
```

It is not part of the build. Its outputs are committed, so a clone builds
without Pillow. Run it when the artwork changes, then commit the results.

## Versioning

`VERSION` holds the only version string anyone writes by hand. Change it there
and nowhere else: the build carries it into both programs through the linker
and into the site through `stamp_version.py`, which rewrites whatever sits
between `<!--VERSION-->` and `<!--/VERSION-->` in `docs/` and touches nothing
else. Run it on its own after changing `VERSION`; a second run changes nothing
and says so.

```powershell
python stamp_version.py
```

## The site

`docs/` is the GitHub Pages site, served from the `main` branch: plain HTML and
CSS with no build step, wearing a copy in `docs/styles.css` of the palette in
`assets/theme.css`; a change to one is made to the other by hand. It shows no
dates anywhere; the version is its only changing text and is stamped as above.

## Cutting a release

1. Set `VERSION`.
2. Run `./build.ps1`, which will not build from a failing tree and stamps the
   site on the way.
3. Install the result and use it: see the by-hand checks in
   [TESTING.md](TESTING.md).
4. Commit, tag `v<version>` and push.
5. Create a GitHub release for the tag and attach
   `dist-installer/ScreenStateSetup.exe`.

The update check reads the same releases feed, so step 5 is what tells every
installed copy that there is a newer version.

## Standing rules

- Every function in `internal/domain` and in `internal/application` is
  exercised by a test; the gate fails otherwise.
- The domain is pure: no I/O, no clock, no window.
- One composition root; it is the only file importing both the application
  layer and the infrastructure.
- No Go or page file (HTML, script, stylesheet) over 400 lines, none left between 381 and 400.
- Every exported type has a doc comment.
- Nothing above infrastructure can end a program.
- No release version written anywhere but `VERSION`.
- No em dashes anywhere, in code, comments or documents.

The reasons are in [ARCHITECTURE.md](ARCHITECTURE.md); the checks that enforce
the first six are in [TESTING.md](TESTING.md). The last two are held by
review: no test checks them.
