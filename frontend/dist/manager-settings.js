/*
 * The settings panel: the two things the product does on its own, starting
 * when you sign in and asking whether a newer version exists.
 */

// drawSettings puts the sign-in setting in the same window as everything else
// (EIR-002, FR-053). It applies immediately: there is no go-ahead button on
// this panel for it to wait on.
function drawSettings() {
    const note = $('settings-note')
    const trouble = [
        state.startupError ? 'The sign-in setting could not be read: ' + state.startupError : '',
        state.updateError ? 'The update setting could not be read: ' + state.updateError : '',
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
}

async function setStartup(enabled) {
    try {
        await backend().SetStartsWithWindows(enabled)
        state.launchOnBoot = enabled
    } catch (e) {
        showError(String(e))
    }
}
