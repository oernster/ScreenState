/*
 * The two panels about a desktop rather than about a profile: what a capture
 * would record and what the last restore actually did.
 */

// openCapture reads the desktop and shows what a capture WOULD record. Nothing
// is written here: a review is a proposal until it is named and confirmed
// (FR-010, FR-011); leaving it writes nothing at all (FR-016).
async function openCapture() {
    busy('Reading the desktop', 'Every window you have open is being looked at.')
    let review
    try {
        review = await backend().Capture()
    } catch (e) {
        showError(String(e))
        return
    }
    const rows = $('capture-rows')
    rows.innerHTML = ''
    const keeping = {}
    review.entries.forEach((candidate) => {
        keeping[candidate.application] = true
        const label = document.createElement('label')
        label.className = 'option'
        const input = document.createElement('input')
        input.type = 'checkbox'
        input.checked = true
        input.onchange = () => { keeping[candidate.application] = input.checked }
        const tick = document.createElement('span')
        tick.className = 'check'
        const text = document.createElement('span')
        text.title = candidate.application + '\nrecognised by ' + candidate.kind
        const title = document.createElement('span')
        title.className = 'label'
        title.textContent = candidate.name
        const hint = document.createElement('span')
        hint.className = 'hint'
        hint.textContent = candidate.program + PART
            + (candidate.windows === 0 ? 'no window placed'
                : candidate.windows === 1 ? '1 window placed' : candidate.windows + ' windows placed')
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
    // FR-082: said before anything is saved, so a profile that will not put the
    // windows back in front of one another is not a surprise at the next sign-in.
    const unstacked = $('capture-unstacked')
    unstacked.hidden = review.unstacked === ''
    if (!unstacked.hidden) {
        unstacked.textContent = 'Which window is in front of which will not be recorded: '
            + review.unstacked + '. The windows will be placed but left in whatever order they are in.'
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
        dialog('report', [{label: 'Close', kind: 'primary', onClick: closeDialog}])
        return
    }
    $('report-title').textContent = 'Last restore: ' + report.profile
    $('report-summary').textContent = report.summary
    // Every display below is named by where it sits; hovering the summary says
    // exactly which display each of those words meant.
    $('report-summary').title = report.displays.join('\n')
    report.notes.forEach((note) => rows.appendChild(emptyLine(note)))
    report.entries.forEach((outcome) => {
        const row = document.createElement('div')
        row.className = 'row'
        const mark = document.createElement('span')
        mark.className = outcome.satisfied ? 'tick' : 'cross'
        mark.textContent = outcome.satisfied ? '✓' : '⚠'
        const words = document.createElement('span')
        words.className = 'words'
        words.title = outcome.application
        const name = document.createElement('span')
        name.className = 'name'
        name.textContent = outcome.name
        const note = document.createElement('span')
        note.className = 'note wrap'
        note.textContent = outcome.satisfied
            ? (outcome.notes.join('; ') || 'put where the profile says it goes')
            : outcome.reason
        words.append(name, note)
        row.append(mark, words)
        rows.appendChild(row)
    })
    dialog('report', [{label: 'Close', kind: 'primary', onClick: closeDialog}])
}
