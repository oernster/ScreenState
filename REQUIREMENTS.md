# ScreenState: Software Requirements Specification

Status: BASELINED 2026-09-20. Every open question in appendix B is closed.

---

## 1. Introduction

### 1.1 Purpose

ScreenState restores a Windows desktop to a chosen arrangement after a reboot or
a sign-in. It records, per user, the applications that should be running and
where each of their windows belongs across the connected displays, then puts the
desktop back into that state without the user arranging it by hand.

### 1.2 Intended audience

The author, as implementer. Contributors reading the repository. Users reading
the README, which is derived from section 1.3 and section 6.

### 1.3 Scope

In scope:

- Named profiles describing the desired end state of a set of applications.
- Capture of a profile from the desktop as it stands.
- Launch of applications that a profile records as running.
- Placement of windows on a named display at a recorded size, position and
  show state.
- Automatic application of one default profile after sign-in.
- Application of any profile on demand.
- A tray presence and a profile management window.

Out of scope (each settled with the owner on 2026-09-19):

- **OOS-1** Content inside an application: browser tabs, open documents, the
  folder an Explorer window is showing.
- **OOS-2** Window z-order, Snap layout groups and virtual desktop placement.
  Reason: the owner cannot test them effectively.
- **OOS-3** Any operating system other than Windows.
- **OOS-4** Sharing or synchronising profiles between machines.
- **OOS-5** Applications running with administrator rights. ScreenState never
  requests administrator rights and reports any window it cannot act on.
- **OOS-6** Automatic profile switching when displays are connected or
  disconnected during a session.
- **OOS-7** Other user accounts. Each signed-in user has their own profiles.
- **OOS-8** Intercepting window creation. ScreenState acts on windows only after
  they exist.

### 1.4 Definitions

| Term | Meaning |
|---|---|
| Agent | The ScreenState process running in the signed-in user's session. It owns the tray icon, performs capture and performs restore. |
| Manager | The profile management window presented by the agent. |
| Setup program | The program that installs, updates, repairs and removes ScreenState. |
| Profile | A named record of the desired end state of a set of applications for one user. |
| Entry | One application within a profile, holding its application identity, whether it should be running and its placements. |
| Placement | One window's desired display, normal rectangle and show state. |
| Application identity | A value identifying an application that stays the same across updates of that application. |
| Display identity | A value identifying a physical display that stays the same across reboots and distinguishes displays of the same model. |
| Show state | One of normal, minimised or maximised. |
| Normal rectangle | The position and size a window occupies when neither minimised nor maximised. |
| Restore | Driving the desktop towards a profile's end state. |
| Ceiling | The maximum span after sign-in that the agent waits for a profile's windows to appear. |
| Default profile | The one profile marked to be applied after sign-in. |
| Report | The readable record of what a restore did, including every entry it could not satisfy. |

Requirement vocabulary: **shall** is binding. **will** describes the
environment. **must** is reserved for external law. **should** never appears in
a requirement body.

### 1.5 References

- ISO/IEC/IEEE 29148:2018, requirement quality characteristics.
- EARS (Easy Approach to Requirements Syntax), Mavin et al., RE'09.
- Repository: https://github.com/oernster/ScreenState, GPL-3.0.

---

## 2. Overall description

### 2.1 Product perspective

A new, self-contained per-user Windows application. No server, no account, no
network dependency. It reads and writes its own profile store on the local
filesystem and calls Windows to enumerate displays, enumerate windows, launch
applications and move windows.

### 2.2 User classes

| Class | Needs | May not |
|---|---|---|
| Desktop owner | Captures profiles, marks a default, applies a profile on demand, reads the report of the last restore. | Act on another user's profiles. Act on windows of applications running with administrator rights (OOS-5). |

There is one user class. ScreenState has no administrator role.

### 2.3 Operating environment

- Windows 11, 64 bit, is the supported target. ScreenState will very likely run
  on Windows 10, since it calls nothing Windows 11 introduced, though that is
  untested and therefore unsupported. Supported means tested; the owner has no
  Windows 10 machine to test on. Nothing is done to prevent it running there.
- No network access is required for any of the product's own behaviour. The one
  exception is the update check (FR-058), which the user can turn off.
- Reference machine for every measured requirement: the owner's desktop, four
  displays, one 3440x1440 primary plus three 3840x2400, mixed positions.
- No network access is required at any point.
- Installed and run per user. No administrator rights at install or at run.

### 2.4 Constraints

- **C-1** ScreenState never requests administrator rights.
- **C-2** All files it writes live under the signed-in user's profile
  directories. No machine-wide registry keys, no machine-wide files.
- **C-3** ScreenState never terminates a process it did not start.
- **C-4** ScreenState makes exactly one kind of network connection: the update
  check in FR-058, an anonymous request asking a release feed what the latest
  version is. It carries no identifier and no profile data, it can be turned
  off, plus everything else the product does works with the machine offline. No
  profile, no window layout and nothing about the desktop ever leaves the
  machine.

### 2.5 Assumptions and dependencies

Each assumption is unconfirmed. Every one of them has a matching open question
in appendix B, an owner and a confirm-by date. No requirement below may depend
on an assumption without naming it.

| ID | Assumption | Owner | Confirm by |
|---|---|---|---|
| A-1 | CONFIRMED 2026-09-19. The monitor id serves, for example `DISPLAY#HSJ1340#5&14514d51&0&UID4356`. Every one was unchanged after a reboot and still sat on the same device name. The UID is what distinguishes displays of the same model: the left and the right screen both report model `HSJ1340` and differ only by `UID4356` against `UID4354`. See appendix E. | Oliver | closed 2026-09-19 |
| A-2 | CONFIRMED 2026-09-19, as three rules rather than one value. A Store-packaged application is identified by its application user model id, which carries no version. An application installed under a versioned directory is identified by the updater command that does not move. Every other application is identified by its path. See appendix E. | Oliver | closed 2026-09-19 |
| A-3 | FALSE AS STATED, measured 2026-09-20. It holds for NordVPN and GameGlass, whose processes were alive 43 minutes after their windows were closed. It does not hold for Postal Gambit, which is an ordinary windowed application: closing its window ends it. The agent therefore cannot treat closing as harmless without knowing which kind it is dealing with. FR-064 answers that by not deciding: closing is a setting the user turns on, off until they do. | Oliver | closed 2026-09-20 |
| A-4 | CONFIRMED 2026-09-20, in one form only. An application that starts with no visible window can be asked to show one by running it again; it cannot be made to show one by acting on the window from outside. Measured against NordVPN: showing the hidden window directly produced an empty frame the application was not drawing, while a second launch made the running instance show and draw that same window, after which the second process exited by itself. | Oliver | closed 2026-09-20 |
| A-5 | CONFIRMED 2026-09-19. A window was moved to each of four displays in turn and maximised there, from a process holding no administrator rights, including from a 96 dpi display to a 240 dpi one. Every move landed where it was told. The sequence that works: restore the window, set its rectangle to the target display's work area, then maximise. See appendix E. | Oliver | closed 2026-09-19 |
| A-6 | NO LONGER LOAD-BEARING. It asked whether startup finishes inside the ceiling, which mattered only while placement waited on the whole profile. FR-055 places each entry as its own window appears, so a late application delays nothing but itself and the ceiling merely bounds how long the agent keeps waiting. | Oliver | closed 2026-09-20 |

---

## 3. Requirements

Priorities follow MoSCoW. The distribution is stated in appendix D.

### 3.1 Functional requirements

#### Profiles and storage

**FR-001 Profile store**
Priority: Must
Requirement: The ScreenState agent shall store the signed-in user's profiles in
the user's own profile store.
Rationale: OOS-7, C-2.
Acceptance: Given two Windows users each holding profiles, when one signs in,
then the manager lists only that user's profiles.
Verified by: an integration test over a redirected store root.

**FR-002 Named profiles**
Priority: Must
Requirement: The ScreenState agent shall identify each profile by a name unique
within the user's profile store.
Acceptance: Given a profile named "Desk", when the user saves a second profile
as "Desk", then the agent refuses the save and states that the name is in use.

**FR-003 Profile contents**
Priority: Must
Requirement: The ScreenState agent shall record in each profile, for every
entry, the application identity, whether the application was running and every
placement of that application's windows.
Acceptance: Given a captured profile holding four running applications with one
window each, when the stored profile is read back, then it holds four entries,
each with one placement naming a display identity, a normal rectangle and a show
state.

**FR-004 Placement contents**
Priority: Must
Requirement: The ScreenState agent shall record in every placement the display
identity, the normal rectangle and the show state of the window.
Rationale: a maximised window still has a normal rectangle, which decides where
it lands when it is restored down.

**FR-005 End state with no window**
Priority: Must
Requirement: When an application in a capture is running with no visible window,
the ScreenState agent shall record that entry as running with no placement.
Rationale: an application can be wanted without any of its windows being wanted
anywhere in particular, which is how the owner's tray applications are usually
left. The entry says the application should run and says nothing about where,
so a restore launches it if absent and then leaves its windows alone.
Acceptance: Given NordVPN running with its window hidden, when the user captures
a profile, then the NordVPN entry records running with no placement.

**FR-006 Atomic write**
Priority: Must
Requirement: The ScreenState agent shall write a profile so that an interruption
during the write leaves either the previous profile or the new profile intact,
never a partial one.
Acceptance: Given a profile store holding "Desk", when a write is interrupted
before completion, then reading "Desk" yields the previous contents.

#### Capture

**FR-010 Capture on demand**
Priority: Must
Requirement: When the user requests a capture, the ScreenState agent shall read
the current state of every candidate application and present the resulting
entries for review.
Rationale: capture happens only when asked. Continuous tracking would record
every temporary arrangement as though it were intended.

**FR-011 Review before write**
Priority: Must
Requirement: When the user confirms a review, the ScreenState agent shall write a
profile holding the entries remaining in the review list.
Acceptance: Given a capture listing twelve entries, when the user removes five
and confirms, then the stored profile holds seven entries.

**FR-016 Review cancelled**
Priority: Must
Requirement: If the user cancels a review, then the ScreenState agent shall
write no profile.

**FR-012 Candidate applications**
Priority: Must
Requirement: The ScreenState agent shall treat as a candidate every application
owned by the signed-in user that either has a top-level window or is recorded in
an existing entry of the profile being captured.
Rationale: without a rule, a capture either lists hundreds of background
processes or silently omits an application the user wants.

**FR-013 Capture excludes ScreenState**
Priority: Must
Requirement: The ScreenState agent shall exclude itself from every capture.

**FR-014 Failure to read a window**
Priority: Should
Requirement: If the agent cannot read the state of a candidate window during a
capture, then the ScreenState agent shall omit that window from the capture and
shall name it in the review as unreadable.

#### Sign-in restore

**FR-020 Settled: WITHDRAWN**
There is no global settled state. It waited on the whole profile before placing
any of it, so the slowest entry gated every other one, although whether Stellody
sits on its display has no bearing on whether Claude has started yet. Replaced
by FR-055. The number is retired rather than reused.

**FR-021 Wait before applying: WITHDRAWN**
Withdrawn with FR-020, for the same reason. An entry whose window exists can be
placed; nothing is gained by holding it back.

**FR-022 Apply on settling: WITHDRAWN**
Withdrawn with FR-020. Placement is now per entry, so there is no single moment
at which the profile is applied.

**FR-055 Place each entry as its window appears**
Priority: Must
Requirement: During a restore, the ScreenState agent shall place each entry as
soon as the window that entry names exists, independently of every other entry;
the entry is satisfied once placed.
Rationale: the end state is defined per entry, so each one converges on its own.
Holding every placement until the whole profile is ready made the restore wait
for its slowest application while the desktop sat wrong; it also required a
definition of finished that Windows cannot supply.
Acceptance: Given a profile naming Stellody and Claude, when Stellody's window
has appeared and Claude's has not, then Stellody is placed and the restore
continues waiting for Claude.

**FR-023 Ceiling**
Priority: Must
Requirement: When the ceiling passes with entries still unsatisfied, the
ScreenState agent shall stop waiting for them and shall record every one it
could not satisfy in the report.
Rationale: the ceiling bounds how long the agent keeps waiting for a window that
may never appear. It is a policy choice rather than a measurement of how long
this machine takes to start.
Acceptance: Given a default profile naming an application that never starts,
when the ceiling passes, then the other entries are already placed and the
report names that application as not started.

**FR-024 Launch missing applications**
Priority: Must
Requirement: When the agent begins a restore, the ScreenState agent shall launch
every application the profile records as running that is not running.
Rationale: launching at the start rather than one at a time lets those
applications load alongside the rest.

**FR-025 No second instance**
Priority: Must
Requirement: The ScreenState agent shall not launch an application that is
already running, except as FR-036 requires in order to make a running
application show a window it is holding hidden.
Rationale: the exception is not a loophole. Running a second copy is the only
measured way to get a hidden window back; on a single-instance application it
starts no second instance: the copy signals the one already running and
exits. Where an application is not single-instance, a second process does
survive and shows a window of its own, which is a different outcome the agent
can see.
Acceptance: Given Discord already running with a visible window, when a restore
begins, then no second Discord process is started.

**FR-026 Launch failure**
Priority: Must
Requirement: If an application cannot be launched, then the ScreenState agent
shall record the failure in the report, naming the application and the reason,
then continue with the remaining entries.

**FR-027 Place a window**
Priority: Must
Requirement: When applying a placement, the ScreenState agent shall set the
window's normal rectangle relative to the recorded display, then set the
recorded show state.
Rationale: maximising a window maximises it on the display its normal rectangle
sits on, so the rectangle is set first.
Acceptance: Given a placement naming the left display and the maximised show
state, when it is applied, then the window is maximised on the left display; restoring
it down places it within that display.

**FR-028 Close an unwanted window: WITHDRAWN**
A restore closes nothing. This required the agent to ask a window to close when
its entry recorded the application as running with no placement. It was drafted
while closing was believed to be uniformly safe. OQ-4 measured that it
is not: NordVPN and GameGlass survive their windows closing, Postal Gambit is
ended by it. Nothing about a window says which kind it is, so the agent would
have been quitting applications and taking whatever was unsaved in them with
it, in the name of tidying a desktop.

An application that should be present but out of the way is recorded with the
minimised show state, which is reversible, loses nothing and quits nothing. The
number is retired rather than reused.

**FR-063 Windows the profile does not name**
Priority: Must
Requirement: When a restore has satisfied every entry it can, the ScreenState
agent shall put away every visible window whose application the profile does not
name, excluding its own windows, then shall name each one in the report. Putting
a window away means minimising it, unless FR-064 says it is to be closed.
Rationale: reported 2026-09-20. Applications that start with Windows and take no
part in a session, NordVPN and GameGlass on the reference machine, arrived on top
of the arrangement and had to be put away by hand, which is the work this product
exists to remove. Minimising is what happens where the user has chosen nothing:
it asks the application for nothing at all and loses nothing. A window already
minimised is left alone, having nothing left to do. The agent's own windows are
never put away: the manager is where Apply was pressed.

**FR-064 Closing the windows the profile does not name**
Priority: Should
Requirement: The ScreenState agent shall offer a setting, off unless the user
turns it on, under which the restore that runs at sign-in asks each window that
FR-063 would put away to close instead. Where such a window is still open once
the settle time has passed, the agent shall minimise it and shall record in the
report that it did not close. Every other restore shall minimise those windows
whatever the setting says, including the one that runs when the agent is started
by hand. The agent shall never close a window the
profile names and shall never close one of its own.
Rationale: reported 2026-09-20 by the owner, having watched FR-063 work. Most of
the applications that start with Windows go to the notification area when their
window is closed, which is where their owner wanted them; minimising leaves them
on the taskbar instead, which is tidier than before and is not what was asked
for. A-3 measured that an ordinary windowed application ends when its window
closes, so this is the user's decision to make and not the agent's: the setting
is off until they turn it on and says in as many words what it costs. Closing is
a request rather than an order, so a refusal is an ordinary answer and is met by
the act the setting replaced. It is the sign-in restore alone because that is
the one that builds a desktop from nothing: pressing Apply happens in the middle
of a session, where a window being asked to close is a surprise the user did not
ask for (ruled by the owner, 2026-09-20). The agent restores the default profile
whenever it starts with no copy of it already running, which is a launch from
the shortcut as well as a sign-in, so the two are told apart by the flag the
sign-in entry carries (FR-046) rather than by the fact that a restore is
running.

**FR-065 Progress while a restore runs**
Priority: Should
Requirement: While a restore started from the manager is running, the manager
shall show how many of the profile's entries have been satisfied out of the
number the profile holds.
Rationale: reported 2026-09-20. The manager holds the user on a panel that
offers nothing while a restore runs, which FR-041 allows to take minutes, so
the panel has to say the wait is going somewhere. Entries are counted rather
than seconds elapsed, because the agent knows how many entries are settled and
cannot know how long an application will take to put a window up: a bar
weighted by time would be a guess drawn as a measurement.

**FR-066 Where the report leaves the user**
Priority: Should
Requirement: When a restore started from the manager ends, the manager shall
present the report over the profile list, so that closing the report leaves the
profile list showing.
Rationale: reported 2026-09-20. The report used to go up over the panel that
said the restore was running, so closing it left the user looking at an
Applying panel for work that had finished, with nothing on it to press. The
list is drawn before the report rather than after it is dismissed, so leaving by
the cross, by Escape or by the backdrop all land in the same place.

**FR-067 The taskbar after a sign-in restore**
Priority: Should
Requirement: When a restore that ran because the user signed in has settled,
the ScreenState agent shall ask every taskbar to paint itself again. It shall
record in the log how many it asked.
Rationale: reported 2026-09-21. After a sign-in the taskbar buttons of the
applications the agent had started were drawn without their icons on the
reference machine; they stayed that way until the user clicked anywhere on the
taskbar: the icons were there and the drawing of them was stale. A restore the
user asked for never showed it, which is why this is the sign-in restore alone.
The act is a repaint and nothing else: it moves no window, ends nothing and
tells the shell nothing. Whether it is enough is not yet known, which is what
the log line is for: a boot that still shows the fault with the line present
says the click does something a repaint does not.

**FR-068 Reading what a profile arranges**
Priority: Should
Requirement: The manager shall show what a profile arranges only when the user
asks for it, in a dialog holding each application, what it arranges and a
control to take it out of the profile. The control that opens the dialog shall
be inert while no profile is selected.
Rationale: reported 2026-09-21. The applications filled a column beside the
profile list and shared it with the settings, which left every part of the
window cramped: a path was cut off after a few words, each setting added took a
row off the list above it and the window could not be read at a glance. What a
profile holds is looked at now and then rather than watched, so it is asked for
rather than always on screen. The main screen keeps the profile list, the
settings and the buttons along the foot; the dialog has room for a path to wrap
and be read whole.

**FR-029 Never terminate**
Priority: Must
Requirement: The ScreenState agent shall not terminate any process it did not
start. It shall not close any window either, with one exception: at sign-in, a
window the profile does not name, while the user has turned FR-064 on.
Rationale: C-3, widened by measurement, then narrowed by a decision. Terminating
is never allowed and never will be. Closing was forbidden too, once OQ-4 showed
that closing a window ends some applications outright, which is a termination
reached by another route. The owner then asked for closing back as a setting,
knowing that measurement, because the applications it is aimed at go to the
notification area rather than ending. So closing is not something the agent
decides: it happens only where the user has said so, only to a window no profile
names and never to a process.

**FR-030 Window refused to close: WITHDRAWN**
Withdrawn with FR-028. There is no close request to be refused.

**FR-031 Missing display**
Priority: Must
Requirement: If the display named by a placement is not connected, then the
ScreenState agent shall apply that placement to the primary display and shall
record the substitution in the report.
Acceptance: Given a profile placing Stellody on the left display, when that
display is disconnected, then Stellody is maximised on the primary display and
the report names the substitution.

**FR-057 Displays changing during a restore**
Priority: Must
Requirement: If the set of connected displays changes while a restore is in
progress, then the ScreenState agent shall continue the restore, shall place
every remaining entry against the displays as they then stand and shall record
the change in the report.
Rationale: abandoning the restore would leave the desktop half arranged, which
is worse than the state it started from. A placement naming a display that is
no longer there is already handled by FR-031, so the change needs no rule of
its own beyond continuing and saying that it happened.
Acceptance: Given a restore in progress with two entries outstanding, when a
display is disconnected, then both remaining entries are placed and the report
names the display that went away.

**FR-058 Update check**
Priority: Should
Requirement: The ScreenState agent shall check once per run whether a newer
version has been released, shall offer the user the download when there is one
and shall not raise a version the user has chosen to skip.
Rationale: every other released application of the owner's carries this; a user
who never hears about a fix does not get it. It is the only reason the
product touches the network, which is why C-4 names it.
Acceptance: Given a release newer than the running version, when the agent
starts, then the user is offered the download once, with the choice to skip
that version or be reminded later.

**FR-059 Turning the update check off**
Priority: Should
Requirement: A user shall be able to turn the update check off, after which the
ScreenState agent shall make no network connection at all.
Rationale: C-4 promises that everything else works offline. A user who wants
that promise absolute is entitled to it.

**FR-060 Supporting the project**
Priority: Should
Requirement: The ScreenState manager shall show a donation link to
https://www.paypal.com/ncp/payment/6FMTGJYFJXFTE, stating that ScreenState is
free and stays free with no paid tier, no licence key and no feature held back.
Rationale: the owner's released applications carry this. The wording matters as
much as the link: an ask that implies something is withheld would be false.

**FR-032 Window off the visible desktop**
Priority: Must
Requirement: The ScreenState agent shall place every window so that it lies
within the bounds of a connected display.
Rationale: a rectangle recorded against a display arrangement that has changed
can otherwise land a window where the user cannot reach it.

**FR-033 Verify and re-apply once**
Priority: Must
Requirement: When a placement has been applied, the ScreenState agent shall
re-read the window after the settle-check delay and shall apply the placement
once more if the window no longer matches it.
Rationale: an application may move its own window after starting. The owner
observes Discord arriving in the wrong place after every reboot.

**FR-034 Give up rather than fight**
Priority: Must
Requirement: If a window does not match its placement after the second
application, then the ScreenState agent shall record it in the report and shall
make no further attempt during that restore.
Rationale: an endless contest with an application would leave a window flicking
between two positions.

**FR-035 Window that cannot be moved**
Priority: Must
Requirement: If the agent cannot move a window, then the ScreenState agent shall
record the window in the report, naming the application, then continue with
the remaining placements.
Rationale: covers OOS-5 and any other window Windows refuses to let a normal
process move.

**FR-036 Show a hidden window**
Priority: Should
Requirement: When an entry has a placement and the application is running with
no visible window, the ScreenState agent shall attempt to make the window
visible before applying the placement.
Rationale: an application that starts into the tray has no window to move.
Two mechanisms were measured against NordVPN. Showing the hidden window from
outside, on 2026-09-19, produced an empty frame the application was not
drawing, so that mechanism is withdrawn. Running a second copy, on 2026-09-20,
made the running instance show and draw that same window, at the same handle
and the same rectangle, after which the second process exited by itself. That
is what a user does from the tray; it is the mechanism this requirement now
uses. Where no window appears within the settle-check delay the report
states that the application could not be shown.

**FR-056 Ask an application to show its own window**
Priority: Should
Requirement: To satisfy FR-036, the ScreenState agent shall run the
application's own launch command again and shall place the window that appears,
rather than acting on the hidden window directly.
Rationale: the window belongs to the application and only the application draws
it. Acting on it from outside produces a frame with nothing in it.
Acceptance: Given NordVPN running with its window hidden, when the agent runs
NordVPN again, then the existing window becomes visible and is placed.

**FR-037 Several windows of one application**
Priority: Should
Requirement: When a profile entry holds more than one placement, the ScreenState
agent shall apply the placements to that application's windows in the order the
agent first saw those windows; the report shall name every placement left
without a window and every window left without a placement.
Rationale: window handles do not survive a session, so a saved window can only
be matched by a rule. Windows records no creation time for a window, so creation
order cannot be read from the system; the order the agent first saw a window
can be. During a restore the two agree, because the agent is watching while the
windows appear: a window seen on a later pass is genuinely newer than one seen
on an earlier one. Windows already open when the agent starts share one moment
and keep the order the first enumeration gave them, which is a stacking order
rather than an age. That limit is stated here rather than hidden behind a claim
about creation the system cannot answer.

**FR-038 Default profile applies at sign-in**
Priority: Must
Requirement: When the user signs in, the ScreenState agent shall restore the
profile marked as default.

**FR-039 No default profile**
Priority: Must
Requirement: If no profile is marked as default, then the ScreenState agent
shall apply no profile at sign-in and shall state in the manager that no default
is set.

**FR-062 One profile is always the default**
Priority: Must
Requirement: While exactly one profile is stored, the ScreenState agent shall
keep that profile marked as the default and shall refuse a request to unmark it.
When a capture is saved while no stored profile is marked as default, the agent
shall mark the saved profile as the default.
Rationale: reported 2026-09-20. The only profile stored was unmarked, so a
restart arranged nothing and the user had to open the manager and press Apply.
A capture is the user saying this is how the desktop should look; a marking
they were never told was owed is not a decision they made. One profile and no
marking has nothing to recommend it: signing in arranges nothing while the only
answer to what should be arranged sits in the list. The marking is settled
wherever the set of profiles is read or changed, so a profile stored by an
earlier version is put right on the next sign-in rather than the one after it.
With two profiles or more the marking is the user's and FR-039 governs: none
marked stays a choice they are entitled to make; a later capture never moves the
marking off the profile they chose.

#### Profile management

**FR-040 Exactly one default**
Priority: Must
Requirement: The ScreenState agent shall hold at most one profile marked as
default; marking a profile as default clears the mark from any other.

**FR-041 Apply on demand**
Priority: Must
Requirement: When the user selects a profile from the tray menu, the ScreenState
agent shall restore that profile immediately.
Rationale: during a session the windows are already there, so every entry can be
placed at once.

**FR-042 Manage profiles**
Priority: Must
Requirement: The ScreenState manager shall let the user create a profile, rename
a profile, delete a profile, mark a profile as default, view the entries of a
profile and remove an entry from a profile.

**FR-043 Confirm before deleting**
Priority: Must
Requirement: When the user requests the deletion of a profile, the ScreenState
manager shall present a confirmation naming the profile and shall delete it only
after the user confirms.

**FR-044 Report of the last restore**
Priority: Must
Requirement: The ScreenState agent shall present the report of the most recent
restore, listing every entry it satisfied and every entry it could not, each
with the reason.
Rationale: a restore that silently half worked is the failure mode this product
exists to remove.

**FR-045 Report reachable from the tray**
Priority: Must
Requirement: When the most recent restore recorded an entry it could not
satisfy, the ScreenState agent shall indicate this on the tray icon and shall
open the report when the user selects it.

**FR-046 Start with Windows**
Priority: Must
Requirement: The ScreenState agent shall run at sign-in through a single
per-user registry entry.
Rationale: a second mechanism, such as a Startup folder shortcut, would let the
two disagree.

**FR-053 Turn start with Windows on and off**
Priority: Must
Requirement: The ScreenState manager shall let the user turn the sign-in entry
of FR-046 on and off.

**FR-047 Single instance**
Priority: Must
Requirement: The ScreenState agent shall run as one instance per signed-in user.

**FR-054 Second launch**
Priority: Must
Requirement: If the agent is launched while an instance is already running for
that user, then the second launch shall present the existing instance's manager
and shall end.

**FR-048 Manager is not required for restore**
Priority: Must
Requirement: The ScreenState agent shall complete a sign-in restore without the
manager window being opened.

**FR-049 Cancel a restore**
Priority: Should
Requirement: When the user cancels a restore in progress, the ScreenState agent
shall stop before the next action and shall record the cancellation in the
report.
Rationale: a restore can run for minutes while waiting for a window to appear. A
wait with no way out is a hang from the user's point of view.

**FR-061 A restore requested during a restore**
Priority: Must
Requirement: When a restore is requested while a restore is already running, the
ScreenState agent shall stop the restore in progress before its next action,
shall leave every window already placed exactly where it is and shall then carry
out the newly requested restore.
Requirement: The ScreenState agent shall record in the report of the replaced
restore that it was replaced; it shall record in the report of the new restore
that it replaced one in progress.
Rationale: a restore is a statement of what the desktop should look like now, so
the newest request is the one that is true. Refusing it leaves the user looking
at a desktop that neither profile describes, with no way to get the one they just
asked for until the first finishes. Undoing the windows already placed is worse
still: it moves windows twice to reach the same end; FR-029 has already ruled
that a restore closes and undoes nothing. Silence about the replacement would
leave two reports that each look like a restore that simply stopped, which is the
failure FR-050 exists to prevent.

#### Diagnostics

**FR-050 Step log**
Priority: Must
Requirement: The ScreenState agent shall write to a log file, for every restore,
each step it attempted with its outcome.
Rationale: the worst restore failures are the ones that raise nothing. A restore
happens minutes after sign-in, when nobody is watching.

**FR-051 Survive a failed entry**
Priority: Must
Requirement: If any single entry fails during a restore, then the ScreenState
agent shall continue with the remaining entries.

**FR-052 Survive a crash of its own work**
Priority: Must
Requirement: If an unexpected failure occurs while the agent is restoring, then
the ScreenState agent shall record it in the log and the report, shall keep the
tray icon present and shall remain able to restore again.
Rationale: an agent that vanishes leaves the user with a half-arranged desktop
and nothing to read.

### 3.2 Non-functional requirements

Every number below is measured on the reference machine in section 2.3.

**NFR-PERF-001 Placement speed**
Priority: Must
Requirement: The ScreenState agent shall complete the placement of up to 20
windows whose windows already exist within 3 seconds.
Method: timestamps in the step log, from the first placement to the last.

**NFR-PERF-002 Quiet period: WITHDRAWN**
The agent no longer waits for a quiet period, so there is no such value to set.
FR-020 settles on the profile's own entries alone. The number is retired rather
than reused, so nothing that cited it can quietly come to mean something else.

**NFR-PERF-003 Ceiling**
Priority: Must
Requirement: The ScreenState agent shall treat 15 minutes after sign-in as the
ceiling, configurable by the user between 1 and 60 minutes.
Method: the value is read from configuration and asserted in a test over a fake
clock.
Rationale: the ceiling is a policy choice about how long to keep waiting for a
window that may never appear. It is deliberately not derived from how long this
machine takes to start; the attempts to measure that are a caution rather than a
source. On the reference machine the applications that start by
themselves all had windows within about 90 seconds of sign-in. Every later
appearance in those runs, at 3, 4, 5 and 7 minutes, was the owner launching
applications by hand while the measurement ran, which was mistaken for slow
startup twice before he said so. Fifteen minutes is therefore generous by an
order of magnitude against the only figure that was ever clean, which is what a
ceiling should be: the point at which waiting is abandoned, not a prediction.

**NFR-PERF-004 Settle-check delay**
Priority: Must
Requirement: The ScreenState agent shall re-read each placed window 10 seconds
after placing it, for the check in FR-033.
Method: the value is read from configuration and asserted in a test over a fake
clock. The default is provisional until A-6 is confirmed.

**NFR-PERF-005 Idle cost**
Priority: Must
Requirement: While no restore or capture is in progress, the ScreenState agent
shall consume under 1 percent of one processor core averaged over 10 minutes.
Method: Windows performance counters over a 10 minute idle observation.

**NFR-PERF-007 Idle memory**
Priority: Must
Requirement: While no restore or capture is in progress, the ScreenState agent
shall hold a working set under 80 MB.
Method: Windows performance counters over a 10 minute idle observation.

**NFR-PERF-006 Startup cost**
Priority: Should
Requirement: The ScreenState agent shall reach the point of watching for the
profile's windows within 2 seconds of being started at sign-in.
Rationale: an agent that is itself slow at sign-in adds to the problem it exists
to solve.

**NFR-REL-001 No data loss**
Priority: Must
Requirement: The ScreenState agent shall lose no stored profile if the process
ends at any point, including during a write.
Method: a test that interrupts a write and reads the store back.

**NFR-REL-002 Unreadable store**
Priority: Must
Requirement: If a profile file cannot be read or does not match the stored
format, then the ScreenState agent shall present the failure naming the file,
shall leave the file unchanged and shall continue with the profiles it could
read.
Rationale: absence is an answer; corruption is a fault; neither is a reason to
end the run.

**NFR-SEC-001 No elevation**
Priority: Must
Requirement: The ScreenState agent and the setup program shall run without
administrator rights.

**NFR-SEC-002 No network**
Priority: Must
Requirement: The ScreenState agent shall open no network connection.
Method: an observation of the process's connections over a full capture and
restore cycle.

**NFR-PRIV-001 What is stored**
Priority: Must
Requirement: The ScreenState agent shall store nothing beyond application
identities, window geometry, window show states, display identities and profile
names.

**NFR-PRIV-002 Stated non-claims**
Priority: Must
Requirement: The documentation shall state that ScreenState does not encrypt its
profile store, does not send anything off the machine and does not restore the
contents of any application.
Rationale: window titles can be revealing. A user might assume protection that
is not there.

**NFR-PORT-001 Target**
Priority: Must
Requirement: ScreenState shall run on Windows 11 64 bit.
Open: Windows 10 (OQ-9).

**NFR-MAINT-001 Layering**
Priority: Must
Requirement: The codebase shall hold the dependency direction
UI to Application to Domain, with Infrastructure depending inwards, enforced by
a structural test.

**NFR-MAINT-002 Coverage**
Priority: Must
Requirement: The test suite shall cover the domain and application layers to
100 percent, enforced as a gate that fails the build below that number.
Rationale: those are the layers reachable with no displays, no clock and no
windows.

**NFR-MAINT-003 Module size**
Priority: Must
Requirement: No source file shall exceed 400 lines, enforced by a structural
test, with build and packaging scripts exempt.

**NFR-OBS-001 Log location and retention**
Priority: Must
Requirement: The ScreenState agent shall write its log under the user's own
application data directory and shall retain the most recent 10 restores.

**NFR-USE-001 Capture effort**
Priority: Should
Requirement: A user shall be able to capture and save a profile from the desktop
as it stands in under 60 seconds, measured from opening the tray menu to the
profile being stored.
Method: a timed walkthrough by the owner on the reference machine.

**NFR-USE-002 Keyboard navigation**
Priority: Should
Requirement: The ScreenState manager shall be operable entirely by keyboard,
with a visible focus indicator on the focused control and on no container.

### 3.3 External interface requirements

**EIR-001 Tray**
Priority: Must
Requirement: The ScreenState agent shall present a tray icon whose menu offers
each profile, a capture, the manager, the report and an exit.

**EIR-002 Manager window**
Priority: Must
Requirement: The ScreenState manager shall present the profile list, the entries
of the selected profile, the default marking and the settings in one window.

**EIR-003 Artwork**
Priority: Must
Requirement: The ScreenState agent shall use the artwork supplied by the owner
for the application icon and the manager, with the icon carried by the
executable rather than as a separate file beside it.

**EIR-004 Windows interfaces used**
Priority: Must
Requirement: The ScreenState agent shall obtain display arrangement, window
enumeration, window geometry, window show state and process information from
documented Windows interfaces.
Rationale: an undocumented interface, such as reading the tray's contents out of
Explorer, is a dependency on furniture Microsoft moves between releases. See
OQ-3.

### 3.4 Data requirements

**DATA-001 Store location**
Priority: Must
Requirement: The ScreenState agent shall hold the profile store under the
signed-in user's local application data directory.

**DATA-002 Format version**
Priority: Must
Requirement: Every stored profile shall carry the version of the format it was
written in.

**DATA-003 Unknown format version**
Priority: Must
Requirement: If a profile carries a format version the agent does not
understand, then the ScreenState agent shall leave the file unchanged, shall
exclude it from the profile list and shall state why.

**DATA-004 Retention**
Priority: Must
Requirement: The ScreenState agent shall retain every profile until the user
deletes it.

**DATA-005 Removal on uninstall**
Priority: Must
Requirement: When the user uninstalls ScreenState, the setup program shall remove
the profile store only if the user agrees to its removal.

---

## 4. Other requirements

### 4.1 Legal

The work is licensed GPL-3.0, as carried in the repository.

### 4.2 Internationalisation

English only in the first release. No requirement below depends on locale.

### 4.3 Risk

A formal FMEA is disproportionate for a personal desktop utility with no safety,
financial or regulatory exposure. The risks that matter are carried as open
questions in appendix B, each with a measurement that settles it.

---

## 5. Worked example: the owner's desktop

The case that prompted the product, recorded as the primary acceptance scenario.

Displays on the reference machine, as Windows Settings numbers them:

| Settings number | Position | Size |
|---|---|---|
| 1, above the others | primary | 3440x1440 |
| 4, left | left of centre | 3840x2400 |
| 3, centre | below display 1 | 3840x2400 |
| 2, right | right of centre | 3840x2400 |

```
Given profile "Desk" is marked as default and records:
  Claude          running, maximised on display 1
  Stellody        running, maximised on display 4
  Discord         running, maximised on display 3
  PigeonPost      running, maximised on display 2
  NordVPN         running, no window
  GameGlass       running, no window
  Postal Gambit   running, no window
And Claude, Discord, PigeonPost, NordVPN and GameGlass start at sign-in by
  themselves
And neither Stellody nor Postal Gambit starts by itself

When the user signs in

Then the agent launches Stellody and Postal Gambit; it launches nothing else
And each of the four placed applications is placed as its own window appears,
  without waiting for the others
And Claude is maximised on display 1
And Stellody is maximised on display 4
And Discord is maximised on display 3
And PigeonPost is maximised on display 2
And the NordVPN, GameGlass and Postal Gambit windows are not touched, since
  their entries record no placement
And all seven applications are still running
And the report lists seven entries satisfied and none outstanding
```

Second scenario, derived from FR-023 and FR-026:

```
Given the profile above
And Stellody cannot be launched because its executable is absent

When the user signs in

Then the other six entries are satisfied
And the report names Stellody, states that it could not be launched and states
  the reason
And the tray icon indicates that the restore was incomplete
```

---

## Appendix A: The silence check

Answers to the routine sources of a missed requirement. Each either points at a
requirement or is recorded as deliberately unaddressed.

| Situation | Answer |
|---|---|
| First run, no profiles | FR-039. The manager states that no default is set. |
| A profile naming an application that is no longer installed | FR-026. Reported, the restore continues. |
| A display arrangement that has changed since capture | FR-031 and FR-032. |
| The largest plausible input | An entry per running application, a placement per window. NFR-PERF-001 fixes the number tested at 20 windows. |
| A restore interrupted part way | FR-049 and FR-052. The report records how far it got. |
| Two restores at once | FR-047 gives one agent per user. FR-061 settles what that one agent does: the newer request replaces the running restore, undoing nothing already placed; both reports say so. |
| Upgrade from a previous version | DATA-002 and DATA-003. |
| The user having no permission | OOS-5 and FR-035. |
| Disk unavailable or store unreadable | NFR-REL-002. |
| The user signing out during a restore | The agent's session ends with it. Nothing is written half way, per FR-006. |
| A display connected during a restore | Out of scope per OOS-6. The restore continues against the arrangement it read at the start. Recorded as OQ-10. |
| Time or timezone change | No requirement depends on wall-clock time, only on elapsed spans. |

---

## Appendix B: Open questions register

Nothing here may be left open at baselining. Each question names the measurement
that settles it. The first five form the spike.

| ID | Question | Owner | Confirm by |
|---|---|---|---|
| OQ-1 | CLOSED. The monitor id, which survived a reboot unchanged and distinguishes same-model displays by their UID. Neither the device name nor the number Windows Settings shows can identify a display: Settings 4 is device DISPLAY3. See appendix E. | Oliver | closed 2026-09-20 |
| OQ-2 | CLOSED. Three rules: application user model id for a Store-packaged application, the updater command for one installed under a versioned directory, the path for everything else. A window class cannot be used, because it carries a GUID that changes every session. See appendix E. | Oliver | closed 2026-09-20 |
| OQ-3 | CLOSED. Yes, from a process without administrator rights, across a 96 dpi to 240 dpi boundary. Restore, set the rectangle to the target work area, then maximise. See appendix E. | Oliver | closed 2026-09-20 |
| OQ-4 | CLOSED. Not uniformly. NordVPN and GameGlass survive their windows closing; Postal Gambit does not. See A-3. See appendix E. | Oliver | closed 2026-09-20 |
| OQ-5 | CLOSED, dissolved rather than answered. It existed to supply two things: the quiet period, which FR-020 no longer has, plus the ceiling, which is a policy choice about how long to keep trying rather than a fact about this machine. Settling is now defined entirely by whether the profile's own entries are satisfied, so no timing needs measuring. | Oliver | closed 2026-09-20 |
| OQ-6 | CLOSED 2026-09-20. Not from outside the window: that produced an empty frame. By asking the application, yes. Running NordVPN again while it was running made the running instance show its own hidden window at the same handle and rectangle; the second process then exited by itself. FR-036 stands, with its mechanism changed to FR-056. | Oliver | closed 2026-09-20 |
| OQ-7 | CLOSED, no longer load-bearing. FR-033 re-applies a placement once when the window no longer matches; FR-034 gives up rather than fight. Both hold whether or not applications move their own windows, so the answer changes no requirement. The one run that recorded movement recorded the owner dragging windows, which is also why this cannot be measured on a machine in use. | Oliver | closed 2026-09-20 |
| OQ-8 | CLOSED 2026-09-20. The newer request wins. The agent stops the restore in progress before its next action, leaves every window already placed where it is and then carries out the new one; both reports record the replacement. Written as FR-061. | Oliver | closed 2026-09-20 |
| OQ-9 | CLOSED 2026-09-20. Windows 11 is the supported target. Windows 10 will very likely work, since nothing Windows 11 introduced is used, though it is untested and therefore unsupported. Nothing is done to prevent it running there. | Oliver | closed 2026-09-20 |
| OQ-10 | CLOSED 2026-09-20. The restore continues against the displays as they then stand and the report records the change. Abandoning it would leave the desktop half arranged, which is worse than where it started. See FR-057. | Oliver | closed 2026-09-20 |
| OQ-11 | CLOSED 2026-09-20. Yes, as every other released application of the owner's carries one. C-4 is reworded to name it as the single outbound call; FR-059 lets the user turn it off and have C-4 absolutely. See FR-058. | Oliver | closed 2026-09-20 |
| OQ-12 | CLOSED 2026-09-20. Yes. FR-060 carries the link in the manager. | Oliver | closed 2026-09-20 |

---

## Appendix C: Traceability

The matrix is filled as implementation proceeds. Every requirement names the
test that verifies it. No requirement holding an open question is entered until
that question is closed.

| Requirement | Design element | Test |
|---|---|---|
| (to be completed) | | |

---

## Appendix D: MoSCoW distribution

Counted from this document rather than carried forward, because the previous
figures had drifted about ten below the requirements actually written. Recounted
on 2026-09-20 by scanning every `Priority:` line: the Must figure had been
written as 56 against 65 actually present, a transposition that the earlier
recount did not catch. FR-061 took it to 66, FR-062 to 67 and FR-063 to 68. Withdrawn requirements carry no
`Priority:` line, so they are not in the Must or Should figures.

| Priority | Count | Notes |
|---|---|---|
| Must | 68 | The product does not work without any one of them. |
| Should | 16 | FR-014, FR-036, FR-037, FR-049, FR-056, FR-058, FR-059, FR-060, FR-064, FR-065, FR-066, FR-067, FR-068, NFR-PERF-006, NFR-USE-001 and NFR-USE-002, plus the second half of FR-045, which is a Should inside a Must. |
| Could | 0 | |
| Won't this time | 8 | OOS-1 to OOS-8. |
| Withdrawn | 6 | FR-020, FR-021, FR-022, FR-028, FR-030 and NFR-PERF-002. Kept in place with their numbers retired so nothing that cited them can quietly come to mean something else. |

The Must proportion is high for a first release of a utility whose whole purpose
is one behaviour. The check that keeps it honest: every Must names a failure the
owner would call the product broken for. Any that does not gets demoted at the
next pass.

---

## Appendix E: measured facts

Taken from the measurement spike, which was deleted once every question in
appendix B was closed. These are the readings the requirements rest on, kept
here because a requirement whose evidence has been thrown away is an assertion.
All were measured on the reference machine in section 2.3.

### Display identity, 2026-09-19, unchanged after a reboot

| Device name | Settings number | Monitor id | DPI |
|---|---|---|---|
| `\\.\DISPLAY1` | 1, top, primary | `DISPLAY#GSM784F#5&14514d51&0&UID4352` | 96 |
| `\\.\DISPLAY2` | 3, centre | `DISPLAY#HSJB30F#5&14514d51&0&UID4357` | 240 |
| `\\.\DISPLAY3` | 4, left | `DISPLAY#HSJ1340#5&14514d51&0&UID4356` | 240 |
| `\\.\DISPLAY4` | 2, right | `DISPLAY#HSJ1340#5&14514d51&0&UID4354` | 240 |

Two facts the product depends on. The Settings number, the device name and the
physical position do not line up, so neither number can identify a display. The
left and right screens share the model code `HSJ1340`; only the UID tells them
apart. A display is therefore named to the user by where it physically sits,
derived from the desktop layout, never by a number Windows reports.

### Application identity, 2026-09-19

| Application | Installed as | Identity that survives its updates |
|---|---|---|
| Claude | `WindowsApps\Claude_2.2553.1.0_x64__pzs8sxrjxfjjc\app\claude.exe` | `Claude_pzs8sxrjxfjjc!Claude`, an application user model id carrying no version |
| Discord | `Discord\app-1.0.9258\Discord.exe` | `Discord\Update.exe --processStart Discord.exe`, which does not move |
| Stellody | `Programs\Stellody\Stellody.exe` | the path, which carries no version |

A window class is not an identity. NordVPN's main window class was
`HwndWrapper[NordVPNApp;Hosted Main;3bcafb52-...]` before a reboot and
`HwndWrapper[NordVPNApp;Hosted Main;0e6920e5-...]` after it: the GUID changes
every session. The process image path does not.

The candidate rule in FR-012, a visible window that is not cloaked, not owned by
another window, not a tool window and not untitled, reduced 523 enumerated
windows to the 8 a user would call windows.

### Placement, 2026-09-19

A window was moved to each display in turn and maximised, from a process with no
administrator rights.

| Target | Landed on | State | Rectangle |
|---|---|---|---|
| `\\.\DISPLAY1`, 96 dpi | `\\.\DISPLAY1` | maximised | x=-8 y=-8 w=3456 h=1408 |
| `\\.\DISPLAY2`, 240 dpi | `\\.\DISPLAY2` | maximised | x=-239 y=1424 w=3872 h=2312 |
| `\\.\DISPLAY3`, 240 dpi | `\\.\DISPLAY3` | maximised | x=-4079 y=1407 w=3872 h=2312 |
| `\\.\DISPLAY4`, 240 dpi | `\\.\DISPLAY4` | maximised | x=3601 y=1407 w=3872 h=2312 |

A maximised window overhangs its display by the border width, which is why each
rectangle is 16 pixels wider than the display it sits on.

### One process, several windows, 2026-09-19

A Notepad window opened for the placement measurement joined an existing Notepad
process that already held another window. Two consequences. An entry cannot
assume one window per application, which is what FR-037 exists for. Whether a
process is still running also fails to say whether closing one of its windows
was harmless, since another window of it may be keeping it alive.

### Closing a window, 2026-09-20

| Application | Process 43 minutes after its window was closed |
|---|---|
| NordVPN | alive, no window |
| GameGlass Hub | alive as 13 processes, none with a window |
| Postal Gambit | gone |

### Showing a hidden window, 2026-09-19 and 2026-09-20

Acting on NordVPN's hidden window from outside made it visible and useless: an
empty frame the application was not drawing. Running NordVPN again while it was
running made the running instance show that same window, at the same handle and
the same rectangle, drawn properly; the second process then exited by itself.
