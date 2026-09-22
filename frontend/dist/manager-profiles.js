/*
 * The profile list and everything done to one profile: the applications it
 * holds, renaming it, deleting it and applying it.
 */

// openProfiles reads the list again and shows it. It reads rather than trusting
// what it held, because a capture, a rename or a delete has just changed it.
async function openProfiles() {
    try {
        profiles = await backend().Profiles()
    } catch (e) {
        showError(String(e), () => show('profiles', profileFooter()))
        return
    }
    if (!profiles.some((profile) => profile.name === selected)) {
        const profile = profiles[0]
        selected = profile ? profile.name : ''
    }
    drawProfiles()
    show('profiles', profileFooter())
    await Promise.all([drawEntries(), drawUnreadable()])
}

// drawUnreadable names, under the list, every profile file that could not be
// read and why (NFR-REL-002, DATA-003). A profile that has quietly stopped
// appearing is the thing a user notices weeks later. The log said so; nobody
// reads a log to find a missing profile.
//
// It is a note rather than an error panel: the rest of the profiles are fine
// and the list stays usable. A store that cannot be checked at all says that
// instead, since saying nothing would read as nothing being wrong.
async function drawUnreadable() {
    const box = $('profile-unreadable')
    let lines
    try {
        // The store's reason already says what is wrong with the file, so it
        // follows the name as it is rather than behind words saying it again.
        lines = (await backend().UnreadableProfiles()).map((unreadableFile) =>
            unreadableFile.file + ': ' + unreadableFile.reason + '.')
    } catch (e) {
        lines = ['The profiles could not be checked for files that cannot be read: '
            + String(e) + '.']
    }
    box.innerHTML = ''
    box.hidden = lines.length === 0
    if (!lines.length) return
    const head = document.createElement('div')
    head.className = 'unreadablehead'
    head.textContent = lines.length === 1
        ? 'One profile file is not listed'
        : lines.length + ' profile files are not listed'
    box.appendChild(head)
    lines.forEach((words) => {
        const line = document.createElement('div')
        line.className = 'unreadableline'
        line.textContent = words
        box.appendChild(line)
    })
    const after = document.createElement('div')
    after.className = 'unreadableline'
    after.textContent = 'Each is left exactly as it is. One written by a newer version'
        + ' is read again once that version is installed.'
    box.appendChild(after)
}

// profileFooter is what the list offers, down the right. Apply leads because it
// is what the window is for; the three that change the profiles follow; Close
// and Quit sit apart at the foot, being about the window rather than the
// profile.
function profileFooter() {
    const none = selected === ''
    return [
        {label: 'Apply', kind: 'primary', disabled: none, onClick: () => applySelected()},
        {label: 'Capture the desktop', onClick: () => openCapture()},
        {label: 'Rename', disabled: none, onClick: () => renameSelected()},
        {label: 'Delete', disabled: none, onClick: () => confirmDelete(selected)},
        {separator: true},
        {label: 'Close', onClick: () => backend().Hide()},
        {label: 'Quit', kind: 'danger', onClick: () => backend().Quit()},
    ]
}

// drawProfiles fills the list. The default marking is a badge rather than a
// tick in a box, because it belongs to the profile rather than being a control
// the row carries.
function drawProfiles() {
    const rows = $('profile-rows')
    rows.innerHTML = ''
    $('profile-count').textContent = profiles.length
        ? profiles.length + (profiles.length === 1 ? ' profile' : ' profiles')
        : ''
    if (!profiles.length) {
        rows.appendChild(emptyLine('No profiles yet. Capture the desktop to make one.'))
        return
    }
    profiles.forEach((profile) => {
        const row = document.createElement('button')
        row.className = 'row' + (profile.name === selected ? ' selected' : '')
        row.onclick = () => {
            selected = profile.name
            drawProfiles()
            setFooter(profileFooter())
            void drawEntries()
        }
        const art = document.createElement('img')
        art.className = 'art'
        art.src = 'profile.png'
        art.alt = ''
        const words = document.createElement('span')
        words.className = 'words'
        const name = document.createElement('span')
        name.className = 'name'
        name.textContent = profile.name
        const note = document.createElement('span')
        note.className = 'note'
        note.textContent = profile.entries === 1 ? '1 application' : profile.entries + ' applications'
        words.append(name, note)
        row.append(art, words)

        // With one profile the marking is a fact rather than a choice (FR-062),
        // so it is drawn as a label: a button that cannot do anything else is a
        // button that lies about what pressing it would achieve.
        if (profiles.length === 1) {
            const only = document.createElement('span')
            only.className = 'badge'
            only.textContent = '✓ Default'
            only.title = 'The only profile there is, so it is the one applied when you sign in'
            row.appendChild(only)
            rows.appendChild(row)
            return
        }
        const badge = document.createElement('button')
        badge.className = 'badge' + (profile.isDefault ? '' : ' quiet')
        // "Set default" was read as a statement that this one IS the default,
        // which is fair: set is a past participle as readily as a verb. The
        // unmarked wording is an instruction now; the marked one is a state
        // with a tick rather than words that could be read either way.
        badge.textContent = profile.isDefault ? '✓ Default' : 'Make default'
        badge.title = profile.isDefault
            ? 'Applied after you sign in. Press to stop applying any profile.'
            : 'Press to apply this profile after you sign in'
        badge.onclick = (event) => {
            event.stopPropagation()
            void toggleDefault(profile)
        }
        row.appendChild(badge)
        rows.appendChild(row)
    })
    // FR-039 in as many words. A list where nothing is marked looks exactly like
    // a list where something is, so a sign-in that arranges nothing arrives as a
    // surprise: it did on 2026-09-20, after a reboot, with one profile stored.
    if (!profiles.some((profile) => profile.isDefault)) {
        rows.appendChild(emptyLine('None of these is marked, so signing in arranges'
            + ' nothing. Press Make default on the one you want.'))
    }
}

// toggleDefault makes a profile the default; where it already is, it leaves
// none marked. FR-039 says an agent with no default applies nothing at sign-in,
// so having none is a choice a user is entitled to make.
async function toggleDefault(profile) {
    try {
        if (profile.isDefault) {
            await backend().ClearDefault()
        } else {
            await backend().SetDefault(profile.name)
        }
    } catch (e) {
        showError(String(e))
        return
    }
    await openProfiles()
}

/* ----------------------------------------------------------------- entries */

// entryRows fills the column with one row per entry.
//
// The application's name leads, with the file it starts and what it arranges
// beneath in the quieter note style. The whole path and the exact recorded
// rectangle and monitor are still there, on the row's tooltip: what was
// captured stays readable in full (FR-068), it just no longer comes first.
function entryRows(container, entries) {
    container.innerHTML = ''
    if (!entries.length) {
        container.appendChild(emptyLine('This profile arranges nothing.'
            + ' Capture the desktop to make one that does.'))
        return
    }
    entries.forEach((profileEntry) => {
        const row = document.createElement('div')
        row.className = 'row'
        const words = document.createElement('span')
        words.className = 'words'
        words.title = details(profileEntry)
        const name = document.createElement('span')
        name.className = 'name wrap'
        name.textContent = profileEntry.name
        words.appendChild(name)
        describe(profileEntry).forEach((line) => {
            const note = document.createElement('span')
            note.className = 'note wrap'
            note.textContent = line
            words.appendChild(note)
        })
        row.appendChild(words)

        const remove = document.createElement('button')
        remove.className = 'rowbtn'
        remove.textContent = 'Remove'
        remove.title = 'Take this application out of the profile'
        remove.onclick = () => void removeEntry(profileEntry.application)
        row.appendChild(remove)
        container.appendChild(row)
    })
}

// drawEntries shows the selected profile's applications in the wide column
// beside the list (FR-068). They had a dialog of their own while the settings
// shared this screen and crowded them; the settings have a dialog now, so what
// a profile arranges is on screen whenever the profile is.
//
// A reading that arrives after the user has moved to another profile is
// dropped rather than drawn over the one they are looking at.
async function drawEntries() {
    const rows = $('entries-rows')
    const asked = selected
    $('entries-title').textContent = asked || 'Applications'
    if (!asked) {
        $('entries-count').textContent = ''
        rows.innerHTML = ''
        rows.appendChild(emptyLine('Press a profile to see what it arranges.'))
        return
    }
    let entries
    try {
        entries = await backend().Entries(asked)
    } catch (e) {
        showError(String(e))
        return
    }
    if (asked !== selected) return
    $('entries-count').textContent = entries.length === 1
        ? '1 application' : entries.length + ' applications'
    entryRows(rows, entries)
}

// The mark between the parts of one line of a row's note.
const PART = ' · '

// describe says in words what one entry asks for, a line at a time: the file it
// starts and whether it runs, then how its window is shown and how large. One
// placement shares the first line; several take a line each.
function describe(profileEntry) {
    const running = profileEntry.running ? 'running' : 'not started'
    const placements = profileEntry.placements
    if (!placements.length) {
        return [[profileEntry.program, running, 'no window placed'].join(PART)]
    }
    if (placements.length === 1) {
        const placement = placements[0]
        return [[profileEntry.program, running, placement.state].join(PART), placement.size]
    }
    return [[profileEntry.program, running].join(PART)].concat(placements.map((placement) =>
        [placement.state, placement.size].join(PART)))
}

// details is exactly what was recorded for one entry, for the row's tooltip:
// the whole identity, how it is recognised, then every placement's rectangle
// and the monitor it was recorded against.
function details(profileEntry) {
    return [profileEntry.application, 'recognised by ' + profileEntry.kind].concat(
        profileEntry.placements.map((placement) =>
            placement.state + ' at ' + placement.rect + ' on ' + placement.display)).join('\n')
}

// removeEntry takes one application out of the profile, then draws the list and
// the column again, since the count beside the profile has changed as well.
async function removeEntry(application) {
    try {
        await backend().RemoveEntry(selected, application)
    } catch (e) {
        showError(String(e))
        return
    }
    await openProfiles()
}

/* ------------------------------------------------------------------ rename */

// renameSelected asks for the new name on the capture panel's own field, since
// naming a profile is the same act in both places.
function renameSelected() {
    const field = $('capture-name')
    $('capture-rows').innerHTML = ''
    $('capture-unreadable').hidden = true
    field.value = selected
    show('capture', [
        {label: 'Cancel', onClick: openProfiles},
        {label: 'Rename', kind: 'primary', onClick: () => void commitRename(field.value)},
    ])
    field.focus()
    field.select()
}

async function commitRename(to) {
    try {
        await backend().Rename(selected, to)
        selected = to.trim()
    } catch (e) {
        showError(String(e), renameSelected)
        return
    }
    await openProfiles()
}

/* ------------------------------------------------------------------ delete */

// confirmDelete names the profile before anything is removed (FR-043). It is a
// panel rather than a box over the window, so the go-ahead here is a button of
// its own that has never meant anything else.
function confirmDelete(name) {
    $('confirm-title').textContent = 'Delete ' + name + '?'
    $('confirm-words').textContent =
        'The profile and everything it arranges will be gone. It cannot be undone.'
        + ' Nothing on your desktop is closed or moved.'
    show('confirm', [
        {label: 'Keep it', kind: 'primary', onClick: openProfiles},
        {label: 'Delete ' + name, kind: 'danger', onClick: () => void commitDelete(name)},
    ])
}

async function commitDelete(name) {
    try {
        await backend().Delete(name)
        selected = ''
    } catch (e) {
        showError(String(e))
        return
    }
    await openProfiles()
}

/* ----------------------------------------------------------------- restore */

// applySelected restores the selected profile, holding the window on the busy
// panel with its bar until the restore ends (FR-041, FR-065).
//
// The list is drawn again before the report goes up, so the panel behind the
// dialog is the one the user belongs on: closing the report leaves them at the
// profiles rather than at an Applying panel for work that has finished. It is
// done by drawing the list rather than by a callback on the dialog, so leaving
// by the cross, by Escape or by the backdrop all land in the same place.
async function applySelected() {
    const name = selected
    // Stop ends the restore before its next action (FR-049). Apply below then
    // returns with a report that says so, so there is nothing more to do here.
    const stop = () => backend().CancelRestore().catch((e) => showError(String(e)))
    busy('Applying ' + name, 'Windows are being put back where the profile says they go.',
        [{label: 'Stop the restore', onClick: stop}])
    const watching = watchProgress()
    try {
        await backend().Apply(name)
    } catch (e) {
        watching.stop()
        showError(String(e))
        return
    }
    watching.stop()
    await openProfiles()
    openReport()
}
