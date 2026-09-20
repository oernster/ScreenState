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
        selected = profiles.length ? profiles[0].name : ''
    }
    drawProfiles()
    settleEntriesButton()
    drawSettings()
    show('profiles', profileFooter())
}

function profileFooter() {
    const none = selected === ''
    return [
        {label: 'Capture the desktop', onClick: () => openCapture()},
        {label: 'Rename', disabled: none, onClick: () => renameSelected()},
        {label: 'Delete', disabled: none, onClick: () => confirmDelete(selected)},
        {label: 'Apply', kind: 'primary', disabled: none, onClick: () => applySelected()},
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
            settleEntriesButton()
            setFooter(profileFooter())
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

// settleEntriesButton makes the button in the bar match what there is to open:
// a profile selected on the left is what gives it something to show, so until
// one is it wears the permanent red ring and does nothing when pressed
// (FR-068).
function settleEntriesButton() {
    $('entries').disabled = selected === ''
}

// entryRows fills the dialog with one row per entry.
//
// A name wraps rather than being cut off after a few words: the dialog is the
// one place a profile's applications are shown, so it is the place that has to
// show a whole path.
function entryRows(container, entries) {
    container.innerHTML = ''
    if (!entries.length) {
        container.appendChild(emptyLine('This profile arranges nothing.'
            + ' Capture the desktop to make one that does.'))
        return
    }
    entries.forEach((entry) => {
        const row = document.createElement('div')
        row.className = 'row'
        const words = document.createElement('span')
        words.className = 'words'
        const name = document.createElement('span')
        name.className = 'name wrap'
        name.textContent = entry.application
        const note = document.createElement('span')
        note.className = 'note wrap'
        note.textContent = describe(entry)
        words.append(name, note)
        row.appendChild(words)

        const remove = document.createElement('button')
        remove.className = 'rowbtn'
        remove.textContent = 'Remove'
        remove.title = 'Take this application out of the profile'
        remove.onclick = () => void removeEntry(entry.application)
        row.appendChild(remove)
        container.appendChild(row)
    })
}

// openEntries shows the selected profile's applications in a dialog, which is
// the only place they are shown (FR-068).
//
// They used to fill a column beside the profile list, where they crowded out
// the settings and left a path cut off after a few words. What a profile holds
// is read now and then rather than watched, so it is asked for rather than
// always on screen. It is reached from the bar rather than from the footer,
// because it belongs to the window rather than to the panel that happens to be
// up.
async function openEntries() {
    if (!selected) return
    let entries
    try {
        entries = await backend().Entries(selected)
    } catch (e) {
        showError(String(e))
        return
    }
    $('entries-title').textContent = selected
    $('entries-summary').textContent = entries.length === 1
        ? '1 application' : entries.length + ' applications'
    entryRows($('entries-rows'), entries)
    dialog('entries', [{label: 'Close', kind: 'primary', onClick: closeDialog}])
}

// describe says in words what one entry asks for.
function describe(entry) {
    const where = entry.placements.map((placement) =>
        placement.state + ' at ' + placement.rect).join('; ')
    const running = entry.running ? 'running' : 'not started'
    if (!where) return entry.kind + ', ' + running + ', no window placed'
    return entry.kind + ', ' + running + ', ' + where
}

// removeEntry takes one application out of the profile, from either place the
// rows are shown. Where the dialog is the one they were removed from, it is
// drawn again over the list it has just changed, rather than being left showing
// a row that is no longer there.
async function removeEntry(application) {
    const fromTheDialog = dialogIsOpen() && $('sheet-entries').classList.contains('active')
    try {
        await backend().RemoveEntry(selected, application)
    } catch (e) {
        showError(String(e))
        return
    }
    await openProfiles()
    if (fromTheDialog) await openEntries()
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
    busy('Applying ' + name, 'Windows are being put back where the profile says they go.')
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
