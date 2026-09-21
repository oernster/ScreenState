/*
 * Everything in the manager that is about the product rather than about your
 * profiles: the Help menu, the guide, About and the licence. The update check
 * the menu also offers lives beside this in manager-updates.js.
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
        {label: 'About ' + state.appName, onChoose: () => void openAbout()},
        {label: 'Licence', onChoose: () => void openLicence()},
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

// The trigger toggles, so a second press on an open menu puts it away rather
// than rebuilding it under the pointer.
$('help').onclick = () => {
    if (helpMenuIsOpen()) closeHelpMenu(true)
    else openHelpMenu()
}

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

// openAbout says what this is, who wrote it and what it stands on. Everything
// in it arrives from the program: the version, the author, the copyright and
// the credits all go out of date in a page and cannot in one place.
async function openAbout() {
    let about
    try {
        about = await backend().About()
    } catch (e) {
        showError(String(e))
        return
    }
    $('about-title').textContent = about.name + ' ' + about.version
    $('about-tagline').textContent = about.tagline
    const lines = $('about-lines')
    lines.innerHTML = ''
    aboutLine(lines, 'Version', about.version)
    aboutLine(lines, 'Author', about.author)
    aboutLine(lines, 'Licence', about.licence)
    const copyright = document.createElement('div')
    copyright.className = 'aboutcopyright'
    copyright.textContent = about.copyright
    lines.appendChild(copyright)

    // Credit where it is owed, with the licence each is offered under beside it.
    const credits = $('about-credits')
    credits.innerHTML = ''
    about.credits.forEach((credit) => {
        const row = document.createElement('li')
        const name = document.createElement('span')
        name.textContent = credit.name
        const licence = document.createElement('span')
        licence.className = 'creditlicence'
        licence.textContent = credit.licence
        row.append(name, licence)
        credits.appendChild(row)
    })
    $('about-thanks').textContent = 'Built on Go and the Qt-free Wails, with thanks to'
        + ' their communities and to everyone above.'

    dialog('about', [
        {label: 'Licence', onClick: () => void openLicence()},
        {label: 'Close', kind: 'primary', onClick: closeDialog},
    ])
}

// aboutLine is one labelled fact about this build.
function aboutLine(into, label, value) {
    const line = document.createElement('div')
    const name = document.createElement('span')
    name.className = 'aboutlabel'
    name.textContent = label
    line.append(name, document.createTextNode(value))
    into.appendChild(line)
}

// openLicence shows the licence itself rather than sending the reader to look
// for a file: a licence a program will not show is one nobody reads. The text
// is carried inside the binary and the dialog's body reads it down gently.
async function openLicence() {
    let text
    try {
        text = await backend().LicenceText()
    } catch (e) {
        showError(String(e))
        return
    }
    $('licence-words').textContent = state.appName + ' is free software. You may use it,'
        + ' study it, share it and change it; anything you pass on carries the same'
        + ' freedoms. It comes with no warranty. The whole of it is below.'
    $('licence-text').textContent = text
    dialog('licence', [
        {label: 'Back', onClick: () => void openAbout()},
        {label: 'Close', kind: 'primary', onClick: closeDialog},
    ])
}
