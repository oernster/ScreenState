# ScreenState Architecture

A background agent that puts the user's windows back where they belong after signing in. A profile
records which applications should be running and where their windows sit; a restore reads one and
makes the desktop match it.

It is local-first and offline. Nothing it needs arrives over a network; it writes nothing outside
the signed-in user's own directories and registry keys. The agent never terminates a program and
closes a window only where the user has turned FR-064 on; the setup program ends only a running copy
of the agent, when the user tells it to.

This file describes what is built. What is specified but not yet built is listed at the end rather
than described here as though it existed.

## Invariant

`UI -> Application -> Domain <- Infrastructure`

Dependencies point inward. The Domain is the stable core and depends on nothing. Every rule below is
enforced by a test under `tests/structural`, not by convention. The table is the whole set, all 19:
a guard that is not listed here is undiscoverable, so a rule claimed in prose and enforced nowhere
reads exactly like one that holds.

One direction is not enforced: nothing stops `internal/ui` importing infrastructure; it does.
`ui.TakeWindowFocus` hands the manager's webview the keyboard through
`internal/infrastructure/window`, since on Windows nothing else reliably does.

Each assertion was proved to bite by planting a violation against it and reading the exit code, with
every plant restored afterwards, except the three page rules marked below. An assertion never seen
to fail is not yet a guard.

| Invariant | Enforcing test | File |
|---|---|---|
| Domain imports nothing from application, infrastructure or ui | `TestDomainHasNoOutwardImports` | `boundary_test.go` |
| Domain is pure: no net, os, filepath, syscall, log, `time.Now`, `math/rand` | `TestDomainIsPure` | `boundary_test.go` |
| Application never imports infrastructure or ui | `TestApplicationDoesNotImportInfrastructure` | `boundary_test.go` |
| The composition root is the only place that wires concrete adapters | `TestCompositionRootIsWhitelisted` | `boundary_test.go` |
| No Go source or page file (HTML, script, stylesheet) exceeds the module-size limit | `TestNoFileExceedsLineLimit` | `size_test.go` |
| No such file sits in the danger band below the limit | `TestNoFileInDangerBand` | `size_test.go` |
| Every exported type carries a doc comment | `TestEveryExportedTypeIsDocumented` | `boundary_test.go` |
| No port above the Windows layer can terminate or kill anything; only the three methods FR-064 names may close a window | `TestNothingAboveInfrastructureCanEndAProgram` | `rulings_test.go` |
| Domain and application import no Windows API | `TestTheDecisionsStayPortable` | `rulings_test.go` |
| Every timing in the application layer has one home in `ports.go` | `TestEveryTimingHasOneHome` | `rulings_test.go` |
| The product's name is written down once, in `product.go` | `TestTheProductIsNamedOnce` | `rulings_test.go` |
| Every application started through the shell is shown without activating (FR-077) | `TestNothingIsStartedInFront` | `rulings_test.go` |
| The shared palette and page furniture match their masters in `assets/` | `TestTheSharedAssetsHaveNotDrifted` | `shared_assets_test.go` |
| The manager's page names nothing: no product name, no tagline | `TestTheManagerPageNamesNothing` | `shared_assets_test.go` |
| Every script and stylesheet in the manager's page is loaded by its `index.html` | `TestEveryManagerScriptIsLoadedByThePage` | `shared_assets_test.go` |
| The manager's page reads only fields the program sends and calls only what it binds | `TestTheManagerWireIsStatedTwiceAndAgrees` | `shared_assets_test.go` |
| The setup page names nothing (not proved by a plant) | `TestTheSetupPageNamesNothing` | `setup_page_test.go` |
| The setup page reads only fields the program sends (not proved by a plant) | `TestTheWireIsStatedTwiceAndAgrees` | `setup_page_test.go` |
| The setup page calls only what the program binds (not proved by a plant) | `TestThePageCallsOnlyWhatIsBound` | `setup_page_test.go` |

The timing rule covers the application layer, where every decision about waiting lives; a timeout
inside an adapter (the release feed's, the setup program's wait for the agent to close) and the
fallback caret blink FR-080 names sit beside the call they bound.

The `rulings_test.go` rows are not style. Each holds a decision the specification makes that a
later edit could undo without anybody noticing.

## Layers

- **Domain** (`internal/domain`): pure Go, standard library only. Immutable value objects validated on
  construction, with `With*` copy methods for change. `ShowState` and `Rect` in `geometry.go`,
  `ApplicationIdentity` and `DisplayIdentity` in `identity.go`, `Placement`, `Entry` and `Profile` in
  `profile.go`. No IO and no wall-clock reads: time enters the product through an injected clock and
  never reaches here at all. This is where a profile gets its meaning and its rules for being valid.
- **Application** (`internal/application`): the use cases, `CaptureService`, `RestoreService`,
  `ManagerService`, `TrayService` and `UpdateService`, plus the ports they depend on: `Desktop`,
  `Processes`, `Launcher`, `ProfileStore`, `Clock`, `Log`, `StrangerPreferences`,
  `CeilingPreferences`, `Splash` and
  `DesktopEvents` with its `DesktopWatch` in `ports.go`; `Startup`, `ReleaseSource` and
  `UpdatePreferences` beside the services that use them. Depends on Domain, `internal/product` (the
  tray's tooltip carries the name) and the standard library only. Every rule about what a restore does lives here and is exercised against hand-written fakes,
  on any machine, with no desktop.
- **Infrastructure** (`internal/infrastructure`): concrete adapters implementing the Application ports.
  `win32` reads and moves real windows and displays and hears the desktop change, `store` keeps the
  profiles, `clock` is the real clock, `runlog` is the step log, `instance` is the single-instance
  mutex and `startup` is the sign-in entry. Never imported by Domain or Application.
- **UI** (`internal/ui`): the notification area icon, its menu, the message loop that serves them,
  the splash and the keyboard handover for the manager's webview. It calls the Application use cases
  plus, for the keyboard handover alone, `internal/infrastructure/window`. The manager window itself is
  a page under `frontend/dist`, bound to `app.go`, `app_profiles.go` and `about.go`. They are clients
  of the use cases too; beyond them they reach only `internal/ui`, for the keyboard handover and to
  refresh the tray. `splash.go` beside them reads the palette and the artwork once for the two
  surfaces the agent draws itself.

`internal/product` sits beside these holding the product's name and its tagline, which reach a path, a
mutex, a log line, a menu entry and the setup program's header. A second copy of a name is how a rename
leaves one surface still announcing the old one.

`internal/infrastructure/settings` keeps the few choices remembered between runs: whether the update
check is wanted (FR-059), which released version the user passed over (FR-058), whether a restore
closes the windows a profile does not name at sign-in rather than minimising them (FR-064) and how
long a restore waits for windows that have not appeared (NFR-PERF-003). It is
deliberately apart from the profile store, because a profile is the user's work and a setting is a preference.
`internal/infrastructure/update` is the release feed.

Two further packages were written for the setup program and are shared with the agent.
`internal/infrastructure/setup` holds the install policy: the per-user paths, the payload extraction,
the registry entries, the shortcuts and the process work; the agent uses it for the sign-in entry and
for whether Windows is set to dark. `internal/infrastructure/window` gives a WebView page the
keyboard, which on Windows nothing else reliably does; both programs need that.

## Composition root

`main.go` is the single composition root. It opens the log, takes the single-instance mutex, builds the
concrete adapters and injects them into the use cases by constructor injection. There is no service
locator and no auto-wiring; nothing in the domain or the application holds state at package level.
The code behind a Win32 callback does, because the callback carries no pointer back to the object
that set it: in `internal/ui` the tray and the splash each keep their one live instance; in `win32`
the event hooks keep the watches running now, the window enumeration keeps the list being gathered
and the shell's registered message number is read once.

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
                       | instance, startup, settings,    |
                       | update, setup, window           |
                       +---------------------------------+
```

The setup program is a second program over the same infrastructure; its own code calls nothing
above it:

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

It calls no use case and knows nothing of the agent's own UI. It does link the store, since
`setup.StateDir` asks the store where the profiles live, which brings the domain and application
packages the store implements into its build. What it installs is a file.

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
  the reference machine are usually left. A capture offers such an application unticked, apart
  from the entries: `Desktop.Background` answers the programs owning a hidden window of the
  candidate shape and no candidate window shown, leaving out any whose program cannot be read and
  any under the Windows directory (`partOfWindows`). `CaptureService.addBackground` drops those
  the review already holds and this product itself. Measured on 2026-09-21 that was still 25
  applications, helpers another application starts among them, which is why the user ticks the
  ones to keep rather than unticking the rest.
- An entry recorded as not running is left entirely alone. A restore ends nothing.
- The ceiling bounds how long the agent keeps waiting for windows that may never appear (FR-023). It is
  a policy choice about when to stop waiting, deliberately not a prediction of how long the machine
  takes to start. The user sets it in the manager, 15 minutes until they do (NFR-PERF-003); each
  restore reads it through `CeilingPreferences` as it begins and counts it from that moment, so a
  changed ceiling governs the next restore without a restart. The `Policy` the service is built
  with holds the default and the bounds a choice is held within (`ceiling.go`).

## Naming things so they survive

Measured on 2026-09-19 and recorded in appendix E of `REQUIREMENTS.md`. The rules live in
`internal/infrastructure/win32/naming.go`, which carries no Win32 call at all and is therefore settled
by tests on any machine.

**An application** is named by its path, which is what starts it the way the user's own double-click
does, with two rules around that (FR-071, measured 2026-09-21):

| Installed as | Named by | Worked example |
|---|---|---|
| A Store package | its path, keeping its model id beside it: the path carries the version, the model id does not | Claude |
| A versioned directory beside an updater | the updater command, which does not move | Discord |
| Anything else | its own path | Stellody |

The kept model id does three jobs once an update has moved the path: the package family inside it
matches the application's new directory, so a running copy is still recognised; the activation
manager still starts it; a window reporting the new path carries the same model id, so
`ApplicationIdentity.Recognises` in the domain matches it to its entry, so the restore places it
rather than putting it away. Two callers use it: `windowsOf` finds an entry's windows to place;
`Profile.Find` decides what the profile does not name. None of them needs the profile
recaptured. `Equal` stays exact, since it says whether two stored entries are the same one.

A window class is not an identity. NordVPN's main window class carried a GUID that changed on every
reboot; the process image path did not.

**A packaged application falls back on the activation manager.** A packaged application is started
from its path like any other (FR-071). The model id is used in two cases: the path fails to start
it while a model id is kept beside the path; an entry saved before the path was kept names the
application by its model id alone. Either way the model id goes to `IApplicationActivationManager`
first and to the shell only where that fails, so the
worst case is the older route (FR-067). The activation manager was brought in because the taskbar
buttons of packaged applications were drawn grey after a sign-in on the reference machine; a boot on
2026-09-21 measured that it does not cure that (the taskbar click in step 7 of What a restore does is
the repair). It stays as the route Windows documents for packaged applications; the log names the
route that started each one. The call is COM with no cgo: a hand-written method table on a goroutine
locked to its own thread, which is never unlocked, so the thread's COM state ends with it. The
comment in `activate_windows.go` records what was tried and measured not to work, so nobody tries it
again.

**A display** is named by the device instance path from its device interface name, for example
`DISPLAY#HSJ1340#5&14514d51&0&UID4356`. Not by the number Windows Settings shows, not by the device
name: both were measured disagreeing with each other and with where the screens physically sit. Two
screens of the same model differ only in the UID, so a model code alone would place a window on the
wrong screen.

## Placing a window

**Nothing a restore places is activated** (FR-074). `ShowWindow` with restore or maximise activates
the window, which lights its taskbar button: a taskbar marks the window last activated on its
display, so a restore that used them left one lit button per display. The window is shown with
`SW_SHOWNOACTIVATE` and moved with `SWP_NOACTIVATE`. Maximising is the hard part: every direct way
activates the window, `ShowWindow` with `SW_MAXIMIZE`, `WM_SYSCOMMAND` with `SC_MAXIMIZE` and
`SetWindowPlacement` with a maximised show state alike. This file used to say the last did not; a
probe on 2026-09-21 counted the activation. What does not activate is two steps: the placement is
set to minimised with `WPF_RESTORETOMAXIMIZED`, then the window is shown with `SW_SHOWNOACTIVATE`,
which restores it maximised on the display its normal rectangle is on. It costs a minimise and
restore animation. `TestMaximisingDoesNotActivate` holds it: it runs only when
`SCREENSTATE_DESKTOP_PROBE` is set, since it opens two windows of its own and takes the front. It
fails with the old call planted back.

**Nothing a restore starts asks for the front** (FR-077). The agent is started at sign-in and
holds no right to the foreground, so an application it starts with `SW_SHOWNORMAL` asks for the
front and is refused; Windows then marks its taskbar button red to ask for attention. `ShellExecuteW`
is given `SW_SHOWNOACTIVATE` instead. Measured on 2026-09-21 from a process started at sign-in with
Claude in front: Windows Terminal by path with the ordinary show state flashed and went red, shown
without activating it came up plain behind Claude. The two model-id routes, `shell:AppsFolder` and
the activation manager, pass no show state on and still go red. They are taken only once an update
has moved a packaged application's path; the one exception is an entry saved before the path was
kept, which names the application by its model id alone.

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
   start, so they load alongside each other. Nothing already running is launched again (FR-025),
   except as step 3 asks.
2. On each pass, read the displays and the windows as they then stand; take every outstanding entry
   as far as the windows now open allow. Between passes the restore waits for Windows to say the
   desktop changed (a window created, shown, hidden, cloaked, uncloaked, destroyed or moved; the
   displays changing) and for nothing else: it never looks again on a timer (FR-079). Where the
   desktop cannot be watched at all, the log and the report say so and the restore reads the
   desktop once more when the ceiling passes. The `DesktopEvents` port carries that;
   `internal/infrastructure/win32/events_windows.go` implements it with window event hooks,
   low-level keyboard and mouse hooks and a hidden window that hears the displays change plus, registered as a shell hook window, each taskbar button flashing
   (`flash_windows.go`), all on a thread of their own and taken down when the watch ends.
3. An entry with fewer windows showing than the profile records has the application run again, once
   per missing window, each run waiting for the window it opens before the next (FR-069). With no
   window showing, the run signals the instance already running to show and draw its own (FR-036,
   FR-056); acting on the hidden window from outside was measured producing an empty frame. With
   some showing, the run is for another window, since some applications open one each time they are
   run; that is done only at sign-in or for an application this restore started itself. An
   application that was already running with windows of its own keeps the windows it has and its
   entry is settled as it stands, since those are the windows the user has. A run not yet answered
   by a window is never followed by another: on 2026-09-21 every launched application was started
   twice in the same second, which is a second copy FR-025 forbids. So an application this restore
   started that goes straight to the tray stays there; the report says so.
4. Whatever the restore needs to know will not happen (a window that never comes, a window asked to
   close that stays open) it stops waiting for at whichever comes first of the user's first key press
   or mouse click and the ceiling, then reports it (FR-079). The ceiling is the only timer save
   one: the flash series step 8 waits out (FR-080).
5. A placed window is read again each time the desktop changes and put back **once** if the
   application has moved it (FR-033). After that one further attempt the agent gives up and says so,
   rather than fighting an application for its own window (FR-034). The watch goes on after the
   restore has ended, in a watch of its own, until the user's first key press or click, the ceiling,
   a newer restore or the user stopping one; what it does then goes to the log, since the report has
   been handed over.
6. Once every entry is settled, each window the profile does not name is put out of the way
   (FR-063). It is minimised; at sign-in with FR-064 turned on it is asked to close instead (see
   What a restore never does). This product's own windows and windows already minimised are left alone.
7. Then every taskbar is posted a left click (FR-072). Explorer draws the button of an application
   started at sign-in without its icon on every display but the first; it leaves that button grey
   until any taskbar is clicked; the same happens mid-session with this product not running, so the
   fault is Windows and the click is the repair. The message goes straight to the
   taskbar's window, so the pointer does not move, nothing is activated and no application's window
   is touched.
8. Once a sign-in restore has settled, the shell is made to build the taskbar button of every
   window it placed afresh, by hiding the window and showing it again (FR-075). A taskbar marks the
   window last activated on its display and an application puts its own window in front as it
   starts, so a desktop assembled at sign-in comes back marked although the recorded desktop was
   not. Rebuilding the button is the only measured way to clear that mark; it costs a flicker. Any
   other restore does it only for the applications it started itself, since nothing
   else can have gained a mark once placing stopped activating (FR-074). A button that cannot be
   rebuilt costs that button alone: the rest are still rebuilt and the log counts both, while a
   restore stopped part way rebuilds no more.
   Before rebuilding, the restore waits until none of those windows has flashed for one full flash
   series (FR-080). An application started without the front asks for it anyway; Windows refuses
   and flashes its button, in a series that outlasted the rebuild by up to 7 seconds in a shell trace
   on 2026-09-21, leaving the mark back. Nothing says a series has ended, so each flash the shell
   reports begins the wait again. The series is the foreground flash count times two caret blinks,
   read from Windows each time, with 530 milliseconds standing in for a caret that does not blink.
   The user's first key press or click and the ceiling end the wait; only then is the button rebuilt
   and the splash told the desktop is ready. A rebuild does not clear the red a finished series leaves
   on a button, so after each rebuild the taskbars are posted the shell's own "window activated"
   notice for that window, then for the window that really has the front; nothing is activated.
9. Displays arriving or going away mid-restore do not abandon it: the remaining entries are placed
   against the displays as they then stand and the change is recorded (FR-057).
10. A restore requested while one is running **replaces** it (FR-061). The running restore stops
    before its next action, every window already placed is left exactly where it is; both reports
    say what happened. Nothing is put back. The user can stop a restore the same way without
    starting another (FR-049): "Stop the restore" on the Applying panel calls `App.CancelRestore`,
    which is `RestoreService.Cancel`. The report then says the restore was cancelled.

Every one of those is exercised against fakes in `internal/application`, where every function is
covered by a test.

**Only a sign-in arranges the desktop by itself** (FR-038). The agent restores the default profile
when it is started with `-hidden` (the flag the sign-in entry passes) and on no other start. It used
to restore on every start: installing rearranged the desktop and opened another window of any
application whose entry records more than one, which is why setup starts it with `-quiet` (FR-076);
a start by hand rearranged the windows the user was working in and left a splash over the manager
they had asked for. Both now open the manager and arrange nothing, saying so in the log; Apply is
the way to arrange the desktop mid-session.

## What a restore never does

It never terminates a process (FR-029, C-3). It closes a window only during the restore that runs
at sign-in, only where the user has turned FR-064 on and only a window the profile being restored
does not name. Apply minimises whatever the setting says. The two are told apart by which door the
restore came through: `RestoreDefault` is reached only from the sign-in start, `Restore` only from
Apply.

Closing was forbidden outright until the owner asked for it back. NordVPN and GameGlass survive their
windows closing; Postal Gambit is ended by it (A-3 in `REQUIREMENTS.md`). Nothing about a window
says which kind it is, so an agent that decided by itself to tidy a desktop by closing windows would
be quitting applications and taking whatever was unsaved in them. What changed is who decides: the setting is off until the user
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
| Step log | `%LOCALAPPDATA%\ScreenState\Log.txt`; each run cuts it down to the 10 most recent restores as it starts (NFR-OBS-001), counting the step `application.RestoreBegins` opens |
| Installed files | `%LOCALAPPDATA%\Programs\ScreenState\`, the agent, its licence and a copy of setup as `uninstall.exe` |
| Settings | `%LOCALAPPDATA%\ScreenState\settings.json`, the update setting, the skipped version, what a restore does with the windows a profile does not name and the ceiling, written as a Go duration such as `20m0s` |
| Webview cache | `%LOCALAPPDATA%\ScreenState\webview`, pinned there so an uninstall knows to look |
| Apps list entry | `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\ScreenState` |
| Sign-in entry | `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, value `ScreenState` (FR-046) |
| Shortcuts | the user's own Start Menu Programs folder and Desktop |

**A window title is never written down** (NFR-PRIV-001). The desktop reads each window's title
into `Window.Description`, which names a window the capture review could not read and nothing
else. A report names every window by its application, since the report of a sign-in restore is
written into the log; a structural rule cannot see a string flow, so two tests in
`restore_privacy_test.go` plant a revealing title and fail on any word that carries it.

Everything the product writes lives under the signed-in user's own directories and registry keys
(C-2, DATA-001). There
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
does not understand is left exactly as it is, kept out of the listing, with the reason stated in the
log on every run (DATA-003) and under the manager's profile list (NFR-REL-002). So is a file that
cannot be read or is not a profile at all. The store answers them through `ProfileStore.Unreadable`
and `ManagerService.Unreadable` passes them to the page; nothing ever writes to such a file.

## The manager window

The manager is the agent's one Wails window, which is why the agent is built by `wails build` rather
than by `go build`. Wails owns the main thread; the notification area icon keeps a locked
thread of its own, because a window belongs to the thread that made it. They meet at one point: the
tray is handed a callback and asks for the manager rather than opening anything itself.

**The window is a client of the use cases.** `app.go` holds the window plumbing and the shapes
crossing the boundary; `about.go` answers About and the licence text. `app_profiles.go` holds most
of the bound methods, each a call into one of the services: `ManagerService`, `CaptureService`,
`RestoreService`, `TrayService` for Apply and `UpdateService` for the update check. The only other
calls are into `internal/ui`: `TakeWindowFocus` for the keyboard and `RefreshTray` after an Apply.
No rule about what a profile means lives in any of the three files, so none of them can be got
wrong there without a test in the application layer failing first. The composition-root test
proves it: `main.go` is still the only file that knows both the application layer and the Windows
layer.

**The page is a script per subject, not one file.** `manager.js` holds what the panels share, the
buttons that belong to the window and the start of a run; beside it sit the interrupting surfaces
(`manager-dialogs.js`), the profile list and everything done to a profile (`manager-profiles.js`),
the capture review with the restore report (`manager-capture.js`), the settings dialog
(`manager-settings.js`), everything about the product rather than the desktop
(`manager-help.js`) and the update check (`manager-updates.js`), with the guide's words apart
again in `guide.js`. `shell.js` is the furniture
shared with the setup page and `autoscroll.js` the gentle self-reading scroll. They are plain
scripts sharing one scope, so `index.html` loads the shared furniture and the guide first, then
`manager.js` ahead of the other manager scripts; a structural test holds the tags and the files to
each other, though not their order.

**The stylesheet is one sheet cut into four in source order.** `manager.css` holds the frame and
the lists, `manager-controls.css` the fields, the marks and the rail, `manager-dialog.css` the
dialog and the Help menu, `manager-sheets.css` About, the licence, the guide and the progress bar.
The cuts fall on the section banners and nowhere else, so the four joined in the order `index.html`
loads them were byte for byte the single file they came from: the cascade could not change. They
were not cut by which panel owns a rule, since shared rules belong to no panel and a different load
order is a different cascade that nothing here would test. The same structural test holds every
stylesheet to a tag.

**The window asks how far a restore has got; nothing pushes it.** The Applying panel carries the same
bar the setup program shows, filled from the entries the restore has satisfied out of the entries the
profile holds (FR-065). The restore keeps that reading in an atomic beside the report rather than in
it, since the report is being written by the restore while the window wants to read it. The page asks
twice a second while the panel is up and stops when the work ends: a push would need a route out
through every layer between the desktop and the webview, while one reading half a second old is
harmless. The report then goes up over the profile list rather than over the Applying panel, so
closing it leaves the user somewhere they can act (FR-066).

**The main screen is three parts: the profiles, what the selected one arranges and the buttons.**
The profile list runs down the left; the selected profile's applications fill the middle, where a
path can wrap and be read whole (FR-068); the buttons of whatever panel is showing run down a rail on
the right, with the donation button at its foot. The settings are a dialog behind the button in the
bar (EIR-002), set apart from the theme and Help by a rule. The layout got here in two steps: the
applications and the settings first shared one column, which left a path cut off after a few words
and a list one row tall; the applications then went to a dialog; once the settings had a dialog of
their own, the middle was free for the applications to come back with the room they lacked. A
reading of a profile that arrives after the user has pressed another is dropped rather than drawn.

**A sign-in start opens no window.** The setup program writes the sign-in entry with a flag that
keeps it shut, so the agent waits in the notification area (FR-046, FR-048); launched by hand it
opens the manager and arranges nothing (FR-038), which is what double-clicking a shortcut means. An entry written without that
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

## The splash

**A restore says it is arranging the desktop and when it is done** (FR-078). The restore service
tells a `Splash` port as a restore begins and again as it ends; the words, including the shortfall
("1 application did not start"), are settled in `restore_splash.go` and tested there. A restore a
newer request stood down says nothing more, so "ready" never flashes up mid-restore; it says so
before closing the channel the newer restore waits on, so the two cannot arrive out of order.

`internal/ui` draws it natively, one window per display on a thread of its own, like the tray: a
Wails application has one window and the manager is it. Each splash is a topmost tool window that
never activates, so it cannot take the keyboard (FR-074) and is not a candidate window any capture,
FR-064 or FR-075 acts on. It closes on events and never on a timer: a click on any splash closes
them all; once the restore has ended, so does the user's next key press or mouse click anywhere,
which raw input with the sink flag delivers to a window that does not hold the keyboard while the
press still reaches its owner.

**Bringing the manager up takes the splash down** (FR-078). The tray, a second launch and an update
offer all bring the window up through `App.bringUp`, which tells the splash to go before the window
is shown: a topmost splash would otherwise sit over the window the user asked for until their next
key press. The message is posted to the splash's own thread; where its windows are still being made
the request is remembered and acted on once they exist. The restore carries on either way. Once a
splash has said ready and is listening for the next press, the log says so; where it could not
listen, the log says that instead.

A click on a splash is not the user taking over (FR-079), so it closes the splash and the restore
carries on as FR-078 says. The restore's own mouse hook hears every click; `takesOver` in
`win32/takeover.go` leaves out a press whose top-level window under the pointer has the splash's
class, which both packages read from `product.SplashClass`. A key press always counts.

Its colours come from the embedded `assets/theme.css`, the palette's one home, rather than being
written again in Go; its logo is reduced from the embedded master artwork to each display's size by
an area average, never enlarged. `readLook` in `splash.go` reads the palette and the artwork once at
start for both surfaces the agent draws, this one and the tray's badge. A palette that cannot be
read shows no splash; artwork that cannot be read shows the words alone; the log says which.

**The tray icon asks for attention the same way** (FR-045). While `NeedsAttention` says the last
restore left something outstanding, the tray shows its icon with a badge in the bottom-right
corner: a disc in the palette's `--danger` ringed in its `--panel`, so it stands clear of the mark
and of any taskbar. There is no second piece of artwork. The badge is drawn in code
(`internal/ui/badge.go`, portable and tested) over the same master artwork, reduced to the small
icon's size, in whichever theme is in force; each palette's icon is built the first time it is
wanted and freed when the tray closes. Where the palette or the artwork cannot be read, the icon
stays plain. After every restore the tray reads its tooltip and its badge again: the manager's
Apply and the sign-in restore call `ui.RefreshTray`; the tray's own Apply posts the same refresh.

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

Help is a drop-down under its button rather than a menu bar, because this window has no menu bar
and a made-up one would be furniture nobody expects. It offers the guide, the report of the last
restore, About, the licence and the update check.

The guide's words live in `frontend/dist/guide.js`, apart from the panel that draws them, so the panel
stays a renderer and the words stay one readable document. Every entry carries the REAL control this
window draws: the image files the window loads, plus the window's own classes for the drawn ones, so
the guide cannot come to show something the window does not. A guide showing anything else is worse
than no guide. It names the furniture first, then states the rules the window cannot say for itself:
what is kept locally, that no program is ever ended and a window is closed only where the setting
says so, the single network call and what cannot be undone.

## The setup program

A second program in the same module, at `installer/`. It is a Wails application like the agent's
manager and wears the same palette, sampled from the agent's own artwork. It carries the built
agent as an embedded zip, so one downloaded file is the whole distribution.

**The install policy is infrastructure, not interface.** `internal/infrastructure/setup` owns the
paths, the fenced payload extraction, the version comparison, the registry writes, the shortcut
handling and the process work. `installer/app.go` is a facade over it and owns no install logic at all,
so what an install does can be read in one place and exercised without a window. The portable half
carries unit tests; the Windows half sits behind a build tag with no-op stubs beside it, so the package
still builds and vets on a machine that is not Windows.

**One reading of the machine decides everything.** `DetectState` on the facade looks once and
answers the mode, the version relation and what is already true of the shortcuts and the sign-in
entry. The screen, its heading, the options on it and the buttons under it all come from that one
answer, which is what stops those four drifting apart.

| Route | When |
|---|---|
| Install | nothing recorded in the Apps list |
| Update | the recorded version is older than the one setup carries |
| Go back a version | the recorded version is newer |
| Manage | the versions match, so there is nothing to install |

Removal is a screen reachable from every other one, so cancelling it returns to whatever was due
behind it. The one exception is a start with `-uninstall`, which is how the Apps list asks for it:
that goes straight to removal.

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
| Build off Windows | `go build ./...` for Linux, so the portable layers stay portable |
| `go test -race` | the whole suite, with the race detector and so with cgo on |
| `go test` | the whole suite again with cgo off, as the product ships, with coverage |
| Every package ran | a package owning tests that ran none of them fails the gate, since a quarantined test binary reports ok |
| Structural suite | the invariants above, each proved by a planted violation |
| Coverage floor | every function in `internal/domain` and in `internal/application` exercised |

The floor is scoped to the two layers a machine can exercise with no filesystem, no clock and no
desktop. Anything short there is a decision nobody made. It counts functions, not statements: a
function counts once a test reaches any statement in it. Infrastructure sits deliberately outside
it: the Windows half needs a real desktop; gating it would mean either a number that means nothing
or tests that assert what happened to be on screen.

Measured statement coverage on 2026-09-21: domain 100%, application 97.2%, clock 100%, store 93.8%,
instance 90.9%, settings 89.1%, runlog 76.7%, win32 41.7%, setup 33.6%, ui 26.8%. The
shortfalls outside the floor are IO and platform failures that would need the disk or the window
manager to fail mid-call, plus the Win32 calls themselves; they are not padded with tests that
assert nothing.

The structural suite also holds both pages' boundaries, which no compiler sees: a page may not write
the product's name or its tagline down, every `state.` field it reads must be a json tag the program
actually sends and every call it makes must be a method the program binds.

Four integration tests in `win32` read the real machine and assert only what must hold anywhere.
Three read the desktop and skip where there is none: the displays and windows, whether each
application on screen is found running and the applications running with every window hidden, none
of them part of Windows. The fourth reads the flash count and caret blink the FR-080 wait is worked
out from. None moves a window or starts an application, since either would disturb the desktop of
whoever ran the suite. A fifth, `TestMaximisingDoesNotActivate`, does move windows (only two of its
own), so it runs only when `SCREENSTATE_DESKTOP_PROBE` is set; the gate skips it.

## Design decisions

| Decision | Why |
|---|---|
| A profile is an end state, never a sequence of steps | Each entry converges on its own, so the restore never waits for its slowest application |
| Placement is per entry, not per profile | Holding every placement until the whole profile was ready required a definition of finished that Windows cannot supply |
| A restore terminates nothing and closes a window only where the user turned closing on | Closing a window ends some applications outright and takes whatever was unsaved in them, so the user decides |
| A newer restore replaces a running one | A restore is a statement of what the desktop should look like now, so the newest statement is the one that is true |
| Undo nothing when replacing | Putting windows back moves them twice to reach the same end |
| The normal rectangle is recorded, even when maximised | It is what decides which display maximising puts the window on |
| A display is named by its device instance path | Every number Windows offers was measured disagreeing with the others |
| An application is named by what starts it, with what survives its updates kept beside it | The path starts it as a double-click does; a Store package's path carries its version, so the model id kept beside it recognises and starts it after an update; a window class carries a GUID that changes every reboot |
| Applications running with no window shown are offered unticked | Helpers another application starts are among them; none of those should be started directly |
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

**Moving a window and starting an application are proved by use, not by test.** No test calls
either, because both would disturb the desktop of whoever ran the suite. Both are run at every
sign-in on the reference machine, where the log records each application started and each window
placed.

**The update check has never been seen to offer a real update.** Its rules are covered end to end
against a stand-in; every outcome the window can show was driven in a browser. The feed now has a
published release to answer with. An offer needs one newer than the version running, so the happy
path is proved only in the shape the adapter promises to produce.

**The manager is proved by use rather than by test.** It has been run for real on the reference
machine: the log records captures that read the desktop, a capture cancelled without writing
anything, a profile written, a profile deleted, the default marking settled, restores at start and
the quit from both the manager and the tray. The panels themselves are still driven in a browser
against a stand-in for the agent, which settles the layout, the palette and the wiring and settles
nothing else. What neither has settled: real keyboard focus and the ring, the second-launch message,
the tray opening a named panel, the donate link and the update offer on screen. Those are read off a
run, not off a suite, so they are checked by the list in TESTING.md. So are the ones added most
recently, whose rules the suite holds where they have any: stopping a restore, the tray's badge,
offering applications running with no window, matching a packaged application after an update, the
ceiling set in the settings dialog, a start by hand arranging nothing, the manager taking the splash
down and the three-part main screen.

**The setup program has installed and nothing else.** It has installed on the reference machine
many times, each over an existing install of the same version: the files are in
`%LOCALAPPDATA%\Programs\ScreenState`, the Apps list entry carries its uninstall and modify
commands, the sign-in entry names the installed file with the flag that keeps the window shut and
both shortcuts are where they belong. So the payload extraction, the registry writes and the
shortcuts are proved for that one path. Everything else is written and not proven: an update to a
newer version, going back to an older one, repair, uninstall, the scheduled removal and the cleanup
of an install under a name the product used to carry. Its screens are still driven in a browser
against a stand-in, which settles the layout and the wiring and settles nothing else.

## Where the code falls short of the specification

Found by reading the source against every requirement during the documentation pass for the first
release. Nothing found that way is left open. Twelve items were on this list and are now built:
cancelling a restore (FR-049), matching a packaged application's window after an update (FR-071),
marking the tray icon after an incomplete restore (FR-045), capturing an application with only a
hidden window (FR-005), counting the rebuilt buttons (FR-075), a click on the splash (FR-078), log
retention (NFR-OBS-001), setting the ceiling (NFR-PERF-003), holding the page files to the module
size (NFR-MAINT-003), keeping window titles out of the log (NFR-PRIV-001), naming an unreadable
profile in the manager (NFR-REL-002, DATA-003) and maximising a window without activating it
(FR-074). A new shortfall found by reading the source against a requirement belongs here, with the
requirement it falls short of.

What remains is proving the rest: see the known limits above.
