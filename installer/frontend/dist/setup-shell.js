/*
 * What the setup window adds to the shared furniture in shell.js: the licence
 * panel, the choices each screen offers and the work behind the go-ahead.
 */

// appName is the product's name, read from the setup program rather than
// written here. The page renders before it arrives, so the static text carries
// none of it and every screen fills it in from this.
let appName = ''

// currentState is the last reading of the machine, kept so a screen that goes
// back can return to the one that was due. One reading decides the screen, the
// heading, the options and the buttons; nothing works any of that out twice.
let currentState = null

/* ---------------------------------------------------------------- licence */

// The licence is a screen reachable from everywhere, so whatever was due stays
// due behind it. The setup program carries ONE licence, its own; what the agent
// is covered by belongs to the agent.
$('licence').onclick = () => {
    showOnly('screen', 'licence')
    setFooter([{label: 'Back', kind: 'primary', onClick: () => route(currentState)}])
}

// freshChoices are what a first install applies; they are what a reinstall puts back.
// The sign-in entry is among them because that is the whole of what this agent
// does: it arranges the desktop after a sign-in, so an install that did not ask
// Windows to start it would have installed something that never runs.
const freshChoices = {startMenu: true, desktop: true, launchOnBoot: true}

// launchOption finishes every screen that writes files. Setup's job is done once
// the agent is running, so the same tick that starts it also closes setup:
// leaving a spent installer on screen asks for a second dismissal that says
// nothing.
function launchOption() {
    return {
        key: 'launch',
        label: `Start ${appName} and close setup when this finishes`,
        checked: true,
    }
}

// shortcutOptions are the three boxes every screen that offers choices carries.
// Each opens on what is ALREADY TRUE rather than ticked: a user who declined a
// desktop shortcut must not be offered one again as though they had asked for
// it; a reinstall must not put it back in silence.
function shortcutOptions(state) {
    return [
        {
            key: 'startMenu', label: 'Add a Start Menu entry',
            hint: 'Find it by typing its name in the Start Menu.',
            checked: state.installed ? state.startMenu : freshChoices.startMenu,
        },
        {
            key: 'desktop', label: 'Add a Desktop shortcut',
            checked: state.installed ? state.desktop : freshChoices.desktop,
        },
        {
            key: 'boot', label: 'Start it when I sign in',
            hint: 'It waits in the notification area and puts your windows back'
                + ' where the default profile says they go.',
            checked: state.installed ? state.launchOnBoot : freshChoices.launchOnBoot,
        },
    ]
}

/* ------------------------------------------------------------------- work */

function onProgress(p) {
    $('progress-fill').style.width = p.pct + '%'
    $('progress-status').textContent = p.msg
}

// run moves to the progress screen and stays there until the work ends in a
// verdict one way or the other. The progress screen offers no actions at all:
// there is nothing here that can safely be interrupted.
async function run(work, title, doneTitle, doneMsg) {
    $('progress-title').textContent = title
    $('progress-fill').style.width = '0'
    $('progress-status').textContent = 'Starting...'
    setFooter([])
    showOnly('screen', 'progress')
    try {
        await work()
        $('done-title').textContent = doneTitle
        $('done-msg').textContent = doneMsg
        showOnly('screen', 'done')
        setFooter([{label: 'Close', kind: 'primary', onClick: () => backend().Quit()}])
    } catch (e) {
        showError(String(e))
    }
}

// finish runs one piece of work, then honours the launch tick. A successful
// launch closes setup; a failed one leaves the reason on screen, so setup never
// disappears having quietly failed.
function finish(work, wanted, title, doneTitle, doneMsg) {
    return withAppClosed(() => run(
        () => work().then(() => {
            if (wanted) return backend().LaunchApp().then(() => backend().Quit())
        }),
        title, doneTitle, doneMsg + (wanted ? ' It is starting now.' : ''),
    ))
}

function showError(message) {
    $('error-msg').textContent = message
    showOnly('screen', 'error')
    setFooter([{label: 'Close', kind: 'primary', onClick: () => backend().Quit()}])
}

// withAppClosed runs the work once the agent is not running. If it is, the offer
// to close it comes first, before a single file is touched, rather than failing
// later on a locked executable.
async function withAppClosed(proceed) {
    if (!(await backend().AppRunning())) {
        proceed()
        return
    }
    showOnly('screen', 'running')
    setFooter([
        {label: 'Cancel', onClick: () => route(currentState)},
        {
            label: 'Close it and continue', kind: 'primary', onClick: async () => {
                setFooter([])
                try {
                    await backend().CloseRunningApp()
                } catch (e) {
                    showError(String(e))
                    return
                }
                proceed()
            },
        },
    ])
}

// install runs one write of the files, whatever the screen called it. The launch
// option is read here so a successful start closes setup rather than leaving it
// waiting on a Close button nobody needs.
function install(read, title, doneTitle, doneMsg) {
    const launchAfter = read('launch')
    const choices = {
        startMenu: read('startMenu'),
        desktop: read('desktop'),
        launchOnBoot: read('boot'),
    }
    return withAppClosed(() => run(
        () => backend().Install(choices).then(() => {
            if (launchAfter) return backend().LaunchApp().then(() => backend().Quit())
        }),
        title, doneTitle,
        doneMsg + (launchAfter ? ' It is starting now.' : ''),
    ))
}
