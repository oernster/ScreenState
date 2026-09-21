# Testing

How ScreenState is tested, what the gate checks and what only a real desktop
can settle. Every command is PowerShell, run from the repository root.

## Before the first run: allow Go's build directory

`go test` links one unsigned executable per package into Go's scratch
directory and runs it from there. Malwarebytes scores a freshly written
unsigned executable in a temporary directory as `Malware.AI` and quarantines
it, which stops the suite running at all. Measured on this project on
2026-09-20: one package's test binary was taken five times in six minutes.

Allow the folder `go env GOTMPDIR` names, which is Go's own scratch directory
and is written by nothing else:

```
C:\Users\<you>\AppData\Local\Temp\go-tmp
```

In Malwarebytes: Settings, then the allow list, then a file or folder, then
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
4. `GOOS=linux go build ./...`: the two portable layers must still build for a
   platform that is not Windows.
5. The suite with the race detector, built with cgo on.
6. The suite as the product ships, with cgo off, measuring coverage.
7. Coverage of `internal/domain` and `internal/application` must be 100%.

Two suite runs, because they cannot be one: the race detector needs cgo, while
the binary that ships is built without it. Running only one would leave the
other configuration untested.

It ends with `All checks passed.` Trust the exit code, not the text:

```powershell
./test.ps1; $LASTEXITCODE
```

`0` means every step passed. A failing step stops the script with the reason.

### The coverage floor

`internal/domain` and `internal/application` are held at 100% together. They
are the layers a machine can exercise with no filesystem, no clock and no
desktop, so anything short of whole there is a decision nobody made.

Infrastructure is deliberately not floored. Its Windows half needs a real
desktop, a real registry and real windows to move; a number over those would
either mean nothing or push the tests into asserting whatever happened to be
on screen. What it does have is listed below.

## What the tests prove

### The rules about a profile

`internal/domain` holds what a profile is and what a placement means, with no
I/O and no clock: `profile_test.go` covers naming, entries and the default
marking, `geometry_test.go` the rectangles, the display matching and the
window states.

### The decisions

`internal/application` is the largest suite, because every decision lives
there behind an interface and every failure can therefore be caused on demand
with a hand-written fake. It covers the capture and what it leaves out, the
restore and its lifecycle, waiting for a window that has not appeared, the
report, the tray, profile naming and marking, the update check and the failure
wording.

There are no mocking libraries. The fakes are written by hand, in
`fakes_test.go` and `fakes_store_test.go`, with fields that inject failures.

### The infrastructure that can be exercised

| Package | What its tests use |
|---|---|
| `store` | a real temporary directory: round trips, the file's shape, a missing file, a corrupt file, an unwritable directory, an interrupted write that must leave the old file whole |
| `settings` | the same, for `settings.json` and its absent-means-on reading |
| `runlog` | a real log file, its header and its steps |
| `clock` | the injected clock the domain is given |
| `instance` | a real named mutex |
| `win32` | the real Windows naming and probing paths, behind a build tag |
| `setup` | the route decisions, the payload extraction with its fence against an archive entry that climbs out of the install directory, the version comparison |

`startup`, `update`, `window` and `internal/ui` have no tests of their own:
they are the registry, the network and the desktop. What can be decided about
them was moved into `internal/application`, which is tested; what is left is
the call itself.

### The wire between the program and its pages

Neither page has a build step, so nothing compiles or type checks them. Three
structural tests stand in for that: a page may read only fields the
program actually sends; it may call only methods the program actually binds;
it may not write the product's name down anywhere. Both pages are held to all
three.

The facade's own test, `wire_test.go`, holds the other rule that the page
depends on: no list may reach a page as `null`. A nil slice is marshalled as
null rather than as an empty array, which a page then reads a length from and
throws, leaving the window on the panel it was waiting with. That was a real
defect, on 2026-09-20, on a capture that found nothing unreadable.

### The structure

`tests/structural` parses the source and fails on a layer violation, an
impure domain, a second composition root, a file over 400 lines or in the 381
to 399 band, an undocumented exported type, a program ended above
infrastructure, a decision that is not portable, a timing value with two
homes, the product's name written twice, a page naming the product, a wire
that disagrees with itself and a shared asset that has drifted from its
master in `assets/`.

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
They are checked on the reference machine and recorded in `REQUIREMENTS.md`
beside each requirement.

| Check | How |
|---|---|
| A capture reads the desktop as it is (FR-010) | Open a handful of windows, capture, compare the review against the screen. |
| A restore puts them back (FR-041) | Move the windows, apply the profile, look. |
| No program is ever ended (FR-029) | Apply a profile with unsaved work open, the setting for unnamed windows off; nothing is lost. |
| Closing the windows a profile does not name (FR-064) | Turn the setting on, sign out and in again, watch where the unnamed applications go: the ones that live in the notification area should be there rather than on the taskbar. Then press Apply with the setting still on; then quit from the tray and start it from the shortcut. Neither should close anything. |
| A window that refuses to close is minimised (FR-064) | With the setting on, leave an unsaved document open in an application no profile names, sign out and in again, then answer the prompt it puts up. |
| The report names what it could not do (FR-044) | Apply a profile naming an application that is not installed. |
| The bar moves while a restore runs (FR-065, FR-066) | Apply a profile holding several applications that are not running: the bar should fill as each is placed. Closing the report should leave the profile list showing. |
| Sign-in (FR-053) | Tick the option, sign out and in again. |
| A packaged application is started by the activation manager (FR-067) | Sign in and read the log: for each packaged application it says whether the activation manager started it or the shell was asked instead. The grey taskbar buttons are a known limit in ARCHITECTURE.md, not a failure of this check. |
| Every window a profile records is opened (FR-069) | Capture with two Notepad windows open, close Notepad, then apply the profile: both windows should come back and be placed, with the unnamed windows closed straight after rather than at the ceiling. |
| A packaged application is launched and left where it opens (FR-070) | Capture with Claude and Windows Terminal open: the review should show them with no window placed. Sign out and in: both should start, neither should be moved and the restore should settle as soon as they are running. Then look at the taskbar before clicking it. |
| A sign-in start opens no window (FR-048) | Sign in with the option ticked: the agent should be in the notification area only. The log says so where the page asked for the keyboard and was left alone. |
| Several displays, mixed scaling | Capture and restore across monitors at different scales. |
| The update check (FR-058, FR-059) | Once per run against the real release feed, then again with the setting off, where nothing should reach the network. |
| Setup, including what an uninstall removes (DATA-005) | Install, update, go back a version, repair and uninstall, each once, then inspect the registry and the folders. |

See also [DEVELOPMENT.md](DEVELOPMENT.md) for building it and
[ARCHITECTURE.md](ARCHITECTURE.md) for the invariants each structural test
guards.
