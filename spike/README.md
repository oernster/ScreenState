# ScreenState measurement spike

Throwaway code. It exists to settle the open questions in the specification's
appendix B before any of them is written into the product as an assumption. It
is not the application, it shares no code with the application and it will be
deleted once every question below is closed.

Build:

```
go build -o screenstate-spike.exe .
```

Every command prints to the console and writes the same text to a log file
beside the executable, named `spike-<command>-<timestamp>.log`.

| Command | Question it settles |
|---|---|
| `screenstate-spike displays` | OQ-1, what identifies a display across a reboot |
| `screenstate-spike windows [--all]` | OQ-2, what identifies an application across its updates |
| `screenstate-spike move <hwnd> <index>` | OQ-3, can a window be placed and maximised on a chosen display |
| `screenstate-spike close <hwnd>` | OQ-4, does closing a window leave the application running |
| `screenstate-spike show <hwnd>` | OQ-6, can a hidden window be shown from outside |
| `screenstate-spike launch <cmd>` | OQ-6, does running a second copy surface the first |
| `screenstate-spike watch [minutes]` | OQ-5 and OQ-7, the real startup timings |

`hwnd` accepts either the hexadecimal form the `windows` command prints
(`0x1074C`) or a plain decimal number. `index` is the number in brackets that
the `displays` command prints.

The spike never terminates a process. `close` asks a window to close, which is
what a user does by hand.

---

## Results so far

Measured on the reference machine on 2026-09-19. Anything not recorded here has
not been measured.

### OQ-1, display identity: half answered

The four displays report these monitor identifiers:

| Index | GDI name | Settings number | Monitor id | DPI |
|---|---|---|---|---|
| 0 | `\\.\DISPLAY1` | 1, top, primary | `DISPLAY#GSM784F#5&14514d51&0&UID4352` | 96 |
| 1 | `\\.\DISPLAY2` | 3, centre | `DISPLAY#HSJB30F#5&14514d51&0&UID4357` | 240 |
| 2 | `\\.\DISPLAY3` | 4, left | `DISPLAY#HSJ1340#5&14514d51&0&UID4356` | 240 |
| 3 | `\\.\DISPLAY4` | 2, right | `DISPLAY#HSJ1340#5&14514d51&0&UID4354` | 240 |

Two findings:

- The Settings number, the GDI name and the physical display do not line up.
  Settings 4 is `\\.\DISPLAY3`. Neither number can identify a display.
- The left and the right display share a model code, `HSJ1340`, so the model
  alone cannot tell them apart. Their UID differs, `UID4356` against `UID4354`,
  which does tell them apart.

What remains: whether the monitor id holds after a reboot. Run `displays`
again after the next restart and compare.

### OQ-2, application identity: answered

| Application | Image path | Identity that survives its updates |
|---|---|---|
| Claude | `WindowsApps\Claude_2.2553.1.0_x64__pzs8sxrjxfjjc\app\claude.exe` | `Claude_pzs8sxrjxfjjc!Claude`, carries no version |
| Discord | `Discord\app-1.0.9258\Discord.exe` | `Discord\Update.exe --processStart Discord.exe`, which does not move |
| Stellody | `Programs\Stellody\Stellody.exe` | the path, which carries no version |
| PigeonPost | `Programs\PigeonPost\PigeonPost.exe` | the path, which carries no version |

So a profile stores a packaged application by its application user model id, an
application installed under a versioned directory by its updater command; every
other application by its path.

The candidate rule proposed in FR-012, a visible window that is not cloaked,
not owned by another window, not a tool window and not without a title, reduced
523 enumerated windows to the 8 a user would call windows.

### OQ-3, placing a window on a chosen display: answered

Measured against a Notepad window launched for the purpose, moved to each of
the four displays in turn, from a process holding no administrator rights:

| Target | Landed on | State | Rectangle |
|---|---|---|---|
| index 0, `\\.\DISPLAY1`, 96 dpi | `\\.\DISPLAY1` | maximised | x=-8 y=-8 w=3456 h=1408 |
| index 1, `\\.\DISPLAY2`, 240 dpi | `\\.\DISPLAY2` | maximised | x=-239 y=1424 w=3872 h=2312 |
| index 2, `\\.\DISPLAY3`, 240 dpi | `\\.\DISPLAY3` | maximised | x=-4079 y=1407 w=3872 h=2312 |
| index 3, `\\.\DISPLAY4`, 240 dpi | `\\.\DISPLAY4` | maximised | x=3601 y=1407 w=3872 h=2312 |

Every move landed where it was told, including the move from the 96 dpi display
to a 240 dpi one. Assumption A-5 holds. A maximised window overhangs its
display by the border width, which is why each rectangle is 16 pixels wider
than the display.

The sequence that works: restore the window, set its rectangle to the target
display's work area, then maximise it.

### A finding that was not being looked for

Notepad runs several windows in one process. The window launched for this
measurement joined an existing Notepad process that already held another
window. Two consequences for the product:

- A profile entry cannot assume one window per application, which is what
  FR-037 exists for.
- Whether a process is still running does not tell you whether closing one of
  its windows was harmless. The `close` command reports the process as running
  because another window of it was open. For NordVPN, GameGlass and Postal
  Gambit the report is still meaningful, since each holds one window.

### OQ-4 to OQ-7: not yet measured

- **OQ-4**: run `close` against NordVPN, GameGlass and Postal Gambit.
- **OQ-4**: run `close` against NordVPN, GameGlass and Postal Gambit.
- **OQ-5 and OQ-7**: run `watch 10` immediately after signing in, twice, on
  separate reboots.
- **OQ-6**: run `show` against a tray-only application's window if it has one,
  then `launch` its executable while it is already running.
