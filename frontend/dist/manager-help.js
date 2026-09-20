/*
 * Everything in the manager that is about the product rather than about your
 * profiles: the Help menu, the guide, About, the licence and the update check.
 */

// helpItems is what the Help menu offers, in the house order: the guide leads,
// because it is what somebody meeting the window reaches for; the report of the
// last restore is next, being about your desktop rather than about the product;
// then a rule, then the three that are about the product itself.
//
// Check for updates is present and inert while the setting is off, rather than
// absent: a menu that changes shape teaches nobody where anything is, while the
// grey entry says the setting exists.
function helpItems() {
    return [
        {label: 'Guide', onChoose: openGuide},
        {label: 'The last restore', onChoose: openReport},
        {separator: true},
        {label: 'About ' + state.appName, onChoose: openAbout},
        {label: 'Licence', onChoose: openLicence},
        {label: 'Check for updates', onChoose: () => void runUpdateCheck(false), off: !state.updateCheck},
    ]
}

// openHelpMenu drops the menu under its trigger and puts the keyboard on the
// first thing it offers, so a press of Enter does something rather than nothing.
function openHelpMenu() {
    const menu = $('help-menu')
    menu.innerHTML = ''
    helpItems().forEach((item) => menu.appendChild(menuEntry(item)))
    menu.hidden = false
    $('help').setAttribute('aria-expanded', 'true')
    const first = menu.querySelector('.menu-item:enabled')
    if (first) first.focus()
}

// closeHelpMenu puts it away and hands the keyboard back to the trigger, so the
// ring is where the user left it rather than at the top of the window.
function closeHelpMenu(toTrigger) {
    const menu = $('help-menu')
    if (menu.hidden) return
    menu.hidden = true
    $('help').setAttribute('aria-expanded', 'false')
    if (toTrigger) $('help').focus()
}

function helpMenuIsOpen() {
    return !$('help-menu').hidden
}

// menuEntry draws one item; failing that, the rule between two groups of them.
function menuEntry(item) {
    if (item.separator) {
        const rule = document.createElement('div')
        rule.className = 'menu-sep'
        rule.setAttribute('role', 'separator')
        return rule
    }
    const entry = document.createElement('button')
    entry.className = 'menu-item'
    entry.setAttribute('role', 'menuitem')
    entry.textContent = item.label
    entry.disabled = !!item.off
    entry.onclick = () => {
        closeHelpMenu(false)
        item.onChoose()
    }
    return entry
}

// The keyboard inside the menu: the arrows walk it and wrap, Home and End jump
// to the ends, Escape closes it and gives the trigger back the ring. Every key
// it answers is consumed, so nothing behind the menu acts on the same press.
$('help-menu').addEventListener('keydown', (event) => {
    const items = Array.from($('help-menu').querySelectorAll('.menu-item:enabled'))
    if (!items.length) return
    const at = items.indexOf(document.activeElement)
    const step = (to) => {
        event.preventDefault()
        items[(to + items.length) % items.length].focus()
    }
    if (event.key === 'ArrowDown') step(at + 1)
    else if (event.key === 'ArrowUp') step(at - 1)
    else if (event.key === 'Home') step(0)
    else if (event.key === 'End') step(items.length - 1)
    else if (event.key === 'Escape') {
        event.preventDefault()
        closeHelpMenu(true)
    } else if (event.key === 'Tab') {
        // Tab leaves a menu rather than walking it: the arrows are the walk;
        // a ring trapped in a dropdown cannot reach the window behind it.
        closeHelpMenu(true)
    }
})

// A press anywhere else closes it, which is what every menu on this desktop
// does. The trigger is left out: its own handler toggles, so closing here first
// would reopen it on the same press.
document.addEventListener('mousedown', (event) => {
    if (!helpMenuIsOpen()) return
    if ($('help-menu').contains(event.target) || $('help').contains(event.target)) return
    closeHelpMenu(false)
})

// twoLines is a name over a note, which is the shape of every row in this
// window: a heading you scan and a line you read if the heading was not enough.
function twoLines(name, note) {
    const words = document.createElement('span')
    words.className = 'words'
    const title = document.createElement('span')
    title.className = 'name'
    title.textContent = name
    const under = document.createElement('span')
    under.className = 'note wrap'
    under.textContent = note
    words.append(title, under)
    return words
}

/* -------------------------------------------------------------------- guide */

// named fills the product's name and its donation address into a line of the
// guide, so the guide can talk about both without writing either down. The
// address has one home, in the program, exactly as the name does.
function named(words) {
    return words.split('%s').join(state.appName).split('%u').join(state.donateUrl)
}

// guideArt returns the picture for one guide entry.
//
// The drawn ones are built exactly the way this window builds them, from the
// same classes, so the guide cannot come to show a control the window does not
// draw. The rest are the real image files the window loads.
function guideArt(art) {
    if (art === 'badge') return sample('badge', 'Default')
    if (art === 'remove') return sample('rowbtn', 'Remove')
    const picture = document.createElement('img')
    picture.className = 'guideart'
    picture.src = art === 'help' ? 'help.png' : art
    picture.alt = ''
    return picture
}

// sample is one of this window's own controls, drawn for the guide to name.
function sample(className, words) {
    const control = document.createElement('span')
    control.className = className
    control.textContent = words
    return control
}

// openGuide draws the document in guide.js. This panel is a renderer: it knows
// how a section is laid out and nothing about what any of it says.
function openGuide() {
    const rows = $('guide-rows')
    rows.innerHTML = ''
    guideSections.forEach((section) => {
        const heading = document.createElement('div')
        heading.className = 'guidehead'
        heading.textContent = section.heading
        rows.appendChild(heading)
        if (section.intro) rows.appendChild(guideLine(named(section.intro)))
        const entries = section.entries || []
        entries.forEach((entry) => {
            const row = document.createElement('div')
            row.className = 'row'
            row.appendChild(guideArt(entry.art))
            row.appendChild(twoLines(entry.name, named(entry.text)))
            rows.appendChild(row)
        })
        const rules = section.rules || []
        rules.forEach((rule) => {
            const row = document.createElement('div')
            row.className = 'row'
            row.appendChild(twoLines(rule.title, named(rule.text)))
            rows.appendChild(row)
        })
        const paragraphs = section.paragraphs || []
        paragraphs.forEach((words) => rows.appendChild(guideLine(named(words))))
    })
    dialog('guide', [{label: 'Close', kind: 'primary', onClick: closeDialog}])
}

function guideLine(words) {
    const line = document.createElement('div')
    line.className = 'empty'
    line.textContent = words
    return line
}

/* ------------------------------------------------------------ about, licence */

function openAbout() {
    $('about-title').textContent = state.appName + ' ' + state.version
    $('about-words').textContent = state.tagline + '. It records where your windows'
        + ' are, puts them back after you sign in and never closes anything.'
    $('about-detail').textContent = 'Copyright Oliver Ernster. Released under the'
        + ' GNU General Public License, version 3.'
    dialog('about', [
        {label: 'Licence', onClick: openLicence},
        {label: 'Close', kind: 'primary', onClick: closeDialog},
    ])
}

function openLicence() {
    $('licence-words').textContent = state.appName + ' is free software, released under'
        + ' the GNU General Public License, version 3. You may use it, study it, share it'
        + ' and change it; anything you pass on carries the same freedoms. It comes with'
        + ' no warranty.'
    $('licence-detail').textContent = 'The full text is in the file named LICENSE, in the'
        + ' folder this program was installed into.'
    dialog('licence', [{label: 'Back', onClick: openAbout}, {label: 'Close', kind: 'primary', onClick: closeDialog}])
}

/* ------------------------------------------------------------------ updates */

// updateDelayMs is how long after the window is ready the check nobody asked
// for waits. Long enough that it never contends with starting up, short enough
// that an offer arrives while the user is still here.
const updateDelayMs = 3000

// The marks the update panel wears: an arrow for an offer, a tick for news that
// is not news and a warning for a question that could not be answered.
const markOffer = '↑'
const markGood = '✓'
const markWarn = '⚠'

// runUpdateCheck asks whether there is a newer version (FR-058).
//
// unbidden is true for the check the agent runs by itself, which says nothing
// unless there is something to say. It is false for one the user pressed a
// button for, which reports every outcome including the two that are not news.
async function runUpdateCheck(unbidden) {
    if (!unbidden) {
        busy('Checking for updates', 'Asking whether a newer version has been released.')
    }
    let found
    try {
        found = await backend().CheckForUpdates(unbidden)
    } catch (e) {
        if (unbidden) return
        showError(String(e), openProfiles)
        return
    }
    if (found.available) {
        if (unbidden) backend().Surface()
        offerUpdate(found, unbidden)
        return
    }
    if (unbidden) return
    reportCheck(found)
}

// reportCheck answers a user who pressed the button. Every outcome is said out
// loud, including the two that are not news: a button that sometimes does
// nothing visible is a button people stop trusting.
function reportCheck(found) {
    if (!found.enabled) {
        showUpdateWord(markWarn, 'The update check is off',
            'Nothing was asked of the network. Turn it back on in the settings to check.')
        return
    }
    if (!found.reached) {
        showUpdateWord(markWarn, 'Could not reach the release feed',
            'Nothing is wrong with what you have installed. Try again later.')
        return
    }
    if (found.skipped) {
        showUpdateWord(markGood, 'Version ' + found.latest + ' is available',
            'You chose to pass over this one. Download it below or leave it.')
        setFooter([
            {label: 'Back', onClick: openProfiles},
            {
                label: 'Download', kind: 'primary',
                onClick: () => backend().OpenInBrowser(found.downloadUrl),
            },
        ])
        return
    }
    showUpdateWord(markGood, 'You are up to date',
        'Version ' + found.current + ' is the newest there is.')
}

// showUpdateWord puts one verdict on the update panel with a Back button, which
// is what every outcome of a check the user asked for ends in.
function showUpdateWord(mark, title, words) {
    $('update-mark').textContent = mark
    $('update-mark').style.color = mark === markGood ? 'var(--ring)' : 'var(--danger)'
    $('update-title').textContent = title
    $('update-words').textContent = words
    dialog('update', [{label: 'Close', kind: 'primary', onClick: closeDialog}])
}

// offerUpdate is the offer itself: download it, pass this one over; or be
// asked again next time.
function offerUpdate(found, unbidden) {
    $('update-mark').textContent = markOffer
    $('update-mark').style.color = 'var(--accent)'
    $('update-title').textContent = state.appName + ' ' + found.latest + ' is available'
    $('update-words').textContent = 'You are running ' + found.current + '.'
    // Every way out of this panel goes to the list, whether the check was asked
    // for from the menu or made itself known once the window was ready.
    const back = closeDialog
    dialog('update', [
        {label: 'Skip this version', onClick: () => void skipVersion(found.latest, back)},
        {label: 'Later', onClick: back},
        {
            label: 'Download', kind: 'primary',
            onClick: () => backend().OpenInBrowser(found.downloadUrl),
        },
    ])
}

async function skipVersion(version, back) {
    try {
        await backend().SkipVersion(version)
    } catch (e) {
        showError(String(e), back)
        return
    }
    back()
}

// setUpdateCheck turns the check on or off (FR-059). Off, nothing is asked of
// the network at all, so the manual entry in Help is switched off with it.
async function setUpdateCheck(enabled) {
    try {
        await backend().SetUpdateCheckEnabled(enabled)
        state.updateCheck = enabled
    } catch (e) {
        showError(String(e))
    }
}

// The trigger toggles, so a second press on an open menu puts it away rather
// than rebuilding it under the pointer.
$('help').onclick = () => {
    if (helpMenuIsOpen()) closeHelpMenu(true)
    else openHelpMenu()
}
