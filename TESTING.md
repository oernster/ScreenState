# Testing

How ScreenState is tested, what the gate checks and what only a real desktop
can settle. Every command is PowerShell, run from the repository root.

## Before the first run: allow Go's build directory

`go test` links one unsigned executable per package into Go's scratch
directory and runs it from there. Malwarebytes scores a freshly written
unsigned executable in a temporary directory as `Malware.AI` and quarantines
it, which stops the suite running at all. Measured on this project on
2026-09-20: one package's test binary was taken five times in six minutes.

Give Go a scratch directory of its own, which nothing else writes to, then
allow that folder. `GOTMPDIR` is empty until it is set, so set it once:

```powershell
go env -w GOTMPDIR="$env:LOCALAPPDATA\Temp\go-tmp"
New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\Temp\go-tmp"
```

`go env GOTMPDIR` then names the folder to allow. In Malwarebytes: Settings, then the allow list, then a file or folder, then
that path with both protections left ticked. Other anti-virus products behave
the same way and want the same folder allowed.

This is a transient build output, never a shipped artefact. Nothing this
project ships is ever put behind an exclusion.

**A quarantine mid-run cannot pass as a green suite.** A test binary taken
while it runs makes `go test` report its package `ok` with no test in it, so
the gate would go green over a package it never exercised. `test.ps1` lists
every package that owns test files, watches each one start at least one test,
then fails naming any that ran none.

## The gate

```powershell
./test.ps1
```

`build.ps1` runs it before it builds anything and has no switch to skip it. In
order:

1. `gofmt -l .` must list nothing.
2. `go vet ./...` must pass.
3. staticcheck must report nothing. It is fetched through `go run` rather than
   installed. `./test.ps1 -Quick` skips this one step while working.
4. `GOOS=linux go build ./...`: the whole module must still build for a
   platform that is not Windows.
5. The suite with the race detector, built with cgo on.
6. The suite as the product ships, with cgo off, measuring coverage.
7. Every function in `internal/domain` and every function in
   `internal/application` must be exercised by at least one test.

Two suite runs, because they cannot be one: the race detector needs cgo (and
with it a C compiler) while the binary that ships is built without it. Running
only one would leave the other configuration untested.

It ends with `All checks passed.` Trust the exit code, not the text:

```powershell
./test.ps1; $LASTEXITCODE
```

`0` means every step passed. A failing step stops the script with the reason.

### The coverage floor

`internal/domain` and `internal/application` are each held at 100% of their
functions: the gate reads `go tool cover -func` and fails naming the package
when any function in it has no statement a test reached. It is a floor on
functions, not on statements; measured statement coverage is 100% for the
domain and a little short of it for the application. They are the layers a machine can
exercise with no filesystem, no clock and no desktop, so a function nothing
calls there is a decision nobody made.

Infrastructure is deliberately not floored. Its Windows half needs a real
desktop, a real registry and real windows to move; a number over those would
either mean nothing or push the tests into asserting whatever happened to be
on screen. What it does have is listed below.

## What the tests prove

### The rules about a profile

`internal/domain` holds what a profile is and what a placement means, with no
I/O and no clock: `profile_test.go` covers naming, entries and the default
marking, `geometry_test.go` the rectangles, the display matching and the
window states. `recognises_test.go` holds the rule that a packaged application
is still recognised once an update has moved its path; both sides must carry
the same model id.

### The decisions

`internal/application` is the largest suite, because every decision lives
there behind an interface and every failure can therefore be caused on demand
with a hand-written fake. It covers the capture and what it leaves out, the
applications running with no window that a capture offers apart, the restore
and its lifecycle, stopping a restore part way, placing a packaged
application's window after an update has moved its path, waiting for a window
that has not appeared, the report, the tray, profile naming and marking, the
update check and the failure wording.

There are no mocking libraries. The fakes are written by hand, in
`fakes_test.go`, `fakes_desktop_test.go` and `fakes_store_test.go`, with fields
that inject failures; `fixtures_test.go` holds the windows and profiles the
tests share.

### The infrastructure that can be exercised

| Package | What its tests use |
|---|---|
| `store` | a real temporary directory: round trips, the default marking, a profile that is not there, a broken file that costs only itself, a profile in a newer format left alone, a store that cannot be read or has gone away, an interrupted write that must leave the old file whole |
| `settings` | the same, for `settings.json`: its absent-means-on reading and a damaged file reported as a fault rather than read as the defaults |
| `runlog` | a real log file, its header and its steps; a run keeping the 10 most recent restores with the header of each one's run; a long log of fewer restores kept whole |
| `clock` | the real clock the application layer is given through its `Clock` port |
| `instance` | a real named mutex |
| `win32` | the naming rules, the stacking-order rule and the rule that a program under the Windows directory is part of Windows and the rule that a click on a splash is not the user taking over, on any platform; behind a build tag, the probes that read the real desktop (its windows, a running application, the applications running with no window shown and a press on the taskbar traced to its top-level window) and the flash series worked out from the machine's settings |
| `setup` | the version comparison, the payload extraction with its fence against an archive entry that climbs out of the install directory, copying and removing trees, the install and state directories and the sign-in entry |
| `internal/ui` | the splash: its palette read from `theme.css`, its logo reduced from the master and how it hears input; the tray's attention badge, drawn in the theme's colours in the corner over the artwork; the icon Windows builds from it once per theme |

`startup`, `update` and `window` have no tests of their own: they are the
registry, the network and the desktop. What can be decided about them was
moved into `internal/application`, which is tested; what is left is the call
itself. Which screen the setup program shows is decided in its page and in
`installer/app.go`, neither of which has a Go test; it is checked by hand.

### The wire between the program and its pages

Neither page has a build step, so nothing compiles or type checks them.
Structural tests stand in for that, holding three rules: a page may read only
fields the program actually sends; it may call only methods the program
actually binds; it may not write the product's name down anywhere. Both pages
are held to all three; every script in the manager's page directory must be
loaded by its `index.html`.

The facade's own test, `wire_test.go`, holds the other rule that the page
depends on: no list may reach a page as `null`. A nil slice is marshalled as
null rather than as an empty array, which a page then reads a length from and
throws, leaving the window on the panel it was waiting with. That was a real
defect, on 2026-09-20, on a capture that found nothing unreadable.

Two more facade tests sit beside it. `window_test.go` holds FR-048: a sign-in
start leaves the window off screen, a start by hand takes the keyboard and
closing the manager puts the window back off screen. `capture_test.go` carries
FR-005 across the wire against the real store in a temporary directory: only
the applications running with no window that the user ticked are saved, each
as running with no placement.

### The structure

`tests/structural` parses the source and fails on a layer violation, an
impure domain, a second composition root, a Go or page file over 400 lines or
in the band just below it, an undocumented exported type, a program ended above
infrastructure, an application started in front, a decision that is not
portable, a timing value with two homes, the product's name written twice, a
page naming the product, a wire that disagrees with itself, a call to a method
nothing binds, a manager script or stylesheet nothing loads and a shared asset that has
drifted from its master in `assets/`.

Each assertion is proved by planting a violation and reading the failure, not
by being believed.

## What the tests never do

- **Move a real window.** Nothing in the suite arranges the desktop of the
  machine running it.
- **Reach the network.** The update check is tested against a fake release
  source; the real one is exercised by hand.
- **Touch the real profiles, settings or log.** Tests work in temporary
  directories.

## Running part of the suite

```powershell
go test ./internal/domain/...
go test -run TestAnUnreadableWindowIsNamedRatherThanDropped -v ./internal/application
go test -cover ./internal/infrastructure/store
```

## Checked by hand

Some requirements need a real desktop, a real sign-in or a real release feed.
They are checked on the reference machine by the list below; what was measured
while each was built is recorded in its rationale in `REQUIREMENTS.md`.

| Check | How |
|---|---|
| A capture reads the desktop as it is (FR-010) | Open a handful of windows, capture, compare the review against the screen. |
| A restore puts them back (FR-041) | Move the windows, apply the profile, look. |
| No program is ever ended (FR-029) | Apply a profile with unsaved work open, the setting for unnamed windows off; nothing is lost. |
| Closing the windows a profile does not name (FR-064) | Turn the setting on, sign out and in again, watch where the unnamed applications go: the ones that live in the notification area should be there rather than on the taskbar. Then press Apply with the setting still on: it should close nothing. |
| A window that refuses to close is minimised (FR-064) | With the setting on, leave an unsaved document open in an application no profile names, sign out and in again, then answer the prompt it puts up. |
| The report names what it could not do (FR-044) | Apply a profile naming an application that is not installed. |
| The bar moves while a restore runs (FR-065, FR-066) | Apply a profile holding several applications that are not running: the bar should fill as each is placed. Closing the report should leave the profile list showing. |
| Sign-in (FR-053) | Tick the option, sign out and in again. |
| A packaged application whose path has moved is started by the activation manager (FR-067) | Only after an update has moved a packaged application's path: sign in and read the log, which says whether the activation manager started it or the shell was asked instead. Before that, the application is started from its path and the log says so. |
| Every window a profile records is opened (FR-069) | Capture with two Notepad windows open, close Notepad, then apply the profile: both windows should come back and be placed, with the unnamed windows put away straight after rather than at the ceiling. |
| The taskbar buttons carry their icons (FR-072) | Sign in and look at every taskbar without clicking one: each button should carry its icon. The log says how many taskbars were sent a click. The pointer should not have moved and the window in front should be the one the restore left there. |
| The desktop comes back unmarked (FR-075) | Sign in and look at the taskbars: no button should carry the mark a taskbar puts on the window last activated on its display; every window should be where the profile put it. The log says how many buttons were built afresh. Each window flickers once as that happens; a maximised window must come back maximised rather than at its normal rectangle. |
| A click on the splash lets the restore carry on (FR-078) | Sign in with an application in the profile that is slow to start, click a splash while it says "Please wait": every splash should close and the slow application should still be placed when it appears. The log should not say "the desktop was taken over with entries outstanding". |
| The flashing has ended before the rebuild (FR-080) | Sign in without touching anything until the splash says ready, then look at the taskbars: no button should be red or flashing, the underline should sit on the window that has the front and clicking a button should bring its window forward rather than minimise it. The log says how long the restore waited for the buttons to stop flashing and how often a flash began the wait again. Only a real sign-in shows it: whether an application asks for the front (and when) belongs to that application. |
| Installing arranges nothing (FR-076) | With a profile recording two Terminal windows and one open, install over an existing copy and let setup start the agent: no window should open, move or close; the log should say setup started that copy so nothing was arranged. |
| Placing a window never activates it (FR-074) | Type into a window on one display, then apply a profile that places windows on the others: the keyboard should stay where it was and no taskbar button should be lit when the restore ends. |
| A packaged application is named and started by its path (FR-071) | Recapture with Claude and Windows Terminal open: the review should show each as a path under `WindowsApps`, not as a model id. Sign out and in: both should start and be placed; the log should not say a model id was used. Then look at the taskbar before clicking it. |
| A packaged application survives its own update (FR-071) | After Claude next updates, apply the profile without recapturing: it should start and be placed, never put away; the log should say it did not start from its path so its model id was used. |
| An application running with no window is offered (FR-005) | With NordVPN in the notification area and its window closed, capture: NordVPN should be listed under "Running with no window shown", unticked, with nothing from under the Windows directory there. Tick it and save; the profile should show NordVPN with no window placed. Quit NordVPN and apply: it should start and no window of its should be moved. |
| The tray icon asks for attention (FR-045) | Apply a profile naming an application that is not installed: once the restore ends, the tray icon should carry a badge in the theme's danger colour in its bottom-right corner, in the light theme and in the dark one. Apply a profile that completes: the badge should go. After a restore started from the manager and after one at sign-in, rest the pointer on the icon: the tooltip should describe that restore. |
| A start by hand arranges nothing (FR-038) | Quit from the tray, move a window the default profile places, then start the agent from its shortcut: the manager should open, nothing should move, no splash should appear and the log should say it was started by hand so nothing was arranged. |
| Bringing the manager up takes the splash down (FR-078) | Sign in, then before touching anything else open the manager from the tray: every splash should close as the window comes up and the log should say the manager was opened so the splash was taken down. Again with a second launch from the shortcut. Before either, the log should say the splash is ready and closes at the next key press or click. |
| The ceiling is the user's (NFR-PERF-003) | In Settings, set the ceiling to 2 minutes. Apply a profile naming an application that is not installed, then touch nothing. The report should say it was still not there when the ceiling of 2m0s passed; the log's opening line for the restore should say ceiling 2m0s. Type 90: the field should come back as 60. |
| The main screen (FR-068, EIR-002) | Press each profile: its applications should fill the middle with every path whole. The buttons should run down the right with Close and Quit at the foot above the donation button, all visible at the smallest window size. The settings should open from the gear, with a rule between it and the theme button. |
| No window title reaches the log (NFR-PRIV-001) | Sign in with a document open in an application the profile does not name, then read the log: the lines about windows put away or asked to close should name the application by its path and never carry the document's title. |
| A restore can be stopped (FR-049) | Apply a profile holding several applications that are not running, then press Stop the restore on the Applying panel: the report should say the restore was cancelled and every window already placed should stay where it is. |
| A sign-in start opens no window (FR-048) | Sign in with the option ticked: the agent should be in the notification area only. The log says so where the page asked for the keyboard and was left alone. |
| Several displays, mixed scaling (FR-027, FR-031, FR-032) | Capture and restore across monitors at different scales. |
| The update check (FR-058, FR-059) | Once per run against the real release feed, then again with the setting off, where nothing should reach the network. |
| Setup, including what an uninstall removes (DATA-005) | Install, update, go back a version, repair and uninstall, each once, then inspect the registry and the folders. |

See also [DEVELOPMENT.md](DEVELOPMENT.md) for building it and
[ARCHITECTURE.md](ARCHITECTURE.md) for the invariants each structural test
guards.
