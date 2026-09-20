# ScreenState Architecture

A background agent that puts the user's windows back where they belong after signing in. A profile
records which applications should be running and where their windows sit; a restore reads one and
makes the desktop match it.

It is local-first and offline. Nothing it needs arrives over a network; it writes nothing outside
the signed-in user's own directories. It never ends a program it did not start.

This file describes what is built. What is specified but not yet built is listed at the end rather
than described here as though it existed.

## Invariant

`UI -> Application -> Domain <- Infrastructure`

Dependencies point inward. The Domain is the stable core and depends on nothing. Every rule below is
enforced by a test under `tests/structural`, not by convention. The table is the whole set: a guard
that is not listed here is undiscoverable, so a rule claimed in prose and enforced nowhere reads
exactly like one that holds.

Each assertion was proved to bite by planting a violation against it and reading the exit code, with
every plant restored afterwards. An assertion never seen to fail is not yet a guard.

| Invariant | Enforcing test | File |
|---|---|---|
| Domain imports nothing from application, infrastructure or ui | `TestDomainHasNoOutwardImports` | `boundary_test.go` |
| Domain is pure: no net, os, filepath, syscall, log, `time.Now`, `math/rand` | `TestDomainIsPure` | `boundary_test.go` |
| Application never imports infrastructure or ui | `TestApplicationDoesNotImportInfrastructure` | `boundary_test.go` |
| The composition root is the only place that wires concrete adapters | `TestCompositionRootIsWhitelisted` | `boundary_test.go` |
| No Go source file exceeds the module-size limit | `TestNoFileExceedsLineLimit` | `boundary_test.go` |
| No source file sits in the danger band below the limit | `TestNoFileInDangerBand` | `boundary_test.go` |
| Every exported type carries a doc comment | `TestEveryExportedTypeIsDocumented` | `boundary_test.go` |
| No port above the Windows layer can terminate or kill anything; only the three methods FR-064 names may close a window | `TestNothingAboveInfrastructureCanEndAProgram` | `rulings_test.go` |
| Domain and application import no Windows API | `TestTheDecisionsStayPortable` | `rulings_test.go` |
| Every timing has one home in `ports.go` | `TestEveryTimingHasOneHome` | `rulings_test.go` |

The last three are not style. Each holds a decision the specification makes that a later edit could
undo without anybody noticing.

## Layers

- **Domain** (`internal/domain`): pure Go, standard library only. Immutable value objects validated on
  construction, with `With*` copy methods for change. `ShowState` and `Rect` in `geometry.go`,
  `ApplicationIdentity` and `DisplayIdentity` in `identity.go`, `Placement`, `Entry` and `Profile` in
  `profile.go`. No IO and no wall-clock reads: time enters the product through an injected clock and
  never reaches here at all. This is where a profile gets its meaning and its rules for being valid.
- **Application** (`internal/application`): the two use cases, `CaptureService` and `RestoreService`,
  plus the ports they depend on (`Desktop`, `Processes`, `Launcher`, `ProfileStore`, `Clock`, `Log`).
  Depends on Domain and the standard library only. Every rule about what a restore does lives here and
  is exercised against hand-written fakes, on any machine, with no desktop.
- **Infrastructure** (`internal/infrastructure`): concrete adapters implementing the Application ports.
  `win32` reads and moves real windows and displays, `store` keeps the profiles, `clock` is the real
  clock, `runlog` is the step log and `instance` is the single-instance mutex. Never imported by Domain
  or Application.
- **UI** (`internal/ui`): the notification area icon, its menu, the message loop that serves them and
  the keyboard handover for the manager's webview, a client of the Application use cases only. The
  manager window itself is a page under `frontend/dist`, bound to `app.go` and `app_profiles.go`,
  which are clients of the use cases too and reach no further.

`internal/product` sits beside these holding the product's name and its tagline, which reach a path, a
mutex, a log line, a menu entry and the setup program's header. A second copy of a name is how a rename
leaves one surface still announcing the old one.

`internal/infrastructure/settings` keeps the few choices remembered between runs: whether the update
check is wanted (FR-059), which released version the user passed over (FR-058) and whether a restore
closes the windows a profile does not name at sign-in rather than minimising them (FR-064). It is deliberately
apart from the profile store, because a profile is the user's work and a setting is a preference.
`internal/infrastructure/update` is the release feed.

Two further packages serve the setup program rather than the agent. `internal/infrastructure/setup`
holds the install policy: the per-user paths, the payload extraction, the registry entries, the
shortcuts and the process work. `internal/infrastructure/window` gives a WebView page the keyboard,
which on Windows nothing else reliably does. Neither is imported by the agent.

## Composition root

`main.go` is the single composition root. It opens the log, takes the single-instance mutex, builds the
concrete adapters and injects them into the use cases by constructor injection. There are no global
singletons, no service locator and no auto-wiring.

The structural test whitelists `main.go` alone. That whitelist is load-bearing rather than decorative:
emptying it makes the test fail naming `main.go`, which is how it was confirmed that the root is seen
at all.

## Dependency direction

```
            +---------------------+
      UI    |  tray, manager      |
            +----------+----------+
                       | calls
            +----------v----------+
            |     application     |  ports (interfaces) + use cases
            +----+-----------+----+
        depends on |         ^ implements
            +------v---+     |
            |  domain  |     |
            +----------+     |
                       +-----+---------------------------+
                       |        infrastructure           |
                       | win32, store, clock, runlog,    |
                       | instance, setup, window         |
                       +---------------------------------+
```

The setup program is a second program over the same infrastructure; it reaches none of the layers
above:

```
            +---------------------+
            |  installer/         |  a Wails window: the screens
            |  app.go, the page   |  and the facade over the policy
            +----------+----------+
                       | calls
            +----------v----------+
            | infrastructure/     |  paths, payload, registry,
            | setup, window       |  shortcuts, processes
            +---------------------+
```

It knows nothing of the domain, the use cases or the agent's own UI. What it installs is a file.

## The end-state model

The single most important design decision. A profile describes what the desktop should look like, never
how to get there. Nothing in it is a step.

That is what lets each entry converge on its own (FR-055). A restore places an entry the moment the
window it names exists, without waiting for any other entry. The earlier design waited for the whole
profile to settle before placing any of it, which made the restore wait for its slowest application
while the desktop sat wrong and required a definition of "finished" that Windows cannot supply. Three
requirements were withdrawn when it went.

The consequences run right through the layer:

- An entry with placements is satisfied when every placement has been applied.
- An entry with no placements says the application should run without saying where. It is satisfied by
  the application running; nothing of its is moved (FR-005). That is how the tray applications on
  the reference machine are usually left.
- An entry recorded as not running is left entirely alone. A restore ends nothing.
- The ceiling bounds how long the agent keeps waiting for windows that may never appear (FR-023). It is
  a policy choice about when to stop waiting, deliberately not a prediction of how long the machine
  takes to start.

## Naming things so they survive

Measured on 2026-09-19 and recorded in appendix E of `REQUIREMENTS.md`. The rules live in
`internal/infrastructure/win32/naming.go`, which carries no Win32 call at all and is therefore settled
by tests on any machine.

**An application** is named by whichever of three things survives its updates:

| Installed as | Named by | Worked example |
|---|---|---|
| A Store package | its application user model id, which carries no version | Claude |
| A versioned directory beside an updater | the updater command, which does not move | Discord |
| Anything else | its own path | Stellody |

A window class is not an identity. NordVPN's main window class carried a GUID that changed on every
reboot; the process image path did not.

**A display** is named by the device instance path from its device interface name, for example
`DISPLAY#HSJ1340#5&14514d51&0&UID4356`. Not by the number Windows Settings shows, not by the device
name: both were measured disagreeing with each other and with where the screens physically sit. Two
screens of the same model differ only in the UID, so a model code alone would place a window on the
wrong screen.

## Placing a window

The order is `restore, set the rectangle, then set the show state`; it is not a preference.
Maximising acts on whichever display the window's rectangle is on, so maximising first would maximise
it where it already was; and a maximised window ignores a move until it has been restored. Measured
from a process with no administrator rights, across a 96 dpi to 240 dpi boundary.

A profile records the **normal** rectangle even for a maximised window, because that is what decides
which display maximising puts it on.

Two rules keep a window reachable. A placement naming a display that is no longer connected is applied
to the primary display and the substitution is recorded (FR-031). Every window is then held within the
display it lands on (FR-032), keeping its size wherever it fits, because a window silently resized is
one the user has to put right by hand.

A window straddling two displays belongs to the one holding the larger share of it, which is how
Windows itself decides where to maximise it.

## What a restore does

1. Launch every application the profile records as running that is not running (FR-024), once, at the
   start, so they load alongside each other. Nothing already running is launched again (FR-025).
2. On each pass, read the displays and the windows as they then stand; take every outstanding entry
   as far as the windows now open allow.
3. An entry whose application is running with no visible window has the application run again, which
   signals the instance already running to show and draw its own window (FR-036, FR-056). Acting on the
   hidden window from outside was measured producing an empty frame the application was not drawing.
4. Once placed, a window is read again after the settle-check delay and put back **once** if the
   application has moved it (FR-033). After that one further attempt the agent gives up and says so,
   rather than fighting an application for its own window (FR-034).
5. Displays arriving or going away mid-restore do not abandon it: the remaining entries are placed
   against the displays as they then stand and the change is recorded (FR-057).
6. A restore requested while one is running **replaces** it (FR-061). The running restore stops before
   its next action, every window already placed is left exactly where it is; both reports say what
   happened. Nothing is put back.

Every one of those is exercised against fakes in `internal/application`, which holds 100% coverage.

## What a restore never does

It never terminates a process it did not start (FR-029, C-3). It closes a window only during the restore that runs at
sign-in, only where the user has turned FR-064 on and only a window no profile names. Apply
minimises whatever the setting says; so does the restore that runs when the agent is started by
hand. The two are told apart by the flag the sign-in entry passes, since the agent restores the
default profile on any start where no copy of it is already running.

Closing was forbidden outright until the owner asked for it back. NordVPN and GameGlass survive their
windows closing; Postal Gambit is ended by it. Nothing about a window says which kind it is, so an
agent that decided by itself to tidy a desktop by closing windows would be quitting applications and
taking whatever was unsaved in them. What changed is who decides: the setting is off until the user
turns it on and states what it costs beside the control, because the applications it is aimed at are
the ones that go to the notification area rather than ending.

The bound is held by the shape of the code rather than by a rule someone has to remember.
`TestNothingAboveInfrastructureCanEndAProgram` fails any port above the Windows layer that grows a
method to terminate anything; it allows exactly three that speak of closing: `Desktop.Close`, which
asks one window, plus the two halves of the setting that decide whether it is asked at all. A fourth
fails the suite. A window that is asked and does not go is minimised instead, which is what the
setting's other arm would have done; the report says which windows those were.

## Data locations

| What | Where |
|---|---|
| Profiles | `%LOCALAPPDATA%\ScreenState\profiles\<name>.json`, one file each |
| Step log | `%LOCALAPPDATA%\ScreenState\Log.txt` |
| Installed files | `%LOCALAPPDATA%\Programs\ScreenState\`, the agent, its licence and a copy of setup |
| Settings | `%LOCALAPPDATA%\ScreenState\settings.json`, the update setting, the skipped version and what a restore does with the windows a profile does not name |
| Webview cache | `%LOCALAPPDATA%\ScreenState\webview`, pinned there so an uninstall knows to look |
| Apps list entry | `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\ScreenState` |
| Sign-in entry | `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, value `ScreenState` (FR-046) |
| Shortcuts | the user's own Start Menu Programs folder and Desktop |

Everything the product writes lives under the signed-in user's own directories (C-2, DATA-001). There
are no machine-wide files and no machine-wide registry keys; it never asks for administrator rights
(C-1). The setup program holds to the same rule, which is why it can install without an elevation
prompt.

The install folder and the folder holding the profiles are deliberately different places, so removing
the product does not remove the user's captures unless they ask. The setup program derives the second
from the store rather than rebuilding it, so what an uninstall offers to clear cannot drift from what
the agent writes.

Profiles are files rather than a database on purpose. A profile is a few hundred bytes that changes when
the user says so, never concurrently; a file each means a profile that cannot be read costs the user
that profile rather than all of them.

Every write is atomic (FR-006): the bytes go to a temporary file in the same directory, reach the disk,
then move onto the target. The same directory matters, because a move between directories is a copy and
a delete rather than one act; a copy can be interrupted half way.

Every profile carries the format version it was written in (DATA-002). A file from a version this build
does not understand is left exactly as it is, kept out of the listing, with the reason stated
(DATA-003).

## The manager window

The agent has one window and it is a Wails window, which is why the agent is built by `wails build`
rather than by `go build`. Wails owns the main thread; the notification area icon keeps a locked
thread of its own, because a window belongs to the thread that made it. They meet at one point: the
tray is handed a callback and asks for the manager rather than opening anything itself.

**The window is a client of the use cases and nothing else.** `app.go` holds the window plumbing and
the shapes crossing the boundary; `app_profiles.go` holds the bound methods, each of which is one
call into `ManagerService`, `CaptureService` or `RestoreService`. No rule about what a profile means
lives in either, so none of them can be got wrong there without a test in the application layer
failing first. The composition-root test proves it: `main.go` is still the only file that knows both
the application layer and the Windows layer.

**The page is a script per subject, not one file.** `manager.js` holds what the panels share, the
buttons that belong to the window and the start of a run; beside it sit the interrupting surfaces
(`manager-dialogs.js`), the profile list and everything done to a profile (`manager-profiles.js`),
the capture review with the restore report (`manager-capture.js`), the settings panel
(`manager-settings.js`) and everything about the product rather than the desktop
(`manager-help.js`), with the guide's words apart again in `guide.js`. They are plain scripts sharing
one scope, so `index.html` loads `manager.js` first and a structural test holds the tags and the
files to each other.

**A sign-in start opens no window.** The setup program writes the sign-in entry with a flag that
keeps it shut, so the agent waits in the notification area (FR-046, FR-048); launched by hand it
opens the manager, which is what double-clicking a shortcut means. An entry written without that
flag reads as off, so turning the setting on rewrites it correctly rather than leaving a sign-in
that opens a window over whatever the user is doing.

**A second launch asks the first for its manager** (FR-054). It finds the running copy's hidden
window by its class and posts a message registered by name, which is the documented way for two
programs to agree on one without either inventing a number (EIR-004). It never arranges the desktop
a second time.

**The restore runs alongside the window rather than before it.** A restore waits for windows to
appear and may take minutes; a manager that could not be opened until it finished would be shut for
the whole of the time a user most wants to look at it. FR-048 asks only that the restore complete
without the window being opened, which it does.

**Nothing is written until a capture is confirmed.** The review is held in the facade between
reading the desktop and confirming what to keep, so a cancelled capture leaves nothing behind
(FR-011, FR-016), including on disk. Deleting a profile is a panel of its own naming it (FR-043),
never a box over the window: the go-ahead there is a button that has never meant anything else.

**The palette and the page furniture have one home.** `assets/theme.css` and `assets/shell.js` are
copied into both page directories by `build.ps1`, because each window embeds its own page and a copy
is the only way to share them. A structural test fails when a copy has drifted, which is the only
thing standing between a copy edited in place and two windows that quietly stop matching.

## The update check

The one outbound connection this product makes, which is what C-4 names: an anonymous request to a
public release feed, asking one question and carrying nothing about the user or the desktop.

**It runs once per run** (FR-058), a moment after the window is ready rather than during startup. It
says nothing unless there is something to say. A feed that cannot be reached, a machine that is
offline and a release that is not newer are all silence: the user did not ask, so they hear nothing.
A check the USER asked for, from Help, reports every outcome including the two that are not news,
because a button that sometimes does nothing visible is a button people stop trusting.

**The endpoint's own contract is the guard.** GitHub's releases/latest answers only a published
release that is neither a draft nor a pre-release, so a tag pushed mid-development is structurally
invisible and work in progress can never raise a prompt. Nothing re-checks those flags here.

**Anything unreadable compares as not newer.** A malformed tag can never raise an offer. That
direction is deliberate: a missed offer costs a user a day, while a spurious one costs the credibility
of every later offer.

**Skipping silences the check that speaks unbidden, never the one the user pressed a button for.**
That is the whole of what skipping is for, so a manual check reports a skipped version as available
and offers the download anyway.

**Turned off, nothing is asked of the network at all** (FR-059). The setting is read before the feed
is touched, so the promise is a shape rather than an intention. The manual entry in Help is switched
off with it: an entry that reached the network while the setting said otherwise would make
the setting a lie.

## Help, the guide and the licence

Help is a panel of rows rather than a menu, because this window has no menu bar and a made-up one
would be furniture nobody expects. It offers the guide, About, the licence, the update check and the
report of the last restore.

The guide's words live in `frontend/dist/guide.js`, apart from the panel that draws them, so the panel
stays a renderer and the words stay one readable document. Every entry carries the REAL control this
window draws: the image files the window loads, plus the window's own classes for the drawn ones, so
the guide cannot come to show something the window does not. A guide showing anything else is worse
than no guide. It names the furniture first, then states the rules the window cannot say for itself:
what is kept locally, that no program is ever ended and a window is closed only where the setting
says so, the single network call and what cannot be undone.

## The setup program

A second program in the same module, at `installer/`. The agent is a native Windows program with no
window; setup needs one, so it is built as a Wails application and wears a palette sampled from the
agent's own artwork. It carries the built agent as an embedded zip, so one downloaded file is the whole
distribution.

**The install policy is infrastructure, not interface.** `internal/infrastructure/setup` owns the
paths, the fenced payload extraction, the version comparison, the registry writes, the shortcut
handling and the process work. `installer/app.go` is a facade over it and owns no install logic at all,
so what an install does can be read in one place and exercised without a window. The portable half
carries unit tests; the Windows half sits behind a build tag with no-op stubs beside it, so the package
still builds and vets on a machine that is not Windows.

**One reading of the machine decides everything.** `DetectState` looks once and answers the mode, the
version relation and what is already true of the shortcuts and the sign-in entry. The screen, its
heading, the options on it and the buttons under it all come from that one answer, which is what stops
those four drifting apart.

| Route | When |
|---|---|
| Install | nothing recorded in the Apps list |
| Update | the recorded version is older than the one setup carries |
| Go back a version | the recorded version is newer |
| Manage | the versions match, so there is nothing to install |

Removal is a screen reachable from every other one rather than a route of its own, so cancelling it
returns to whatever was due behind it.

**An operation moves to a different screen; nothing is greyed in place.** The progress screen offers no
actions at all, because there is nothing there that can safely be interrupted. Every path ends in a
verdict or in the agent running: setup never finishes by quietly doing nothing.

**Options open on what is already true**, never all ticked, so a user who declined a desktop shortcut is
not offered one again as though they had asked for it. On the manage screen a toggle applies
immediately, since there is no go-ahead button there for it to wait on.

**The agent is asked about before a single file is touched.** Extracting over a locked executable fails
part way and leaves a half-written install, so a running agent gets its own screen offering Cancel or
"Close it and continue". It is ended by executable name and never by process tree: descent is decided
from recorded parent process ids, which churn, so a tree kill can end setup itself and the window then
vanishes with nothing said.

**The page carries no product name.** Nothing compiles or type checks a string in a page, so a name
written there would survive a rename in silence. The name, the tagline and every path arrive on the
state the program hands over; a structural test fails if either is written into the page anyway.

## Errors

Sentinel errors tested with `errors.Is`, wrapped with `%w`, no custom error types. Absence is an answer
rather than a fault: no default profile, no window yet, no model id on an ordinary application. A fault
is the thing that exists and cannot be read; the two are worded differently.

A restore that fails unexpectedly is recorded in the log and the report and leaves the agent able to
restore again (FR-052). An agent that vanishes leaves a half-arranged desktop and nothing to read, which
is the failure this product exists to remove.

The step log takes the run's error output on a windowed build, which is given none of its own, so
everything written there would otherwise be lost, including the Go runtime's own report of a crash.

## Quality enforcement

| Gate | What it is |
|---|---|
| `gofmt` | formatting, no deviation |
| `go vet` | the standard checks |
| `staticcheck` | a stricter superset of vet |
| `go test -race` | the whole suite, with the race detector |
| Structural suite | the invariants above, each proved by a planted violation |
| Coverage floor | 100% over `internal/domain` and `internal/application` |

The floor is scoped to the two layers a machine can exercise with no filesystem, no clock and no
desktop. Anything short there is a decision nobody made. Infrastructure sits deliberately outside it:
the Windows half needs a real desktop; gating it would mean either a number that means nothing or
tests that assert what happened to be on screen.

Measured coverage at the time of writing: domain 100%, application 100%, clock 100%, store 93.8%,
instance 90.9%, win32 71.9%, runlog 66.7%. The shortfalls are IO and platform failures that would need
the disk or the window manager to fail mid-call; they are not padded with tests that assert nothing.

The structural suite also holds the setup program's boundary, which no compiler sees: the page may not
write the product's name or its tagline down, every `state.` field it reads must be a json tag the
program actually sends and every call it makes must be a method the program binds. These three are the
newest assertions in the suite and are the only ones not yet proved by a planted violation.

Two integration tests in `win32` read the real machine and assert only what must hold anywhere, skipping
where there is no desktop. They do not move a window or start an application, since either would disturb
the desktop of whoever ran the suite.

## Design decisions

| Decision | Why |
|---|---|
| A profile is an end state, never a sequence of steps | Each entry converges on its own, so the restore never waits for its slowest application |
| Placement is per entry, not per profile | Holding every placement until the whole profile was ready required a definition of finished that Windows cannot supply |
| A restore closes and terminates nothing | Closing a window ends some applications outright and takes whatever was unsaved in them |
| A newer restore replaces a running one | A restore is a statement of what the desktop should look like now, so the newest statement is the one that is true |
| Undo nothing when replacing | Putting windows back moves them twice to reach the same end |
| The normal rectangle is recorded, even when maximised | It is what decides which display maximising puts the window on |
| A display is named by its device instance path | Every number Windows offers was measured disagreeing with the others |
| An application is named by what survives its updates | A path carrying a version breaks at the next update; a window class carries a GUID that changes every reboot |
| A hidden window is shown by running the application again | Acting on the hidden window from outside produced an empty frame the application was not drawing |
| One file per profile, not a database | A profile that cannot be read costs the user that profile rather than all of them |
| The Windows layer is split into portable rules and system calls | The rules are string work and can be settled by tests on any machine |

## Known limits

**First-seen order.** FR-037 matches placements to windows in the order the agent first *saw* them,
which is what the system can actually answer: Windows records no creation time for a window and no
call reports one. During a restore first-seen order and age agree, because the agent is watching
while the windows appear. Windows already open when the agent starts all share one moment; within it
they keep the order the first enumeration gave, which is a stacking order rather than an age. So the
placements of an application whose windows were all open before the agent started are matched in
stacking order; the user cannot predict that from the order they opened them.

**Unverified against a real desktop.** Moving a window and starting an application are implemented and
unproven: no test calls either, because both would disturb the desktop of whoever ran the suite. They
need a deliberate run.

**The update check has never asked the real feed.** Its rules are covered end to end against a
stand-in; every outcome the window can show was driven in a browser. No request has left this
machine: there is no published release to find, so the happy path is proved only in the shape the
adapter promises to produce.

**The manager is proved by use rather than by test.** It has been run for real on the reference
machine: the log records captures that read the desktop, a capture cancelled without writing
anything, a profile written, a profile deleted, the default marking settled, restores at start and
the quit from both the manager and the tray. The panels themselves are still driven in a browser
against a stand-in for the agent, which settles the layout, the palette and the wiring and settles
nothing else. What neither has settled: real keyboard focus and the ring, the second-launch message,
the tray opening a named panel, the donate link and the update offer on screen. Those are read off a
run, not off a suite, so they are checked by the list in TESTING.md.

**The setup program has installed and nothing else.** It has installed on the reference machine
twice, the second time over an existing install of the same version: the files are in
`%LOCALAPPDATA%\Programs\ScreenState`, the Apps list entry carries its uninstall and modify
commands, the sign-in entry names the installed file with the flag that keeps the window shut and
both shortcuts are where they belong. So the payload extraction, the registry writes and the
shortcuts are proved for that one path. Everything else is written and not proven: an update to a
newer version, going back to an older one, repair, uninstall, the scheduled removal and the cleanup
of an install under a name the product used to carry. Its screens are still driven in a browser
against a stand-in, which settles the layout and the wiring and settles nothing else.

## Not built yet

Nothing. Every requirement in the specification is built. What remains is proving it: see the known
limits above.
