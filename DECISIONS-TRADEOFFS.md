# Decisions and trade-offs

The deliberate choices ScreenState rests on: what was chosen, what was given
up for it and why. Each entry is the decision as the product makes it today.
The detail behind each one, with the tests that hold it, lives in
[ARCHITECTURE.md](ARCHITECTURE.md) and the specification
([REQUIREMENTS.md](REQUIREMENTS.md)), whose appendix E records the
measurements the requirements rest on.

## The product as a whole

### A desktop described as an end state

A profile says what the desktop should look like, never how to get there.
Nothing in it is a step; a restore drives each entry towards that state on its
own.

- **Rather than:** a recorded script of launches and moves to be replayed.
- **Gains:** an entry is placed the moment its window exists; a restore can be
  stopped, replaced or run again without leaving a sequence half done.
- **Costs:** a profile cannot say that one application must start before
  another.

### Local, per user, on one machine

Profiles, settings and the log are kept in the signed-in user's own local
application data. There is no server, no account and nothing synchronised.

- **Rather than:** profiles shared between machines or between the accounts
  on one.
- **Gains:** nothing to sign in to; everything but the update check works with
  the network off; each user's desktop is their own.
- **Costs:** a profile belongs to one account on one machine. Another machine
  is captured afresh.

### Measured before it was built

The specification was written first and every assumption in it was settled
against the owner's own desktop by a throwaway spike. The spike is gone; its
readings are kept in the specification.

- **Rather than:** building on what Windows is assumed to do.
- **Gains:** designs were settled by a reading rather than an argument; a
  requirement keeps the evidence it rests on.
- **Costs:** what was measured was measured on one machine of four displays.

### What ScreenState deliberately is not

It puts windows where they belong at sign-in or when asked. It does not
restore what is inside an application, Snap groups or virtual desktops. It
does not arrange windows while the user works. It runs on Windows alone.

- **Rather than:** a window manager that acts all the time; a session
  restorer.
- **Gains:** a small surface held to a high bar; nothing fights the user in
  the middle of a session.
- **Costs:** those jobs need other tools. An application that moves its own
  window after a restore is left where it put it.

### Windows 11 supported; Windows 10 not prevented

Supported means tested and there is no Windows 10 machine to test on. Nothing
stops it running there: the one thing it asks of Windows 11 alone, rounded
corners on the splash, is ignored by older Windows.

- **Rather than:** claiming support nobody has tested; refusing to run.
- **Gains:** every claim of support is one a run has borne out.
- **Costs:** anyone on Windows 10 runs it untested.

### No cgo in anything shipped

Both programs are built without C. Windows is reached through Go's own system
package, including the one COM call the product makes.

- **Rather than:** calling Windows through C.
- **Gains:** what ships is built without a C compiler.
- **Costs:** the race detector needs cgo, so the gate runs the whole suite
  twice and a build machine still needs a C compiler.

## What a profile records

### Only what is on screen

A capture offers the applications with a window shown. An application running
only in the notification area is not offered.

- **Rather than:** also offering the applications whose windows are all
  hidden; on the reference machine that list ran to 25, helpers among them.
- **Gains:** ScreenState arranges the screen, not which programs run; the
  review lists only what there is to arrange.
- **Costs:** an application wanted at sign-in with no window has to be started
  some other way.

### Nothing written until the capture is confirmed

The reading of the desktop is held in memory while the user reviews it. An
application left unticked is never saved; a cancelled capture writes nothing.

- **Rather than:** saving the reading and editing it afterwards.
- **Gains:** a capture abandoned half way leaves nothing behind, on disk
  included.
- **Costs:** none recorded.

### The only profile is the default

While there is one profile it is the one applied at sign-in and unmarking it
is refused. A capture saved while nothing is marked takes the marking. With
two or more profiles the marking is the user's.

- **Rather than:** waiting for the user to mark one, which left a captured
  desktop unarranged after a restart.
- **Gains:** the first capture is applied at the next sign-in with no further
  step.
- **Costs:** a single profile cannot be kept without being applied at sign-in,
  short of turning the sign-in start off.

### The normal rectangle, even for a maximised window

A placement records the window's normal rectangle beside its show state.

- **Rather than:** the maximised bounds.
- **Gains:** the normal rectangle is what decides which display maximising
  puts the window on.
- **Costs:** none recorded.

### One file per profile, written atomically

Each profile is a file of its own carrying the format version it was written
in. A write either completes or leaves the old file untouched. A file that
cannot be read is left exactly as it is and named in the manager; so is one in
a newer format.

- **Rather than:** a database; one file for every profile; rewriting in place.
- **Gains:** an unreadable file costs that profile alone; an interrupted write
  leaves the old profile or the new one, never half of either.
- **Costs:** none recorded.

### Profiles kept apart from the program

The profiles sit in the user's own data; the program sits in its own install
folder. Setup asks the agent's own store where the profiles are rather than
knowing it separately.

- **Rather than:** keeping the user's captures beside the program.
- **Gains:** removing ScreenState leaves the captures unless the user asks for
  them to go; what an uninstall offers to clear cannot drift from what the
  agent writes.
- **Costs:** setup carries the store and the rules it depends on.

## Naming things so they survive

### An application is named by its path, with what survives an update beside it

An application is named by the path that starts it, as the user's own
double-click does. A Store package keeps its model id beside the path, since
its path carries a version. An application installed under a versioned folder
is named by its updater, which does not move.

- **Rather than:** the window class, which was measured changing on every
  reboot; naming a packaged application by its model id alone.
- **Gains:** a packaged application is still recognised, started and placed
  after an update has moved its path, without recapturing.
- **Costs:** three kinds of identity, each with its own rules.

### The activation manager as the fallback for a packaged application

Where a packaged application's path no longer starts it, its model id goes to
the route Windows documents for packaged applications, then to the shell only
if that fails.

- **Rather than:** the shell's route alone.
- **Gains:** the documented route first; the worst case is the older route;
  the log names the route that started each application.
- **Costs:** COM written by hand, on a thread of its own.

### A display is known by the identity Windows gives the device

A display is recorded by its device identity, which tells two screens of the
same model apart.

- **Rather than:** the number Settings shows or the device name; both were
  measured disagreeing with each other and with where the screens sit. A model
  code alone would confuse two identical monitors.
- **Gains:** an identity that was measured unchanged after a reboot and that
  tells identical screens apart.
- **Costs:** a person cannot read it, which the next entry answers.

### A person is told which display by where it sits

The manager and the report name a display by its position among those
connected, such as top, left, centre or right. The identity stays a hover away
and in the log. Within one restore each display keeps the name it was first
given.

- **Rather than:** showing the identity; the number Settings shows.
- **Gains:** a position is readable and is worked out from the displays
  connected now, so it cannot go stale; within a report a name means one
  monitor.
- **Costs:** a name can change between restores when a display comes or goes.

### An application is shown by its name, its path a hover away

The manager, the review and the report lead with a name worked out from the
identity: the file's name, the program an updater starts or a package's name.
Everything recorded stays a hover away.

- **Rather than:** the full path on every row; the executable is not what a
  person calls the application.
- **Gains:** applications read as a person calls them; nothing recorded is
  hidden.
- **Costs:** the name comes from the identity, so it is the file's name rather
  than whatever the application calls itself.

### A window title is never written down

Titles are read only to name a window the review could not read. The report
and the log name every window by its application.

- **Rather than:** naming windows by their titles.
- **Gains:** the log of a sign-in cannot say what the user was working on.
- **Costs:** the report says which application, not which of its windows.

## The restore

### Each entry is placed as its own window appears

A restore places an entry the moment the window it names exists, without
waiting for any other entry.

- **Rather than:** holding every placement until the whole profile had
  settled; waiting for a quiet period with no new windows. Both made the restore
  wait for its slowest application and needed a definition of finished that
  Windows cannot supply.
- **Gains:** a slow application delays nothing but itself; no figure that
  differs per machine.
- **Costs:** the desktop is assembled piece by piece while applications are
  still starting.

### The ceiling is a policy and the user's to set

How long a restore waits for windows that have not appeared is the user's
choice in the manager, in whole minutes within fixed bounds, with a generous
default. Each restore reads it as it begins and counts it from that moment.

- **Rather than:** a prediction of how long the machine takes to start; a
  figure compiled in.
- **Gains:** a changed ceiling governs the next restore without a restart.
- **Costs:** an application slower than the ceiling is reported as missing.

### Waiting on events, never on a timer

Between passes a restore waits for Windows to say the desktop changed and for
nothing else. The ceiling and the wait for taskbar flashing to end are the
only timed waits.

- **Rather than:** looking again at an interval.
- **Gains:** the desktop is read when there is something to read.
- **Costs:** a change Windows does not announce goes unheard, so the restore
  listens to the taskbar's notices as well as to the windows; a kind of change
  nobody listens for is seen only at the next change that is heard, the
  user's first key or the ceiling.

### The user's first key or click ends the waiting

Once the user presses a key or clicks, the restore stops waiting for what has
not come and reports it. A click on the splash does not count.

- **Rather than:** carrying on while the user works.
- **Gains:** a restore never competes with somebody who has started work.
- **Costs:** a key pressed early in a slow sign-in leaves the stragglers
  unplaced and the stacking order as it came up.

### Put back once, then left alone

A placed window is read again as the desktop changes and put back once if its
application moved it. After that the restore gives up and says so.

- **Rather than:** holding the window in place for as long as it takes.
- **Gains:** no tug of war with an application over its own window.
- **Costs:** an application that moves its window twice keeps it where it
  wants it.

### A missing display sends its windows to the primary

A placement naming a display that is not connected goes to the primary
display and the substitution is recorded. Every window is then held within
the display it lands on, keeping its size wherever it fits.

- **Rather than:** leaving the window where it was; placing it off screen.
- **Gains:** every window ends up somewhere it can be reached.
- **Costs:** connecting the display later in the session does not move the
  windows back; that needs another Apply.

### A newer restore replaces a running one; nothing is undone

A restore asked for while one is running stops the running one before its
next action. Stopping a restore from the manager does the same without
starting another. Windows already placed stay where they are.

- **Rather than:** queueing restores; putting windows back as they were.
- **Gains:** the newest statement of the desktop is the one that is true;
  no window is moved twice to reach the same end.
- **Costs:** a stopped restore leaves the desktop part arranged, on purpose.

### Only a sign-in arranges the desktop by itself

The default profile is applied only when Windows starts the agent at sign-in.
A start by hand and the start setup makes open the manager and arrange
nothing.

- **Rather than:** restoring on every start, which made each install rearrange
  the desktop and open windows nobody asked for.
- **Gains:** installing or opening ScreenState never moves the windows
  somebody is working in. Apply arranges the desktop whenever it is asked.
- **Costs:** after an install the profile waits for the next sign-in or for
  Apply.

### A hidden window is shown by running the application again

An application holding its window hidden is started again, which signals the
running copy to show and draw its own window. A run not yet answered by a
window is never followed by another.

- **Rather than:** showing the hidden window from outside, which was measured
  producing an empty frame the application was not drawing.
- **Gains:** the application draws its own window; no application is started
  twice by mistake.
- **Costs:** an application this restore started that goes straight to the
  notification area stays there; the report says so.

### More windows only where they cannot surprise

An entry with fewer windows showing than the profile records has its
application run once per missing window. That happens only at sign-in or for
an application this restore started itself.

- **Rather than:** opening the missing windows on every Apply.
- **Gains:** an Apply in the middle of a session keeps the windows the user
  has.
- **Costs:** Apply does not bring back a second window closed in a running
  application.

### Nothing is ever ended; closing is the user's choice, at sign-in only

The agent never terminates a program. Windows the profile does not name are
asked to close only during the sign-in restore and only where the user has
turned that on; it is off until they do. Apply and a start by hand minimise
them whatever the setting says. A window asked to close that stays is
minimised.

- **Rather than:** forbidding closing outright; closing whenever the desktop
  needs tidying. Some applications survive their window closing while others
  are ended by it; nothing about a window says which kind it is.
- **Gains:** applications that live in the notification area can go there at
  sign-in; the user decides the risk, with what it costs stated beside the
  control.
- **Costs:** with the setting on, an ordinary application not in the profile
  ends at sign-in.

### Windows the profile does not name are minimised

Once a restore has settled everything it can, every visible window the
profile does not name is minimised. This product's own windows, hidden
windows and windows already minimised are left alone.

- **Rather than:** leaving applications that start with Windows on top of the
  arrangement.
- **Gains:** the arrangement is what the user sees.
- **Costs:** those applications stay on the taskbar, which is why closing is
  offered.

## The keyboard and the taskbar

### Nothing placed is activated

A window is placed without being activated, maximised windows included.

- **Rather than:** the direct ways to maximise, each of which was measured
  activating the window, lighting its taskbar button and taking the keyboard.
- **Gains:** the keyboard stays where the user is typing; no button is left
  lit.
- **Costs:** each maximised window minimises and comes back, an animation the
  user sees.

### Nothing started asks for the front

Every application started by its path or its updater is started without being
activated.

- **Rather than:** the ordinary way of starting a program. An agent started
  at sign-in holds no right to the foreground, so the request is refused and
  Windows marks the button red.
- **Gains:** applications come up behind the window in front, with plain
  buttons.
- **Costs:** the routes that start a packaged application by its model id
  cannot pass that on and can still go red; they are taken only once an
  update has moved its path.

### Placing keeps a window's place among the others

A window is put back beneath whatever was above it once it has been placed.

- **Rather than:** letting a window restored from minimised come out on top of
  the ones that were in front of it.
- **Gains:** the order a restore finds survives the placing.
- **Costs:** none recorded.

### The recorded stacking order is set once, last

A capture ranks each window among the profile's own windows by walking the
stacking order from the top. A restore stacks them in that order as its last
step, without activation.

- **Rather than:** the order Windows lists windows in, which was measured not
  to be the stacking order; keeping the order after the restore has ended.
- **Gains:** windows outside the profile keep their place relative to it;
  nothing earlier in the restore can undo the order.
- **Costs:** it is skipped once the user has pressed a key or clicked. A
  profile captured before ranks were recorded keeps whatever order the windows
  come up in.

### A click posted to every taskbar

Once a restore has settled, every taskbar is posted a click. Windows draws the
button of an application started at sign-in without its icon on every display
but the first until a taskbar is clicked, with or without ScreenState running.

- **Rather than:** the other answers tried; a repaint, the notice that icons
  have changed and the activation manager were each measured not to cure it.
- **Gains:** every button carries its icon without the user clicking; the
  pointer does not move and no application's window is touched.
- **Costs:** none recorded.

### Taskbar buttons rebuilt after a sign-in

After a sign-in restore the taskbar button of every window it placed is built
afresh, by hiding the window and showing it again without activation, then
putting it back where it was among the others. First the restore waits until
none of those windows has flashed for one full flash series. Any other
restore does this only for applications it started.

- **Rather than:** stopping the flash of each button, which was measured not to
  clear the red a finished series leaves; leaving the desktop marked.
- **Gains:** a desktop assembled at sign-in comes back unmarked, as it was
  recorded.
- **Costs:** each window flickers once; the flash wait holds the end of the
  restore for some seconds.

## Privacy and the network

### One connection: the update check

The only outbound request asks GitHub for the latest published release. It
carries no identifier and nothing about the desktop. Turned off, nothing is
asked of the network at all; the manual check in Help is switched off with it.

- **Rather than:** no check at all; any telemetry.
- **Gains:** no profile, window layout or anything about the desktop leaves
  the machine.
- **Costs:** one request per run while the check is on.

### Once per run, quiet unless there is news

The agent checks once, shortly after it starts. It says nothing unless a
newer release is out. A check the user asks for reports every outcome. A
version it cannot read is never treated as newer. Skipping a version silences
only the check nobody asked for.

- **Rather than:** a check at launch and every day after; one that reports
  every outcome.
- **Gains:** updates are found without nagging; a malformed tag can never
  raise an offer.
- **Costs:** an agent left running for weeks does not look again until it is
  next started.

### Donations go through the browser

The donation button hands an address to the default browser and stops there.

- **Rather than:** a panel of its own; making the request itself.
- **Gains:** the application makes no connection beyond the update check.
- **Costs:** none recorded.

### Plain files, no encryption

Profiles, settings and the log are ordinary files in the user's own folder.

- **Rather than:** encrypting them.
- **Gains:** simple files a person can inspect; the log is where faults the
  tests could not show have been found.
- **Costs:** anyone with access to the account can read the application
  paths, window positions, display identities and profile names.

### The log keeps a set number of whole restores

Each run cuts the log to its most recent restores, each with the header of its
run.

- **Rather than:** starting the log afresh past a size, which could throw away
  the restore being looked into.
- **Gains:** a restore is kept whole or not at all.
- **Costs:** a log of fewer restores is kept whole whatever its size.

## The interface

### An agent in the notification area, a window when asked

Started at sign-in, the agent opens no window and waits in the notification
area. The tray menu applies a profile, captures, opens the report or opens
the manager. A second launch asks the running copy for its manager.

- **Rather than:** a window at every sign-in; several copies running.
- **Gains:** a sign-in arranges the desktop without anything appearing over
  it; there is only ever one agent.
- **Costs:** the manager is reached through the tray or a shortcut.

### A splash on every display, closed by events

A restore puts a splash on every display saying the desktop is being
prepared, then that it is ready. It never takes the keyboard. A click on any
splash closes it; once it says ready, so does the next key press or click
anywhere. It never closes on a timer. Opening the manager takes it down.

- **Rather than:** a notice that times out; no notice.
- **Gains:** the user knows the desktop is still being arranged without the
  keyboard being taken from them.
- **Costs:** a splash that says ready stays until the next key or click. It is
  drawn natively, since the manager is the one window the agent's framework
  gives it.

### Pages with no framework and no build step

Both windows are plain HTML, CSS and JavaScript. Structural tests stand in for
a compiler: a page may read only the fields the program sends, call only what
the program offers and never write the product's name.

- **Rather than:** a front-end framework with a build step.
- **Gains:** nothing to install or build for the pages.
- **Costs:** each record must be held under one variable name of its own; a
  record read under any other name goes unchecked.

### Dialogs inside the page, modal to the window's frame too

About, the guide, the licence, the report and the settings are dialogs over
the manager. While one is open the window refuses a close from its frame;
the cross is greyed to match.

- **Rather than:** panels that replaced the window; a cross that hid the
  window with a dialog still open.
- **Gains:** the window behaves as one with a dialog up.
- **Costs:** modality is held in the program, since a greyed close was
  measured still reaching the window.

### Help as a drop-down; the guide drawn from the real controls

Help is a menu under its button. The guide's entries carry the images and
styles the window itself uses.

- **Rather than:** a panel of rows offering other panels; a made-up menu bar
  in a window that has none; screenshots.
- **Gains:** the guide cannot come to show a control the window does not
  have.
- **Costs:** the guide is laid out from the controls rather than freely.

### Long dialogs read themselves

A long dialog scrolls gently on its own and steps aside the moment the reader
takes over.

- **Rather than:** static pages.
- **Gains:** long text can be read hands free.
- **Costs:** none recorded.

### Progress counted in entries, asked for while it matters

The Applying panel's bar fills with the entries satisfied out of the entries
the profile holds. The page asks for the reading at short intervals while the
panel is up.

- **Rather than:** seconds, which the agent cannot know; a push through every
  layer between the desktop and the page.
- **Gains:** the bar measures something real; no route out through every
  layer was needed.
- **Costs:** the bar can be a moment behind.

### The tray badge is drawn in code

While the last restore left something outstanding, the tray icon carries a
disc in the theme's danger colour, drawn over the product's own artwork.

- **Rather than:** a second piece of artwork.
- **Gains:** one mark; the badge follows the light or dark theme.
- **Costs:** none recorded.

### One home for the palette and the page furniture

The palette and the shared page script are written once and copied into both
windows by the build; a test fails when a copy has drifted. The splash and the
badge read their colours from the same palette.

- **Rather than:** colours written where they are used.
- **Gains:** the manager, setup and the splash cannot quietly stop matching.
- **Costs:** the website repeats the colours it uses by hand.

## Building and installing

### Installed per user, without administrator rights

Everything is written under the user's own folders and registry keys. It
never asks for administrator rights, to install or to run.

- **Rather than:** a machine-wide install.
- **Gains:** no elevation prompt at any point.
- **Costs:** it cannot act on an application running with administrator
  rights; each account installs separately.

### A setup program of its own, carrying the agent

Setup is a second program built the same way as the agent, with the agent
inside it, so one file is the whole distribution. One reading of the machine
decides the route (install, update, go back a version or manage) and
everything on the screen. Every file it unpacks is checked to land inside the
install folder.

- **Rather than:** a generic installer.
- **Gains:** one identity throughout; the screen, its options and its buttons
  cannot drift apart.
- **Costs:** setup is the product's own to maintain. Only installing over the
  same version has been proved on a real machine.

### Setup ends its own agent by name, never by tree

A running agent is asked about before any file is touched. If the user
chooses to close it, it is ended by its executable name.

- **Rather than:** ending a process tree, which decides parentage from
  process ids that are reused and so can end setup itself.
- **Gains:** a locked executable never leaves a half-written install; setup
  never vanishes mid-run.
- **Costs:** none recorded.

### One version string

One file holds the only version written by hand. The build carries it into
both programs and a script stamps it into the site.

- **Rather than:** a version written into the source or the pages.
- **Gains:** a release cannot announce one version in one place and another
  elsewhere.
- **Costs:** the site must be stamped by a script, since a page cannot read
  the file itself.

### Icons committed rather than generated at build

Every image is reduced from its master by a script that is not part of the
build; the results are committed.

- **Rather than:** generating them on every build.
- **Gains:** a clone builds without the imaging library.
- **Costs:** the script has to be run and its output committed whenever the
  artwork changes.

### GPL, with a commercial licence offered

ScreenState is free and open source under GPL-3.0. A commercial licence for
its own code is offered separately.

- **Rather than:** one licence for every use.
- **Gains:** free to everyone; closed-source use has a route.
- **Costs:** none recorded.

## Engineering

### Layers with one place where they meet

The code is split into domain, application, infrastructure and interface,
each depending only inward, with one composition root wiring them by
constructor. Structural tests hold the boundaries.

- **Rather than:** convention alone; a dependency injection framework.
- **Gains:** every rule about what a restore does can be tested with no
  desktop, disk or clock.
- **Costs:** more packages and explicit wiring.

### Portable rules apart from system calls

The rules that are string work, such as naming applications, sit apart from
the calls into Windows, which sit behind stubs on other platforms. The gate
builds the whole product for a platform that is not Windows.

- **Rather than:** one Windows-only package.
- **Gains:** the rules are settled by tests on any machine.
- **Costs:** a stub beside every system call.

### Rulings held as shapes

Nothing above the Windows layer can terminate anything; every timing has one
home; the product's name is written once; nothing is started in front. A
structural test holds each.

- **Rather than:** rules a reader has to remember.
- **Gains:** a later edit cannot undo one of the specification's decisions
  without the suite failing.
- **Costs:** a new way of closing or a new timing has to argue its way past a
  test.

### Every function covered where it means something

Every function in the domain and application layers must be reached by a
test. Infrastructure is tested but not held to a figure.

- **Rather than:** one figure over the whole program, which would need its
  Windows half moving real windows or tests asserting whatever was on screen.
- **Gains:** a function nothing calls in the decision layers is a decision
  nobody made.
- **Costs:** the floor counts functions, not statements; the Windows calls
  rely on use and a few opt-in probes.

### Small files

No source or page file may pass a fixed line limit; one just below it is cut
well below rather than shaved.

- **Rather than:** letting files grow.
- **Gains:** files split at real seams.
- **Costs:** many small files.

### A package that ran no tests fails the gate

The gate lists every package that owns tests and fails naming any that ran
none.

- **Rather than:** trusting the test tool's ok. An anti-virus quarantining a
  test binary makes its package report ok with no test run, which happened
  repeatedly on this project.
- **Gains:** a green gate means every package was exercised.
- **Costs:** none recorded.

### Tests with real parts that never disturb the desktop

There is no mocking library; fakes are written by hand. No test reaches the
network or moves a window it did not make. The few probes that open windows
of their own are skipped unless asked for. Every structural guard but the two
on the setup page was proved by planting a violation.

- **Rather than:** mocks; tests that arrange the desktop of whoever runs them.
- **Gains:** a passing test means the real thing works; anyone can run the
  suite while working.
- **Costs:** moving windows and starting applications are proved by use at
  each sign-in rather than by the gate.
