/*
 * The surfaces that interrupt the window: the sheet a dialog puts up over it,
 * the panel that says something went wrong and the panel that holds it still
 * while work runs.
 */

// dismissArmMs is how long a freshly opened dialog ignores a press on its
// backdrop. Without it the second press of a double-click on the control that
// opened the dialog lands on the backdrop that has just appeared underneath the
// pointer, so the dialog flashes open and vanishes.
const dismissArmMs = 400

let dialogArmedAt = 0
let dialogReturnsTo = null

// The dialog's body is the one surface in this window long enough to want
// reading to the reader: the guide runs well past the height of any dialog. The
// cycle is attached once to the body rather than to each sheet, because the body
// is what scrolls; the actions below it are pinned, so Close never drifts away
// while the words move.
const dialogReader = autoScroll($('dialog-body'))

// dialog puts one sheet up over the window that stays where it was. buttons is
// the row along its foot, the same shape the footer takes, so a dialog and a
// panel are described the same way.
function dialog(name, buttons, returnsTo) {
    showOnly('sheet', name)
    setButtons($('dialog-actions'), buttons)
    $('backdrop').hidden = false
    dialogArmedAt = Date.now()
    dialogReturnsTo = returnsTo || document.activeElement
    const first = $('dialog-actions').querySelector('.btn.primary:enabled')
        || $('dialog-actions').querySelector('.btn:enabled')
    if (first) first.focus()
    // After the focus, not before it: the cycle treats the keyboard arriving as
    // a reader taking hold; the dialog's own opening focus is not one.
    dialogReader.restart()
}

// closeDialog puts it away and gives the keyboard back to whatever opened it,
// so the ring is where the reader left it rather than at the top of the window.
function closeDialog() {
    if ($('backdrop').hidden) return
    $('backdrop').hidden = true
    dialogReader.stop()
    const back = dialogReturnsTo
    dialogReturnsTo = null
    // What opened it is usually a menu item, which the next open rebuilds, so
    // the control that is still there is the trigger the menu hangs from.
    if (back && document.contains(back)) back.focus()
    else $('help').focus()
}

function dialogIsOpen() {
    return !$('backdrop').hidden
}

$('dialog-close').onclick = closeDialog

// A press on the backdrop closes it, which is what a dialog on this desktop
// does. A press on the dialog itself is not one: the backdrop is its parent, so
// the press would otherwise reach it on the way up.
$('backdrop').onmousedown = (event) => {
    if (event.target !== $('backdrop')) return
    if (Date.now() - dialogArmedAt < dismissArmMs) return
    closeDialog()
}

// dialogStops are the controls inside the open dialog, in the order they are
// drawn: the cross in its corner, then the row of actions along its foot.
function dialogStops() {
    return Array.from($('dialog').querySelectorAll('button:enabled'))
}

// The keyboard while a dialog is open. Escape closes it, which is the one key
// every dialog on this desktop answers. Tab and Shift+Tab walk its own controls
// and WRAP: a window behind a dialog is not to be reached over the top of it, so
// a ring that leaves the dialog leaves the reader pressing Tab at a window that
// cannot answer. The page behind takes the ring back when the dialog closes.
document.addEventListener('keydown', (event) => {
    if (!dialogIsOpen()) return
    if (event.key === 'Escape') {
        event.preventDefault()
        closeDialog()
        return
    }
    if (event.key !== 'Tab') return
    const stops = dialogStops()
    if (!stops.length) return
    event.preventDefault()
    const at = stops.indexOf(document.activeElement)
    const step = event.shiftKey ? -1 : 1
    // An unknown starting point means the ring is outside the dialog, so the
    // next press brings it back to the end the reader is travelling towards.
    const next = at < 0
        ? (event.shiftKey ? stops.length - 1 : 0)
        : (at + step + stops.length) % stops.length
    stops[next].focus()
})

/* ------------------------------------------------------------------- error */

function showError(words, back) {
    // An error belongs to the window rather than to the aside that was open, so
    // the dialog goes away rather than the message appearing behind it.
    closeDialog()
    $('error-words').textContent = words
    show('error', [{label: 'Back', kind: 'primary', onClick: back || openProfiles}])
}

/* -------------------------------------------------------------------- busy */

// busy holds the window on a panel that offers nothing while work runs. A
// restore can take minutes: a window that looked ready to take another
// instruction would be lying about what it is doing.
//
// The bar starts empty every time. A bar still showing where the last piece of
// work got to says this one has already made progress it has not made.
function busy(title, words) {
    $('busy-title').textContent = title
    $('busy-words').textContent = words
    $('busy-fill').style.width = '0'
    $('busy-status').textContent = ''
    show('busy', [])
}

// progressEveryMs is how often the busy panel asks how far the restore has got.
// Fast enough that the bar moves while somebody is watching it; slow enough
// that a restore waiting fifteen minutes for a window is not asked a thousand
// times about it.
const progressEveryMs = 500

// watchProgress fills the bar from what the restore has actually settled: the
// entries it has satisfied out of the entries the profile holds (FR-065).
//
// Entries rather than seconds, because seconds are not knowable here: an
// application may put its window up at once or two minutes later, so a bar
// weighted by time would be a guess drawn as a measurement.
//
// It answers a handle to stop it with, which the caller does when the work
// ends. A reading that fails stops the watch rather than being shown or
// swallowed: the piece of work itself reports what went wrong; a bar that stops
// moving is visible where a message written to a console is not.
function watchProgress() {
    let timer = 0
    const stop = () => {
        if (timer) window.clearInterval(timer)
        timer = 0
    }
    const read = () => backend().RestoreProgress().then((reading) => {
        if (!reading || !reading.running) return
        const done = reading.total ? Math.round(100 * reading.satisfied / reading.total) : 0
        $('busy-fill').style.width = done + '%'
        $('busy-status').textContent = reading.satisfied + ' of ' + reading.total
            + (reading.total === 1 ? ' application placed' : ' applications placed')
    }, stop)
    timer = window.setInterval(read, progressEveryMs)
    read()
    return {stop}
}
