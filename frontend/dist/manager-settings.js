/*
 * The settings dialog: starting when you sign in, what a restore does with the
 * windows a profile does not name, how long a restore waits for windows that
 * have not appeared and asking whether a newer version exists.
 */

// openSettings puts the settings up over the window (EIR-002, FR-053). Each
// applies immediately: there is no go-ahead button in the dialog for it to wait
// on, so Close is the only action it carries.
function openSettings() {
    drawSettings()
    dialog('settings', [{label: 'Close', kind: 'primary', onClick: closeDialog}])
}

$('settings').onclick = openSettings

// drawSettings fills the dialog from the last reading the program gave.
function drawSettings() {
    const note = $('settings-note')
    const trouble = [
        state.startupError ? 'The sign-in setting could not be read: ' + state.startupError : '',
        state.updateError ? 'The update setting could not be read: ' + state.updateError : '',
        state.closeUnnamedError
            ? 'The setting for windows a profile does not name could not be read: '
                + state.closeUnnamedError
            : '',
        state.ceilingError
            ? 'How long a restore waits could not be read: ' + state.ceilingError
            : '',
    ].filter((line) => line !== '')
    note.textContent = trouble.join('  ')
    note.hidden = trouble.length === 0
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
        {
            key: 'close-unnamed',
            label: 'At sign-in, close the windows a profile does not name',
            hint: 'A restore puts every other window out of the way. Off, they are'
                + ' minimised. On, the restore that runs when you sign in asks them'
                + ' to close instead, which is what pressing the cross does: most'
                + ' applications that start with Windows go to the notification area,'
                + ' while an ordinary one ends and takes anything unsaved with it.'
                + ' One that refuses is minimised instead and the report says which.'
                + ' Apply always minimises, whatever this says.',
            checked: state.closeUnnamed,
            disabled: !!state.closeUnnamedError,
            onChange: (on) => void setCloseUnnamed(on),
        },
        {
            key: 'updates',
            label: 'Check for updates',
            hint: 'Asks a public release feed once per run whether a newer version'
                + ' exists. It carries nothing about you. Turned off, nothing is'
                + ' asked of the network at all.',
            checked: state.updateCheck,
            disabled: !!state.updateError,
            onChange: (on) => void setUpdateCheck(on),
        },
    ])
    drawCeiling()
}

// drawCeiling sets the field for how long a restore waits (NFR-PERF-003). The
// bounds arrive from the program rather than being written here, so the field
// cannot offer a value the program would change.
function drawCeiling() {
    const field = $('ceiling-minutes')
    field.min = String(state.ceilingMinimum)
    field.max = String(state.ceilingMaximum)
    field.value = state.ceilingError ? '' : String(state.ceilingMinutes)
    field.disabled = !!state.ceilingError
    $('ceiling-hint').textContent = 'A restore places each window as it appears. One that'
        + ' never appears is waited for until whichever comes first: your first key press'
        + ' or click; this long after the restore began. Anything from '
        + state.ceilingMinimum + ' to ' + state.ceilingMaximum + ' minutes.'
}

// A change is kept when the field is left or Enter is pressed, not at every
// key: a half-typed "2" on the way to "25" is not a setting.
$('ceiling-minutes').onchange = () => void setCeiling($('ceiling-minutes').value)

// setCeiling records the minutes typed. What comes back is what was kept, which
// differs only where the bounds held it in, so the field shows the truth rather
// than what was typed.
async function setCeiling(typed) {
    const chosen = Math.round(Number(typed))
    if (typed === '' || !Number.isFinite(chosen)) {
        drawCeiling()
        return
    }
    try {
        state.ceilingMinutes = await backend().SetCeilingMinutes(chosen)
    } catch (e) {
        showError(String(e))
        return
    }
    drawCeiling()
}

async function setCloseUnnamed(closing) {
    try {
        await backend().SetCloseUnnamedWindows(closing)
        state.closeUnnamed = closing
    } catch (e) {
        showError(String(e))
    }
}

async function setStartup(enabled) {
    try {
        await backend().SetStartsWithWindows(enabled)
        state.launchOnBoot = enabled
    } catch (e) {
        showError(String(e))
    }
}
