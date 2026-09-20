# ScreenState
ScreenState: window layout profiles for Windows

`REQUIREMENTS.md` is the specification. `ARCHITECTURE.md` describes how the product is built and
which test enforces each invariant. [DEVELOPMENT.md](DEVELOPMENT.md) is how to build it from source;
[TESTING.md](TESTING.md) is how it is tested, including what a first test run needs doing to the
machine.

## Building

```
./test.ps1     every check: formatting, vet, staticcheck, the suite twice, the coverage floor
./build.ps1    the agent and the setup program; the gate runs first and cannot be skipped
```

`./build.ps1 -SkipInstaller` stops after the agent. The setup program needs the Wails command line
tool; the agent does not.

Outputs:

| File | What it is |
|---|---|
| `build/bin/ScreenState.exe` | the agent |
| `dist-installer/ScreenStateSetup.exe` | the setup program, carrying the agent inside it |

The icons are generated from `assets/application-icon.png` by `python tools/genicons.py` and committed,
so a clone needs neither Python nor Pillow to build anything. Run it when the artwork changes.

## Installing

Run `ScreenStateSetup.exe`. Everything it writes is per user, so Windows never asks for administrator
rights: the files go under `%LOCALAPPDATA%\Programs`, the Apps list entry and the sign-in entry under
`HKCU`. It covers install, update, going back a version, repair, reinstall and uninstall. It registers
itself with Windows, so Modify and Repair in the Apps list reopen it rather than sending you back to
the download.

Your captured profiles live somewhere else, so removing the product leaves them alone unless you tick
the box that says otherwise.

## Using it

The agent waits in the notification area. Click its icon or launch it again from the Start Menu
or the desktop. Either way the manager opens with the profiles down the left, what the selected one arranges down
the right and the sign-in setting under them. Right-click the icon instead for the menu, which
applies a profile, starts a capture, opens the report of the last restore or quits.

Capture the desktop to make a profile. Nothing is saved until you name it and confirm; anything you
untick is left out. Marking a profile as the default is what makes it the one applied after you sign
in; marking none means nothing is applied.

Started by Windows at sign-in it opens no window at all, which is the point of it: it puts your
windows back and waits.
