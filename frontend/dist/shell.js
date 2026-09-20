/*
 * The furniture both windows share: the theme toggle, showing one panel at a
 * time, the footer that belongs to the panel showing, the checkbox rows and the
 * repair for a launch that came up with no keyboard.
 *
 * The manager and the setup program each embed their own copy of the page they
 * show, so this file is COPIED into both by build.ps1 and a structural test
 * fails when a copy has drifted from it. Edit this file; never a copy.
 */

const $ = (id) => document.getElementById(id)

function backend() {
    return window.go && window.go.main && window.go.main.App
}

/* ------------------------------------------------------------------ theme */

// applyTheme sets the theme and re-faces the button with the artwork for the
// theme it would switch TO, so the sun shows while you are in the dark. Both happen here: a
// repaint that left the button showing the mode just departed invites a second
// press.
function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme)
    const dark = theme === 'dark'
    $('theme-icon').src = dark ? 'light-mode.png' : 'dark-mode.png'
    $('theme').title = dark ? 'Switch to light' : 'Switch to dark'
}

function currentTheme() {
    return document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'light'
}

$('theme').onclick = () => applyTheme(currentTheme() === 'dark' ? 'light' : 'dark')

/* ----------------------------------------------------------------- panels */

// showOnly displays one panel of a kind and hides the rest. Both windows are a
// stack of panels rather than a set of controls turned on and off: an operation
// moves to a different panel, so there is never a row of greyed boxes.
function showOnly(kind, name) {
    document.querySelectorAll('.' + kind).forEach((el) => el.classList.remove('active'))
    $(kind + '-' + name).classList.add('active')
}

// setFooter rebuilds the row for the panel now showing. It is never one fixed
// row relabelled as it goes: a relabelled row has to remember what it used to
// mean, which is how a go-ahead button on a removal panel kept the styling of a
// safe action. Passing an empty list leaves no buttons at all, which is what a
// panel with nothing safe to offer offers.
function setFooter(buttons) {
    const footer = $('footer')
    footer.innerHTML = ''
    buttons.forEach((spec) => {
        const el = document.createElement('button')
        el.className = 'btn' + (spec.kind ? ' ' + spec.kind : '')
        el.textContent = spec.label
        el.disabled = !!spec.disabled
        el.onclick = spec.onClick
        footer.appendChild(el)
    })
    focusFooter()
}

// focusFooter puts focus on the button a panel leads with, so Enter does the
// obvious thing and the ring says where it would land.
function focusFooter() {
    const footer = $('footer')
    const first = footer.querySelector('.btn.primary:enabled') || footer.querySelector('.btn:enabled')
    if (first) first.focus()
}

// keyboardSettleMs is how long the webview is given to come up holding the
// keyboard before the page decides it has not.
const keyboardSettleMs = 400

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

/* ---------------------------------------------------------------- options */

// renderOptions fills a container with checkboxes and returns a reader for their
// values, so no panel has to know the ids of its own boxes.
function renderOptions(container, specs) {
    container.innerHTML = ''
    const boxes = {}
    specs.forEach((spec) => {
        const label = document.createElement('label')
        label.className = 'option'
        const input = document.createElement('input')
        input.type = 'checkbox'
        input.checked = !!spec.checked
        input.disabled = !!spec.disabled
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

// waitForBackend gives the bindings time to appear, so a window that cannot
// reach its program says so rather than sitting blank.
const backendTries = 100
const backendWaitMs = 50

async function waitForBackend() {
    let tries = 0
    while (!backend() && tries < backendTries) {
        await new Promise((resolve) => setTimeout(resolve, backendWaitMs))
        tries++
    }
    return !!backend()
}
