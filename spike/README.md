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

### OQ-1, display identity: answered

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

Re-read after a reboot on 2026-09-19: every monitor id is unchanged; each still
sits on the same GDI name. A profile can therefore store a display by its
monitor id. Assumption A-1 holds.

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

### OQ-6, showing a hidden window: answered; the answer kills FR-036

Measured against NordVPN's own window, which the application had hidden rather
than destroyed. Showing it from outside worked in the narrow sense: the window
became visible. It was useless: the application was no longer drawing it, so it
appeared as an empty black frame.

So a window a tray application has hidden cannot be brought back by showing it.
FR-036 as drafted is withdrawn. What remains to test is asking the application
to show its own window, by running a second copy of it, which is what a user
does from the tray.

### OQ-4, closing a window: answered for NordVPN and GameGlass

Measured after a reboot, from the owner closing both windows by hand as he
normally does. Both windows still exist and are hidden, with their rectangles
preserved; both processes are alive: NordVPN pid 23672, GameGlass Hub pid
32248. Closing leaves the application running. Assumption A-3 holds for both.
Postal Gambit was not running, so it is untested.

### A window class is not an identity

NordVPN's main window class was
`HwndWrapper[NordVPNApp;Hosted Main;3bcafb52-...]` before the reboot and
`HwndWrapper[NordVPNApp;Hosted Main;0e6920e5-...]` after it. The class carries
a GUID that changes each session, so a profile cannot match a window by its
class. The process image path can.

### The earlier close attempt, which was wrong

NordVPN ignored both the plain close message and the system menu close. That is
not a verdict: the window was hidden and partly torn down at the time, which is
not the state it is in at sign-in. The measurement has to be taken against a
window the application has just shown by itself.

This is a lesson about the spike as much as the product. A measurement taken
against the wrong state answers a question nobody asked.

### OQ-5, startup timings: bounded, not finished

Measured from a sign-in run on 2026-09-19 that began 2 minutes 9 seconds after
boot, because a Startup entry only fires once the shell is up:

- The last window to appear was Claude, 5 minutes 34 seconds after boot.
- The longest gap between appearances was 3 minutes 25 seconds, before Claude.
- No window moved during the run, so OQ-7 has no evidence either way.

From those two numbers the ceiling moved from 5 minutes to 15 and the quiet
period from 15 seconds to 60.

### A defect in this spike, found the hard way

The first sign-in run wrote its events to the console and the log through one
writer that took the console first. When the console went away, its write
failed and the log write never happened, so the summary was lost and any event
after that point was lost in silence. The log is now written first and the
console is allowed to fail. A measurement tool that depends on someone
watching it is not a measurement tool.

### OQ-4, closing a window: answered for all three; they do not agree

A second sign-in run on 2026-09-20 closed all three tray windows by hand at
2m21, 2m22 and 2m24. Their processes were read back 43 minutes later:

| Application | Window closed at | Process 43 minutes later | Closing it |
|---|---|---|---|
| NordVPN | 2m21 | pid 25156 alive, no window | leaves it running |
| GameGlass Hub | 2m22 | 13 processes alive, none with a window | leaves it running |
| Postal Gambit | 2m24 | no process at all | ends the application |

So assumption A-3 does not hold in general. It holds for the two applications
that live in the tray; Postal Gambit is an ordinary windowed application and
closing its window is quitting it. A profile that closes a window is therefore
doing one of two quite different things depending on what it is closing; it
cannot tell which from the window alone.

GameGlass also shows why a live process is a weak signal: it runs thirteen of
them, the same shape as the Notepad finding above. What matters is whether a
window can be brought back, never whether something is still running.

### OQ-5, startup timings: a better bound from a run that was cut short

A hidden run started 31 seconds after boot, against 2 minutes 9 seconds for the
first attempt, so a scheduled task firing at logon is the right harness.

Measured from that run:

- 15 windows appeared in the first 7 minutes 12 seconds.
- The last one observed was Chrome at 7m12.
- The largest gap between two appearances was 2 minutes 1 second.
- Claude, the slowest starter in the first run at 5m34, appeared at 4m5s here.

The ceiling is not established by this run, because the run did not finish: see
the defect below. What it does establish is a floor of 7 minutes 12 seconds of
appearances, which is already longer than the 5 minutes the specification
originally assumed.

### OQ-7, do windows move after they appear: CLOSED; it was us

Three windows moved after appearing: Discord at 68s to 128s, PigeonPost at 146s
to 148s, Stellody at 216s to 228s. Each burst had the shape of a hand drag,
several moves a few hundred milliseconds apart through intermediate
coordinates, ending in a snap to maximised. The owner confirmed he was dragging
those windows between displays at the time, so the run holds no evidence about
what applications do by themselves.

It will not be measured again, for two reasons. The test cannot be run: it needs
25 minutes during which nobody touches the machine, which is 25 minutes of the
owner's own working day. More to the point it settles nothing, since FR-033
re-applies a placement once when the window no longer matches; FR-034 gives up
rather than fight. Those hold whether or not applications move their own
windows.

### The summary was lost again, in a new way

The run was told to watch for 25 minutes. It stopped writing at 7 minutes 12
seconds and never wrote its Summary section; the process was gone by the
time anyone looked. The binary was untouched on disk at its original size, no
crash was recorded in the Application log and the scheduled task's limit is an
hour, so none of the obvious explanations fits. **Why it stopped is not known.**

The response is not to find out. It is that the question should never have
depended on the process surviving. Every number in the two sections above was
computed from the event log after the fact, without the summary, because the
log holds every event and the summary is only a view over it.

So the fix is to make that view a command rather than a thing written once at
the end. A run that dies still answers the question; the tool stops depending
on its own survival. That is the same lesson as the first defect,
learned again at a different level.

### OQ-5 is closed too, by removing what needed it

The owner's ruling: only the target state matters, so who cares what the desktop
looked like at boot. The specification agreed with him in its own words, since
NFR-PERF-002 said the quiet period "only catches stragglers the profile does not
name". A window the profile does not name is one the product will never
place. Waiting for it bought nothing.

FR-020 now settles on the profile's own entries alone: every application it
records as running is running; every one with a placement has a window. The
quiet period is withdrawn, the ceiling is a policy choice rather than a
measurement; the sign-in timing study is not needed at all.

### OQ-6, showing a hidden window: ANSWERED, by asking rather than telling

Measured on 2026-09-20 against NordVPN, running with its window hidden at hwnd
0x20754, pid 25156, rect x=1269 y=345 w=902 h=702.

Running NordVPN again started pid 17440. One second later the SAME window,
hwnd 0x20754 belonging to pid 25156, became visible at the same rectangle;
pid 17440 then exited by itself. The window passes the candidate rule. Because the
application showed it rather than being overruled, it is drawing it: no empty
frame this time.

So A-4 is true in one form and false in the other. A hidden window cannot be
made useful by acting on it from outside, which is what killed the first
mechanism. The application can be asked to show it, by running it again, which
is exactly what a user does from the tray. FR-036 stands with its mechanism
replaced, now stated as FR-056.

This also forced FR-025 open. It said the agent shall not launch an application
that is already running, which forbids the only measured way to recover a
hidden window. It now carries that exception, justified by the measurement: on a single-instance application the second copy signals the
first and exits rather than becoming a second instance.

### Appendix B is finished

Every question this spike existed for is closed: OQ-1, OQ-2, OQ-3 and OQ-6
answered, OQ-4 answered and found not to hold in general, OQ-5 and OQ-7
dissolved by defining settling per entry. Nothing in the specification now
waits on a measurement.

By its own first paragraph this code can be deleted. It is kept only until the
owner says otherwise; everything it established lives in this file and in
REQUIREMENTS.md rather than in the code.
