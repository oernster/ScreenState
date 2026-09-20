const $ = (id) => document.getElementById(id)

// appName is the product's name, read from the setup program rather than
// written here. The page renders before it arrives, so the static text carries
// none of it and every screen fills it in from this. Written down, it goes stale
// through a rename with nothing to say so.
let appName = ''

// currentState is the last reading of the machine, kept so a screen that goes
// back can return to the one that was due. One reading decides the screen, the
// heading, the options and the buttons; nothing works any of that out twice.
let currentState = null

function backend() {
    return window.go && window.go.main && window.go.main.App
}

/* ------------------------------------------------------------------ theme */

// applyTheme sets the theme and re-faces the button to the theme it would
// switch TO, so the sun shows while you are in the dark. Both happen here: a
// repaint that left the button showing the mode just departed invites a second
// press.
function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme)
    const dark = theme === 'dark'
    $('theme-sun').classList.toggle('showing', dark)
    $('theme-moon').classList.toggle('showing', !dark)
    $('theme').title = dark ? 'Switch to light' : 'Switch to dark'
}

function currentTheme() {
    return document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'light'
}

$('theme').onclick = () => applyTheme(currentTheme() === 'dark' ? 'light' : 'dark')

/* ---------------------------------------------------------------- screens */

function showScreen(name) {
    document.querySelectorAll('.screen').forEach((el) => el.classList.remove('active'))
    $('screen-' + name).classList.add('active')
}

// setFooter rebuilds the row for the screen now showing. It is never one fixed
// row relabelled as it goes: a relabelled row has to remember what it used to
// mean, which is how a go-ahead button on a removal screen kept the styling of a
// safe action. Passing an empty list leaves no buttons at all, which is what a
// screen with nothing safe to offer offers.
function setFooter(buttons) {
    const footer = $('footer')
    footer.innerHTML = ''
    buttons.forEach((spec) => {
        const el = document.createElement('button')
        el.className = 'btn' + (spec.kind ? ' ' + spec.kind : '')
        el.textContent = spec.label
        el.onclick = spec.onClick
        footer.appendChild(el)
    })
    focusFooter()
}

// keyboardSettleMs is how long the webview is given to come up holding the
// keyboard before the page decides it has not.
const keyboardSettleMs = 400

// focusFooter puts focus on the button a screen leads with, so Enter does the
// obvious thing and the ring says where it would land.
function focusFooter() {
    const footer = $('footer')
    const first = footer.querySelector('.btn.primary') || footer.querySelector('.btn')
    if (first) first.focus()
}

// settleKeyboard repairs a launch that came up with no keyboard at all. See the
// window package for the race it loses; the page is the only thing that can
// tell, because focusing an element is not the same as the document HAVING
// focus.
function settleKeyboard() {
    window.focus()
    focusFooter()
    window.setTimeout(() => {
        if (document.hasFocus()) return
        void backend().TakeKeyboard().then(() => {
            window.focus()
            focusFooter()
        })
    }, keyboardSettleMs)
}

/* ---------------------------------------------------------------- licence */

// The licence is a screen reachable from everywhere, so whatever was due stays
// due behind it. The setup program carries ONE licence, its own; what the agent
// is covered by belongs to the agent.
$('licence').onclick = () => {
    showScreen('licence')
    setFooter([{label: 'Back', kind: 'primary', onClick: () => route(currentState)}])
}

/* ---------------------------------------------------------------- options */

// renderOptions fills a container with checkboxes and returns a reader for their
// values, so no screen has to know the ids of its own boxes.
function renderOptions(container, specs) {
    container.innerHTML = ''
    const boxes = {}
    specs.forEach((spec) => {
        const label = document.createElement('label')
        label.className = 'option'
        const input = document.createElement('input')
        input.type = 'checkbox'
        input.checked = !!spec.checked
        if (spec.onChange) input.onchange = () => spec.onChange(input.checked)
        const tick = document.createElement('span')
        tick.className = 'check'
        const text = document.createElement('span')
        const title = document.createElement('span')
        title.className = 'label'
        title.textContent = spec.label
        text.appendChild(title)
        if (spec.hint) {
            const hint = document.createElement('span')
            hint.className = 'hint'
            hint.textContent = spec.hint
            text.appendChild(hint)
        }
        label.append(input, tick, text)
        container.appendChild(label)
        boxes[spec.key] = input
    })
    return (key) => boxes[key].checked
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
    showScreen('progress')
    try {
        await work()
        $('done-title').textContent = doneTitle
        $('done-msg').textContent = doneMsg
        showScreen('done')
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
    showScreen('error')
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
    showScreen('running')
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
