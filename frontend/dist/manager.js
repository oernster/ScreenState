/*
 * What the manager window adds to the shared furniture in shell.js: the profile
 * list, the entries of the selected profile, the capture review, the report and
 * the settings.
 */

// state is the last reading the program gave, kept so a panel that goes back
// can return to the one that was due.
let state = null

// selected is the profile whose entries are showing, empty for none.
let selected = ''

// profiles is the last list read, so a rename or a delete can find its
// neighbour without asking again.
let profiles = []

/* ------------------------------------------------------------------ panels */

// show moves to a panel and gives it its own footer. Every panel is reached
// through here, so there is one place that knows what each one offers.
function show(name, buttons) {
    showOnly('view', name)
    setFooter(buttons)
}

function showError(words, back) {
    $('error-words').textContent = words
    show('error', [{label: 'Back', kind: 'primary', onClick: back || openProfiles}])
}

// busy holds the window on a panel that offers nothing while work runs. A
// restore can take minutes: a window that looked ready to take another
// instruction would be lying about what it is doing.
function busy(title, words) {
    $('busy-title').textContent = title
    $('busy-words').textContent = words
    show('busy', [])
}

/* ---------------------------------------------------------------- profiles */

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
    await drawEntries()
    drawSettings()
    show('profiles', profileFooter())
}

function profileFooter() {
    const none = selected === ''
    return [
        {label: 'Quit', kind: 'danger', onClick: () => backend().Quit()},
        {label: 'Close', onClick: () => backend().Hide()},
        {label: 'Delete', disabled: none, onClick: () => confirmDelete(selected)},
        {label: 'Rename', disabled: none, onClick: () => renameSelected()},
        {label: 'Capture the desktop', onClick: () => openCapture()},
        {label: 'Apply', kind: 'primary', disabled: none, onClick: () => applySelected()},
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
            void drawEntries()
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

        const badge = document.createElement('button')
        badge.className = 'badge' + (profile.isDefault ? '' : ' quiet')
        badge.textContent = profile.isDefault ? 'Default' : 'Set default'
        badge.title = profile.isDefault
            ? 'Applied after you sign in. Press to stop applying any profile.'
            : 'Apply this profile after you sign in'
        badge.onclick = (event) => {
            event.stopPropagation()
            void toggleDefault(profile)
        }
        row.appendChild(badge)
        rows.appendChild(row)
    })
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

// drawEntries fills the right-hand column with the entries of the selected
// profile (EIR-002).
async function drawEntries() {
    const rows = $('entry-rows')
    rows.innerHTML = ''
    $('detail-title').textContent = selected || 'Nothing selected'
    $('entry-count').textContent = ''
    if (!selected) {
        rows.appendChild(emptyLine('Select a profile to see what it arranges.'))
        return
    }
    let entries
    try {
        entries = await backend().Entries(selected)
    } catch (e) {
        rows.appendChild(emptyLine(String(e)))
        return
    }
    $('entry-count').textContent = entries.length === 1 ? '1 application' : entries.length + ' applications'
    if (!entries.length) {
        rows.appendChild(emptyLine('This profile arranges nothing. Capture the desktop to make one that does.'))
        return
    }
    entries.forEach((entry) => {
        const row = document.createElement('div')
        row.className = 'row'
        const words = document.createElement('span')
        words.className = 'words'
        const name = document.createElement('span')
        name.className = 'name'
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
        rows.appendChild(row)
    })
}

// describe says in words what one entry asks for.
function describe(entry) {
    const where = entry.placements.map((placement) =>
        placement.state + ' at ' + placement.rect).join('; ')
    const running = entry.running ? 'running' : 'not started'
    if (!where) return entry.kind + ', ' + running + ', no window placed'
    return entry.kind + ', ' + running + ', ' + where
}

async function removeEntry(application) {
    try {
        await backend().RemoveEntry(selected, application)
    } catch (e) {
        showError(String(e))
        return
    }
    await openProfiles()
}

/* ---------------------------------------------------------------- settings */

// drawSettings puts the sign-in setting in the same window as everything else
// (EIR-002, FR-053). It applies immediately: there is no go-ahead button on
// this panel for it to wait on.
function drawSettings() {
    const note = $('settings-note')
    if (state.startupError) {
        note.textContent = 'The sign-in setting could not be read: ' + state.startupError
        note.hidden = false
    } else {
        note.hidden = true
    }
    renderOptions($('settings-options'), [
        {
            key: 'boot',
            label: 'Start when I sign in',
            hint: 'It waits in the notification area and puts your windows back'
                + ' where the default profile says they go.',
            checked: state.launchOnBoot,
            disabled: !!state.startupError,
            onChange: (on) => void setStartup(on),
        },
    ])
}

async function setStartup(enabled) {
    try {
        await backend().SetStartsWithWindows(enabled)
        state.launchOnBoot = enabled
    } catch (e) {
        showError(String(e))
    }
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

async function applySelected() {
    const name = selected
    busy('Applying ' + name, 'Windows are being put back where the profile says they go.')
    try {
        await backend().Apply(name)
    } catch (e) {
        showError(String(e))
        return
    }
    openReport()
}

/* ----------------------------------------------------------------- capture */

// openCapture reads the desktop and shows what a capture WOULD record. Nothing
// is written here: a review is a proposal until it is named and confirmed
// (FR-010, FR-011); leaving it writes nothing at all (FR-016).
async function openCapture() {
    busy('Reading the desktop', 'Every window you have open is being looked at.')
    let review
    try {
        review = await backend().Capture('')
    } catch (e) {
        showError(String(e))
        return
    }
    const rows = $('capture-rows')
    rows.innerHTML = ''
    const keeping = {}
    review.entries.forEach((entry) => {
        keeping[entry.application] = true
        const label = document.createElement('label')
        label.className = 'option'
        const input = document.createElement('input')
        input.type = 'checkbox'
        input.checked = true
        input.onchange = () => { keeping[entry.application] = input.checked }
        const tick = document.createElement('span')
        tick.className = 'check'
        const text = document.createElement('span')
        const title = document.createElement('span')
        title.className = 'label'
        title.textContent = entry.application
        const hint = document.createElement('span')
        hint.className = 'hint'
        hint.textContent = entry.kind + ', '
            + (entry.windows === 1 ? '1 window placed' : entry.windows + ' windows placed')
        text.append(title, hint)
        label.append(input, tick, text)
        rows.appendChild(label)
    })
    if (!review.entries.length) {
        rows.appendChild(emptyLine('Nothing on the desktop could be recorded.'))
    }
    const unreadable = $('capture-unreadable')
    unreadable.hidden = review.unreadable.length === 0
    if (!unreadable.hidden) {
        unreadable.textContent = 'Could not be read, so they are left out: '
            + review.unreadable.join(', ')
    }

    const field = $('capture-name')
    field.value = ''
    show('capture', [
        {label: 'Cancel', onClick: () => { backend().CancelCapture(); void openProfiles() }},
        {
            label: 'Save profile', kind: 'primary',
            onClick: () => void saveCapture(field.value, keeping),
        },
    ])
    field.focus()
}

async function saveCapture(name, keeping) {
    const keep = Object.keys(keeping).filter((application) => keeping[application])
    try {
        await backend().SaveCapture(name, keep, false)
        selected = name.trim()
    } catch (e) {
        showError(String(e), () => show('capture', [
            {label: 'Cancel', onClick: () => { backend().CancelCapture(); void openProfiles() }},
            {
                label: 'Save profile', kind: 'primary',
                onClick: () => void saveCapture($('capture-name').value, keeping),
            },
        ]))
        return
    }
    await openProfiles()
}

/* ------------------------------------------------------------------ report */

// openReport shows what the most recent restore did (FR-044): every entry it
// satisfied and every one it could not, each with the reason.
async function openReport() {
    const report = await backend().Report()
    const rows = $('report-rows')
    rows.innerHTML = ''
    if (!report.held) {
        $('report-title').textContent = 'No restore has run yet'
        $('report-summary').textContent =
            'Apply a profile or sign in with one marked as the default. What happened will then be here.'
        show('report', [{label: 'Back', kind: 'primary', onClick: openProfiles}])
        return
    }
    $('report-title').textContent = 'Last restore: ' + report.profile
    $('report-summary').textContent = report.summary
    report.notes.forEach((note) => rows.appendChild(emptyLine(note)))
    report.entries.forEach((entry) => {
        const row = document.createElement('div')
        row.className = 'row'
        const mark = document.createElement('span')
        mark.className = entry.satisfied ? 'tick' : 'cross'
        mark.textContent = entry.satisfied ? '✓' : '⚠'
        const words = document.createElement('span')
        words.className = 'words'
        const name = document.createElement('span')
        name.className = 'name'
        name.textContent = entry.application
        const note = document.createElement('span')
        note.className = 'note wrap'
        note.textContent = entry.satisfied
            ? (entry.notes.join('; ') || 'put where the profile says it goes')
            : entry.reason
        words.append(name, note)
        row.append(mark, words)
        rows.appendChild(row)
    })
    show('report', [{label: 'Back', kind: 'primary', onClick: openProfiles}])
}

/* ------------------------------------------------------------------ donate */

// openDonate states what FR-060 requires it to state. The wording matters as
// much as the link: nothing is held back, so an ask implying otherwise would be
// false.
function openDonate() {
    $('donate-words').textContent = state.appName + ' is free and stays free.'
        + ' There is no paid tier, no licence key and no feature held back.'
        + ' If it saves you time and you would like to put something in, the'
        + ' page below is where it goes.'
    $('donate-url').textContent = state.donateUrl
    show('donate', [
        {label: 'Back', onClick: openProfiles},
        {
            label: 'Open the page', kind: 'primary',
            onClick: () => window.runtime.BrowserOpenURL(state.donateUrl),
        },
    ])
}

/* -------------------------------------------------------------------- odds */

function emptyLine(words) {
    const line = document.createElement('div')
    line.className = 'empty'
    line.textContent = words
    return line
}

// prefersDark asks the machine, so the window opens in the appearance the rest
// of Windows is wearing rather than in whichever one this page happens to name
// first.
function prefersDark() {
    return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
}

$('donate').onclick = openDonate

// The mark is the product's artwork, written beside this page by the icon
// generator and committed. Nothing stands in for it: where it cannot be loaded
// the badge goes altogether, leaving the name and its tagline to say which
// window this is, rather than a broken image or a drawing of something else.
$('mark').onerror = () => { $('mark').remove() }

async function init() {
    applyTheme(prefersDark() ? 'dark' : 'light')
    if (!(await waitForBackend())) {
        showError('Could not reach the agent.', () => {})
        return
    }
    state = await backend().DetectState(prefersDark())
    document.title = state.appName
    $('brand').textContent = state.appName
    $('tagline').textContent = state.tagline + ' (v' + state.version + ')'
    applyTheme(state.prefersDark ? 'dark' : 'light')

    // The tray says which panel to open on, so the entry a user chose is the
    // one they arrive at rather than always the list.
    window.runtime.EventsOn('open', (view) => {
        if (view === 'capture') { void openCapture(); return }
        if (view === 'report') { void openReport(); return }
        void openProfiles()
    })

    await openProfiles()
    settleKeyboard()
}

window.addEventListener('DOMContentLoaded', init)
