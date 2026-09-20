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
| No port above the Windows layer can close, terminate or kill anything | `TestNothingAboveInfrastructureCanEndAProgram` | `rulings_test.go` |
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
- **UI**: not built yet. The tray and the manager window will be clients of the Application use cases
  only, exactly as the agent is.

`internal/product` sits beside these holding the product's name, which reaches a path, a mutex and a
log line. A second copy of a name is how a rename leaves one surface still announcing the old one.

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
      UI    |  tray, manager      |  not built yet
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
                       | instance                        |
                       +---------------------------------+
```

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

It never terminates a process it did not start; it never closes a window either (FR-029, C-3).

Closing was allowed until it was measured. NordVPN and GameGlass survive their windows closing; Postal
Gambit is ended by it. Nothing about a window says which kind it is, so an agent that tidied a desktop
by closing windows would be quitting applications and taking whatever was unsaved in them.

This is held by the shape of the code rather than by a rule someone has to remember. The `Desktop` port
offers no method that can close or end anything; `TestNothingAboveInfrastructureCanEndAProgram`
fails any port that grows one. An application that should be present but out of the way is recorded
minimised, which is reversible and quits nothing.

## Data locations

| What | Where |
|---|---|
| Profiles | `%LOCALAPPDATA%\ScreenState\profiles\<name>.json`, one file each |
| Step log | `%LOCALAPPDATA%\ScreenState\Log.txt` |

Everything the product writes lives under the signed-in user's own directories (C-2, DATA-001). There
are no machine-wide files and no machine-wide registry keys; it never asks for administrator rights
(C-1).

Profiles are files rather than a database on purpose. A profile is a few hundred bytes that changes when
the user says so, never concurrently; a file each means a profile that cannot be read costs the user
that profile rather than all of them.

Every write is atomic (FR-006): the bytes go to a temporary file in the same directory, reach the disk,
then move onto the target. The same directory matters, because a move between directories is a copy and
a delete rather than one act; a copy can be interrupted half way.

Every profile carries the format version it was written in (DATA-002). A file from a version this build
does not understand is left exactly as it is, kept out of the listing, with the reason stated
(DATA-003).

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

**Creation order.** FR-037 matches placements to windows "in the order the windows were created".
Windows records no creation time for a window; no call answers it. What the agent uses is the order
it first *saw* each window. During a restore the two agree, because the agent is watching while the
windows appear. Windows already open when the agent starts all share one moment; within it they
keep the order the first enumeration gave, which is a stacking order rather than an age. The requirement
is not verifiable as written and wants either rewording or a different matching rule.

**Unverified against a real desktop.** Moving a window and starting an application are implemented and
unproven: no test calls either, because both would disturb the desktop of whoever ran the suite. They
need a deliberate run.

## Not built yet

The tray icon and its menu, the manager window, the review step of a capture as something a user can
see, the sign-in registry entry (FR-046), the update check (FR-058, FR-059), the donation link (FR-060)
and the setup program. The specification covers all of them; this document will describe them when they
exist and not before.
