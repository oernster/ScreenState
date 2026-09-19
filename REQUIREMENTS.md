# ScreenState: Software Requirements Specification

Status: DRAFT, not baselined. Open questions in appendix B must be closed before
baselining.

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
- Closing of windows for applications a profile records as running without a
  visible window.
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
| Settled | The condition defined in FR-020 that ends the wait after sign-in. |
| Quiet period | The span during which no new top-level window may appear for the startup to count as settled. |
| Ceiling | The maximum span after sign-in that the agent waits for settling. |
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

- Windows 11, 64 bit, as the supported target. Windows 10 support is an open
  question (OQ-9).
- Reference machine for every measured requirement: the owner's desktop, four
  displays, one 3440x1440 primary plus three 3840x2400, mixed positions.
- No network access is required at any point.
- Installed and run per user. No administrator rights at install or at run.

### 2.4 Constraints

- **C-1** ScreenState never requests administrator rights.
- **C-2** All files it writes live under the signed-in user's profile
  directories. No machine-wide registry keys, no machine-wide files.
- **C-3** ScreenState never terminates a process it did not start.
- **C-4** ScreenState makes no network connection. Nothing is sent off the
  machine.

### 2.5 Assumptions and dependencies

Each assumption is unconfirmed. Every one of them has a matching open question
in appendix B, an owner and a confirm-by date. No requirement below may depend
on an assumption without naming it.

| ID | Assumption | Owner | Confirm by |
|---|---|---|---|
| A-1 | A display identity exists that survives a reboot and distinguishes three displays of the same model. | Oliver | 2026-09-26 |
| A-2 | An application identity exists that survives an application's updates, both for Store-packaged applications such as Claude and for applications installed into versioned directories such as Discord. | Oliver | 2026-09-26 |
| A-3 | Closing the main window of NordVPN, GameGlass and Postal Gambit leaves each application running in the tray. | Oliver | 2026-09-26 |
| A-4 | An application that starts with no visible window can be made to show one from outside the application. | Oliver | 2026-09-26 |
| A-5 | A window can be moved to a target display and maximised there by a process without administrator rights, across displays with different scaling. | Oliver | 2026-09-26 |
| A-6 | Confirmed in part on 2026-09-19: the last startup window appeared 5 minutes 34 seconds after boot, under the 15 minute ceiling. A second run is needed to confirm that nothing appears later. | Oliver | 2026-09-26 |

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
Rationale: this is how the owner's NordVPN, GameGlass and Postal Gambit windows
come to be closed on restore, without a separate concept of dismissal.
Acceptance: Given NordVPN running with its window closed, when the user captures
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

**FR-020 Settled**
Priority: Must
Requirement: The ScreenState agent shall treat the startup as settled when every
application recorded in the profile as running is running, every such
application that has a placement has at least one top-level window and no new
top-level window has appeared for the quiet period.
Rationale: Windows gives no signal that startup has finished. The profile itself
defines what finished means.

**FR-021 Wait before applying**
Priority: Must
Requirement: While the startup is not settled, the ScreenState agent shall not
apply any placement.
Acceptance: Given a default profile naming Discord, when Discord's window has
not yet appeared, then no window has been moved.

**FR-022 Apply on settling**
Priority: Must
Requirement: When the startup becomes settled, the ScreenState agent shall apply
the default profile.

**FR-023 Ceiling**
Priority: Must
Requirement: If the startup has not settled within the ceiling after sign-in,
then the ScreenState agent shall apply the default profile to the state as it
stands and shall record every entry it could not satisfy in the report.
Acceptance: Given a default profile naming an application that never starts,
when the ceiling passes, then the other entries are placed and the report names
that application as not started.

**FR-024 Launch missing applications**
Priority: Must
Requirement: When the agent begins a restore, the ScreenState agent shall launch
every application the profile records as running that is not running.
Rationale: launching at the start rather than after settling lets those
applications load alongside the rest.

**FR-025 No second instance**
Priority: Must
Requirement: The ScreenState agent shall not launch an application that is
already running.
Acceptance: Given Discord already running, when a restore begins, then no second
Discord process is started.

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

**FR-028 Close an unwanted window**
Priority: Must
Requirement: When an entry records running with no placement and that
application has a visible top-level window, the ScreenState agent shall request
that the window close.
Acceptance: Given NordVPN showing its window after sign-in, when the restore
applies, then the NordVPN window is closed and the NordVPN process is still
running.

**FR-029 Never terminate**
Priority: Must
Requirement: The ScreenState agent shall not terminate any process it did not
start.
Rationale: C-3. Closing a window is a request the application may refuse.

**FR-030 Window refused to close**
Priority: Must
Requirement: If a window the profile records as unwanted is still present after
the close request, then the ScreenState agent shall record it in the report and
shall leave it alone.

**FR-031 Missing display**
Priority: Must
Requirement: If the display named by a placement is not connected, then the
ScreenState agent shall apply that placement to the primary display and shall
record the substitution in the report.
Acceptance: Given a profile placing Stellody on the left display, when that
display is disconnected, then Stellody is maximised on the primary display and
the report names the substitution.

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
Measured on 2026-09-19 against NordVPN: making its hidden window visible from
outside produced an empty frame the application was not drawing, so A-4 is
false in that form and the mechanism is withdrawn. What is left to measure is
whether running a second copy of the application makes it show its own window,
which is what a user does from the tray. Until that is measured, this
requirement is unsatisfied and the report states that the application cannot be
shown.

**FR-037 Several windows of one application**
Priority: Should
Requirement: When a profile entry holds more than one placement, the ScreenState
agent shall apply the placements to that application's windows in the order the
windows were created; the report shall name every placement left without a
window and every window left without a placement.
Rationale: window handles do not survive a session, so a saved window can only
be matched by a rule. Creation order is a rule the user can predict.

**FR-038 Default profile applies at sign-in**
Priority: Must
Requirement: When the user signs in, the ScreenState agent shall restore the
profile marked as default.

**FR-039 No default profile**
Priority: Must
Requirement: If no profile is marked as default, then the ScreenState agent
shall apply no profile at sign-in and shall state in the manager that no default
is set.

#### Profile management

**FR-040 Exactly one default**
Priority: Must
Requirement: The ScreenState agent shall hold at most one profile marked as
default; marking a profile as default clears the mark from any other.

**FR-041 Apply on demand**
Priority: Must
Requirement: When the user selects a profile from the tray menu, the ScreenState
agent shall restore that profile immediately, without waiting for settling.
Rationale: during a session the desktop has already settled. Waiting would be a
delay with no purpose.

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
Rationale: a restore can run for minutes while waiting for settling. A wait with
no way out is a hang from the user's point of view.

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
Requirement: When the startup has settled, the ScreenState agent shall complete
the placement of up to 20 windows within 3 seconds.
Method: timestamps in the step log, from the settling event to the last
placement.

**NFR-PERF-002 Quiet period**
Priority: Must
Requirement: The ScreenState agent shall treat 60 seconds with no new top-level
window as the quiet period, configurable by the user between 5 and 600 seconds.
Method: the value is read from configuration and asserted in a test over a fake
clock.
Rationale: measured on the reference machine on 2026-09-19, the longest gap
between windows appearing during startup was 3 minutes 25 seconds, before
Claude's window. A quiet period alone cannot cover that. It does not need to:
FR-020 waits for every application the profile names; the quiet period only
catches stragglers the profile does not name. The first draft of 15 seconds was
replaced by this measurement.

**NFR-PERF-003 Ceiling**
Priority: Must
Requirement: The ScreenState agent shall treat 15 minutes after sign-in as the
ceiling, configurable by the user between 1 and 60 minutes.
Method: as NFR-PERF-002.
Rationale: measured on the reference machine on 2026-09-19, the last startup
window appeared 5 minutes 34 seconds after boot; the owner reports roughly 10
minutes before the machine is usable. The first draft of 5 minutes would
have given up while startup was still running.

**NFR-PERF-004 Settle-check delay**
Priority: Must
Requirement: The ScreenState agent shall re-read each placed window 10 seconds
after placing it, for the check in FR-033.
Method: as NFR-PERF-002. The default is provisional until A-6 is confirmed.

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
Requirement: The ScreenState agent shall reach the point of waiting for settling
within 2 seconds of being started at sign-in.
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
Requirement: A user shall be able to capture and save a profile from a settled
desktop in under 60 seconds, measured from opening the tray menu to the profile
being stored.
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
And Claude, Discord, PigeonPost, NordVPN, GameGlass and Postal Gambit start at
  sign-in by themselves
And Stellody does not start by itself

When the user signs in

Then the agent launches Stellody and launches nothing else
And no window is moved until every one of the seven applications is running,
  every one of the four placed applications has a window and no new window has
  appeared for the quiet period
And Claude is maximised on display 1
And Stellody is maximised on display 4
And Discord is maximised on display 3
And PigeonPost is maximised on display 2
And the NordVPN, GameGlass and Postal Gambit windows are closed
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
| Two restores at once | FR-047 gives one agent per user. A restore requested while one is running is refused with a statement. Recorded as OQ-8, since the behaviour has not been specified. |
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
| OQ-1 | What identifies a display so that the value survives a reboot and tells three displays of the same model apart? Measure: read every available identifier for the four displays, reboot, read again, compare. | Oliver | 2026-09-26 |
| OQ-2 | What identifies an application so that the value survives its updates? Claude runs from a versioned Store directory, Discord from a versioned per-user directory. Measure: read the candidate identifiers for both, then determine how each is launched without naming a versioned path. | Oliver | 2026-09-26 |
| OQ-3 | Can a window be moved and maximised on a target display by a process without administrator rights, across displays with different scaling? Measure: move a window to each of the four displays and read back its position. | Oliver | 2026-09-26 |
| OQ-4 | Does closing the main window of NordVPN, GameGlass and Postal Gambit leave each running? Measure: close each and read the process list. | Oliver | 2026-09-26 |
| OQ-5 | What are the real timings on the reference machine: the span from sign-in to the last startup window; also whether any application moves its own window after appearing? Measure: log window creations and positions from sign-in for 10 minutes, across two reboots. | Oliver | 2026-09-26 |
| OQ-6 | Can an application that starts with no visible window be made to show one from outside it? Decides whether FR-036 stands or is withdrawn. Measure: attempt it against Discord and NordVPN. | Oliver | 2026-09-26 |
| OQ-7 | Where does Discord land after a reboot today; is it the same place each time? Informs FR-033. Measure: record its position across two reboots. | Oliver | 2026-09-26 |
| OQ-8 | What happens when a restore is requested while one is already running? | Oliver | before baselining |
| OQ-9 | Is Windows 10 a supported target? | Oliver | before baselining |
| OQ-10 | What happens when a display is connected or disconnected mid restore? | Oliver | before baselining |
| OQ-11 | Does ScreenState carry an update check, as the owner's other applications do? | Oliver | before baselining |
| OQ-12 | Does ScreenState carry a donation button, as the owner's other released applications do? | Oliver | before baselining |

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

| Priority | Count | Notes |
|---|---|---|
| Must | 50 | The product does not work without any one of them. |
| Should | 8 | FR-014, FR-036, FR-037, FR-049, NFR-PERF-006, NFR-USE-001, NFR-USE-002 and the second half of FR-045. |
| Could | 0 | |
| Won't this time | 8 | OOS-1 to OOS-8. |

The Must proportion is high for a first release of a utility whose whole purpose
is one behaviour. The check that keeps it honest: every Must names a failure the
owner would call the product broken for. Any that does not gets demoted at the
next pass.
