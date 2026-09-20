# ScreenState
ScreenState: window layout profiles for Windows

`REQUIREMENTS.md` is the specification. `ARCHITECTURE.md` describes how the product is built and
which test enforces each invariant.

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
