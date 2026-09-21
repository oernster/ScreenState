/*
 * The manager window itself: what every panel shares, the buttons that belong
 * to the window rather than to a panel and the start of a run. Each panel is a
 * file of its own beside this one, loaded after it.
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

/* --------------------------------------------------------------------- bar */

// openDonatePage opens the donation page in the browser (FR-060). It takes the
// user straight there rather than through a panel of its own: a button that
// says what it is does not need a screen asking whether it was meant. What
// FR-060 requires the manager to STATE, that nothing is held back, opens the
// button's own tooltip; init fills it in from freeWords. It is not a line
// under the button because the rail has no height to spare: at the smallest
// window a line there pushed Quit out of sight.
function openDonatePage() {
    window.runtime.BrowserOpenURL(state.donateUrl)
}

$('donate').onclick = openDonatePage

/* ------------------------------------------------------------------- start */

// A failure while a panel is being drawn happens after the call it was waiting
// on has already answered, so it is outside the try that guards the call. Left
// alone it rejects a promise nobody is holding; the window then keeps saying the
// words it was waiting with: a capture with nothing unreadable did exactly that
// on 2026-09-20. Anything that gets this far is a defect, so it says so rather
// than being swallowed.
window.addEventListener('unhandledrejection', (event) => {
    event.preventDefault()
    showError('Something went wrong drawing that: ' + String(event.reason))
})

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
    $('donate').title = named(freeWords) + ' ' + $('donate').title
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

    // FR-058: once per run, a moment after the window is ready. It is silent
    // unless there is something to say.
    window.setTimeout(() => void runUpdateCheck(true), updateDelayMs)
}

window.addEventListener('DOMContentLoaded', init)
