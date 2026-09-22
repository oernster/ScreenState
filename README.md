# ScreenState

Window layout profiles for Windows.

> **Commercial licences available.** ScreenState is free and open source under the GNU General
> Public License, version 3. If those terms do not suit what you are building, such as a
> closed-source product, a commercial licence can be bought from me separately. It covers my own
> code; third-party libraries keep their own licences. See
> [commercial licensing](https://ernster.dev/commercial-licensing.html).

ScreenState puts your desktop back after you sign in. Capture the desktop once as a named profile;
from then on, signing in starts the applications the profile records as running and puts each of
their windows on the display it belongs on, at the size and in the state it was captured in.

Site: [oernster.github.io/ScreenState](https://oernster.github.io/ScreenState/)

## Who it is for

People who arrange the same windows across the same displays every time they sign in, especially
across several monitors; they would rather not.

It is not for arranging windows as you work: it acts when you sign in or press Apply, never
continuously. Nor is it for bringing back what is inside an application, nor for anything but
Windows.

## What it does

- **Captures the desktop as a profile.** It lists every application with a window on screen and
  how many windows it has, recording where each one sits; you untick what you do not want and name
  the profile. Only what is on screen is
  captured: an application with no window shown has nothing to arrange. Nothing is written until
  you confirm.
- **Restores the default profile at sign-in**, waiting in the notification area without opening a
  window. It starts what is missing and places each window as soon as it appears, without waiting
  for the slowest application. A message on every display says the desktop is being prepared, then
  that it is ready. How long a restore waits for windows that have not appeared is yours to set in
  Settings.
- **Applies any profile on demand** from the tray menu or the manager. A restore started from the
  manager can be stopped with Stop the restore on its Applying panel; every window already placed
  stays where it is and the report says the restore was cancelled.
- **Knows displays apart** by the identity Windows gives each screen, so two monitors of the same
  model are never confused. A display that is gone sends its windows to the primary one.
- **Names applications by what survives their updates**: the path, the updater that does not move
  or (for a packaged application whose path moves with every version) its model id kept beside the
  path. The model id is what still starts it and what still recognises its window once an update
  has moved it to a new path.
- **Speaks in the names you use.** The manager lists each application by its name, with how its
  window is shown, on which display and at what size beneath; displays are named by where they sit
  (top, left, centre, right) in the manager and in the report. Nothing recorded is hidden: the full
  path, each rectangle and each display's identity are a hover away; the log pairs every position
  with its display's identity.
- **Leaves the keyboard and the taskbar alone.** Nothing it starts or places takes the keyboard
  from the window you are typing in; no taskbar button is left lit, red or missing its icon.
- **Puts away the windows the profile does not name**, by minimising them; at sign-in it can close
  them instead, if you turn that on.
- **Reports what it could not do**, naming each application and the reason; the report is a click
  away in the tray menu and in the manager's Help. The tray icon's tooltip says what the last
  restore did and is read again after every restore, wherever it was started. While the last
  restore left something outstanding, the icon carries a red badge. A profile file that cannot be
  read is named under the manager's list of profiles rather than left out in silence.

## What it does not do

- **It never ends another program.** The one process anything here ends is its own agent, which
  the setup program stops, when you tell it to, before replacing or removing its files. It closes a window only where you have
  turned that on, only at sign-in and only for a window the profile being restored does not name.
- **It does not restore what is inside an application**: browser tabs, open documents and the
  folder an Explorer window shows are the application's business. Nor stacking order, Snap groups
  or virtual desktops.
- **It does not encrypt its profiles.** They are plain files in your own folder, holding
  application paths, window positions, display identities and profile names.
- **It sends nothing about you anywhere.** Its one network connection asks whether a newer version
  has been released, carries no identifier and can be turned off, after which it connects to
  nothing at all.
- **It never asks for administrator rights**, to install or to run; so it cannot act on an
  application running with them.

Windows 11 is the supported target. Windows 10 will very likely work: the one thing it asks of
Windows 11 alone is rounded corners on the sign-in message, which older Windows ignores. It is
untested.

## Stack

| Part | What |
|---|---|
| Language | Go, no cgo in anything shipped |
| Windows | Win32 through `golang.org/x/sys/windows`, with WinEvent, shell and low-level input hooks |
| Manager window | Wails v2 hosting a plain HTML, CSS and JavaScript page in WebView2 |
| Setup program | a second Wails program carrying the agent inside it |
| Storage | one JSON file per profile, written atomically |
| Tests | Go's `testing`, hand-written fakes, a structural suite |

## Installing

Download `ScreenStateSetup.exe` from the
[latest release](https://github.com/oernster/ScreenState/releases/latest) and run it. Everything it
writes is per user, so Windows never asks for administrator rights: the files go under
`%LOCALAPPDATA%\Programs\ScreenState`, the Apps list entry and the sign-in entry under `HKCU`. The
same program installs, updates, goes back a version, repairs, reinstalls and uninstalls. It registers itself with
Windows, so Modify and Repair in the Apps list reopen it rather than sending you back to the
download.

Your captured profiles live somewhere else, so removing the product leaves them alone unless you
tick the box that says otherwise.

## Using it

The agent waits in the notification area. Click its icon or launch it again from the Start Menu or
its desktop shortcut and the manager opens: the profiles down the left, what the selected profile
arranges beside them and its buttons down the right, with the settings behind the gear at the top.
Starting it by hand arranges nothing; only a sign-in does that by itself, while Apply does it whenever you ask.
Right-click the icon instead for the menu, which applies a profile, opens the manager, starts a
capture, opens the report of the last restore or quits.

Capture the desktop to make a profile. Marking a profile as the default is what makes it the one
applied after you sign in; while there is only one profile, it is the default. The manager also
renames and deletes profiles and takes an application out of one.

The settings say whether it starts when you sign in, whether the sign-in restore closes the windows
a profile does not name rather than minimising them, how long a restore waits for windows and
whether it checks for updates.

Started by Windows at sign-in it opens no window at all, which is the point of it: it puts your
windows back and waits.

## Testing

```powershell
./test.ps1
```

Formatting, vet, staticcheck, a build for a platform that is not Windows, the whole suite twice
(with the race detector, then as the product ships) and the coverage floor. Trust the exit code.
[TESTING.md](TESTING.md) has what each part proves and what only a real desktop can settle,
including what a first test run needs doing to the machine.

## Building

```powershell
./build.ps1                  # the agent and the setup program; the gate runs first
./build.ps1 -SkipInstaller   # the agent only
```

| File | What it is |
|---|---|
| `build/bin/ScreenState.exe` | the agent |
| `dist-installer/ScreenStateSetup.exe` | the setup program, carrying the agent inside it |

Both programs are built with the Wails command line tool. [DEVELOPMENT.md](DEVELOPMENT.md) lists
every tool a build needs and what the build does, in order.

## Documents

- [REQUIREMENTS.md](REQUIREMENTS.md): the specification, with the measurements each requirement rests on.
- [ARCHITECTURE.md](ARCHITECTURE.md): how it is built and which test enforces each invariant.
- [DEVELOPMENT.md](DEVELOPMENT.md): building from source and cutting a release.
- [TESTING.md](TESTING.md): the gate, what the tests prove and the checks done by hand.

## Supporting the project

ScreenState is free and stays free: there is no paid tier, no licence key and no feature held back
behind a donation. The same link sits at the foot of the manager's right-hand rail; pressing it
there hands the address to your browser, so the application itself still makes no connection beyond the update
check.

<a href="https://www.paypal.com/ncp/payment/6FMTGJYFJXFTE"><img src="docs/donate.png" alt="Donate to ScreenState" width="120"></a>

## Licence

GPL-3.0; see [LICENSE](LICENSE). Commercial licences are available: see
[commercial licensing](https://ernster.dev/commercial-licensing.html).
