# Development

How to build ScreenState from source, what each script does and how a release
is cut. Every command is PowerShell, run from the repository root.

## Tools

| Tool | Needed for | Get it |
|---|---|---|
| Go, at the version `go.mod` names or later | everything | [go.dev/dl](https://go.dev/dl/) |
| Wails CLI v2.12.0 | both programs, each of which has a window | `go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0` |
| WebView2 runtime | running either program | ships with Windows 11 |
| Python 3 with Pillow | regenerating the icons only | [python.org](https://www.python.org/downloads/), then `python -m pip install pillow` |

staticcheck is not installed: `test.ps1` fetches it through `go run`, so
nothing about building unchanged code depends on what is on the machine.

cgo is off for everything that ships. The only exception is the race detector,
which needs it and is therefore its own pass inside the gate. No C compiler is
needed for a build.

Allow Go's scratch directory in your anti-virus before the first test run;
without that the suite cannot run at all. See [TESTING.md](TESTING.md).

## Building

```powershell
./build.ps1
```

In order, it:

1. Reads `VERSION`.
2. Runs [`test.ps1`](TESTING.md). A failure stops the build; there is no
   switch to skip it.
3. Refuses to go on without `assets/application-icon.png` and its `.ico`,
   which are committed rather than derived at build time.
4. Copies that one icon into both programs' build directories, so the agent,
   the setup program, both shortcuts, the taskbar button and the notification
   area icon all wear it.
5. Copies `assets/theme.css` and `assets/shell.js` over the copies each page
   directory holds, which is why those two are edited in `assets/` and nowhere
   else.
6. Builds `build/bin/ScreenState.exe` with `wails build`, with the version
   passed in through `-ldflags`.
7. Zips the agent and its licence into `installer/payload.zip`.
8. Builds the setup program from `installer/`, the same way.
9. Copies the result to `dist-installer/ScreenStateSetup.exe`, the one file
   that ships.
10. Puts the 22-byte empty zip back in `installer/payload.zip`, so
    `go build ./...` and the tests keep working without a full build and no
    payload can reach a commit.

To build only the agent:

```powershell
./build.ps1 -SkipInstaller
```

Everything generated is ignored by git: `build/`, `installer/build/`,
`dist-installer/`, `frontend/wailsjs/` and `installer/frontend/wailsjs/`.

### A note on `-ldflags`

`-X` only reaches a `var`. Against a `const` it silently does nothing, which
is a whole release shipping while announcing `0.0.0-dev`, so `main.version`
and the setup program's `main.appVersion` are both declared `var` on purpose.

## Running from source

```powershell
go run .
```

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

`assets/application-icon.png` is the master. `tools/genicons.py` makes the
`.ico` beside it and writes the page artwork (the mark, the light and dark
toggle faces, the help, profile and donate art) into the page directories that
want them.

```powershell
python tools/genicons.py
```

It is not part of the build. Its outputs are committed, so a clone builds
without Python or Pillow. Run it when the artwork changes, then commit the
results.

## Versioning

`VERSION` holds the only version string in the repository. Change it there and
nowhere else; the build carries it into both programs through the linker.

## Cutting a release

1. Set `VERSION`.
2. Run `./build.ps1`, which will not build from a failing tree.
3. Install the result and use it: see the by-hand checks in
   [TESTING.md](TESTING.md).
4. Commit, tag `v<version>` and push.
5. Create a GitHub release for the tag and attach
   `dist-installer/ScreenStateSetup.exe`.

The update check reads the same releases feed, so step 5 is what tells every
installed copy that there is a newer version.

## Standing rules

- `internal/domain` and `internal/application` stay at 100% coverage together;
  the gate fails otherwise.
- The domain is pure: no I/O, no clock, no window.
- One composition root; it is the only file importing both the UI and the
  infrastructure.
- No Go file over 400 lines, none left between 381 and 399.
- Every exported type has a doc comment.
- Nothing above infrastructure ends the program.
- No version string outside `VERSION`.
- No em dashes anywhere, in code, comments or documents.

The reasons are in [ARCHITECTURE.md](ARCHITECTURE.md); the checks that enforce
them are in [TESTING.md](TESTING.md).
