/* ----------------------------------------------------------------- routes */

function routeInstall(state) {
    $('install-title').textContent = `Install ${appName} ${state.thisVersion}`
    $('install-path').textContent = state.installDir
    const read = renderOptions($('install-options'), shortcutOptions(state).concat([
        launchOption(),
    ]))
    showOnly('screen', 'install')
    setFooter([
        {label: 'Cancel', onClick: () => backend().Quit()},
        {
            label: 'Install', kind: 'primary',
            onClick: () => install(read, `Installing ${appName}`,
                `${appName} is installed`,
                'You are on v' + state.thisVersion + '.'),
        },
    ])
}

// routeChange serves both directions of a version change, because an update and
// a step back differ only in wording and in which button is the safe one. The
// heading names neither version: two are in play, so naming one there would
// leave the other unsaid. They go in the flow line instead.
function routeChange(state) {
    const goingBack = state.relation === 'older'
    $('update-title').textContent = goingBack ? 'Go back a version?' : 'Update available'
    $('update-lead').textContent = goingBack
        ? 'This setup file carries an older version than the one installed. Your captured profiles are untouched.'
        : 'A newer version is ready to install. Your captured profiles are untouched.'
    $('update-from').textContent = 'v' + state.installedVersion
    $('update-to').textContent = 'v' + state.thisVersion
    const read = renderOptions($('update-options'), shortcutOptions(state).concat([
        launchOption(),
    ]))
    showOnly('screen', 'update')
    setFooter([
        {label: 'Uninstall', kind: 'danger', onClick: () => routeUninstall(state)},
        {label: 'Not now', onClick: () => backend().Quit()},
        {
            label: goingBack ? 'Go back' : 'Update', kind: 'primary',
            onClick: () => install(read,
                goingBack ? 'Going back a version' : `Updating ${appName}`,
                goingBack ? 'Version changed' : `${appName} is updated`,
                'You are on v' + state.thisVersion + '.'),
        },
    ])
}

// routeManage is the screen for a matching version, where there is nothing to
// install. Its boxes therefore act IMMEDIATELY: a box that only took effect on a
// go-ahead would never take effect at all, since this screen has no go-ahead
// that writes files.
function routeManage(state) {
    $('manage-title').textContent = `${appName} ${state.installedVersion} is installed`
    const live = () => backend().SetShortcuts(read('startMenu'), read('desktop'))
    const read = renderOptions($('manage-options'), [
        {
            key: 'startMenu', label: 'Add a Start Menu entry',
            checked: state.startMenu, onChange: () => live(),
        },
        {
            key: 'desktop', label: 'Add a Desktop shortcut',
            checked: state.desktop, onChange: () => live(),
        },
        {
            key: 'boot', label: 'Start it when I sign in',
            hint: 'It waits in the notification area and puts your windows back'
                + ' where the default profile says they go.',
            checked: state.launchOnBoot,
            onChange: (on) => backend().SetLaunchOnBoot(on),
        },
        launchOption(),
    ])
    showOnly('screen', 'manage')
    setFooter([
        {label: 'Uninstall', kind: 'danger', onClick: () => routeUninstall(state)},
        {label: 'Close', onClick: () => backend().Quit()},
        {
            label: 'Reinstall', onClick: () => finish(
                () => backend().Install(freshChoices), read('launch'),
                `Reinstalling ${appName}`, `${appName} is reinstalled`,
                'The files were written again and the shortcuts put back as a new install would leave them.'),
        },
        {
            label: 'Repair', kind: 'primary',
            onClick: () => finish(
                () => backend().Repair(), read('launch'),
                `Repairing ${appName}`, 'Repair complete',
                'The files have been put back and nothing else was changed.'),
        },
    ])
}

// routeUninstall is a screen rather than a route of its own. It is reachable
// from everywhere and Cancel returns to whatever was behind it, so the run's own
// route never becomes removal; where removal was the only reason setup is open,
// Cancel closes it.
function routeUninstall(state) {
    $('uninstall-state').textContent = state.stateDir
    const read = renderOptions($('uninstall-options'), [
        {
            key: 'state', label: 'Also delete my captured profiles and the log',
            hint: 'Removes everything the agent has saved. It cannot be undone.',
            checked: false,
        },
    ])
    showOnly('screen', 'uninstall')
    setFooter([
        {label: 'Cancel', onClick: () => state.installed ? route(state) : backend().Quit()},
        {
            label: 'Uninstall', kind: 'danger',
            onClick: () => withAppClosed(() => run(
                () => backend().Uninstall(read('state')),
                `Removing ${appName}`, `${appName} is removed`,
                'The agent, its shortcuts and its sign-in entry are gone.')),
        },
    ])
}

// route is the one place the reading of the machine turns into a screen.
function route(state) {
    currentState = state
    if (state.mode === 'uninstall') {
        routeUninstall(state)
    } else if (state.mode !== 'manage') {
        routeInstall(state)
    } else if (state.relation === 'same') {
        routeManage(state)
    } else {
        routeChange(state)
    }
}

async function init() {
    applyTheme('light')
    if (!(await waitForBackend())) {
        showError('Could not reach the setup program.')
        return
    }
    window.runtime.EventsOn('progress', onProgress)
    const state = await backend().DetectState()
    appName = state.appName
    document.title = `${appName} Setup`
    $('brand').textContent = `${appName} Setup`
    $('tagline').textContent = state.tagline
    $('uninstall-title').textContent = `Remove ${appName}?`
    $('running-title').textContent = `${appName} is open`
    applyTheme(state.prefersDark ? 'dark' : 'light')
    route(state)
    settleKeyboard()
}

// The mark is the product's artwork, written beside this page by the icon
// generator and committed. Nothing stands in for it: where it cannot be loaded
// the badge goes altogether, leaving the name and its tagline to say which
// program this is, rather than a broken image or a drawing of something else.
$('mark').onerror = () => { $('mark').remove() }

window.addEventListener('DOMContentLoaded', init)
