/*
 * The update check (FR-058, FR-059): the one the agent runs by itself once per
 * run, the one the user asks for from Help, the offer and passing a version
 * over. It was the last part of manager-help.js until that file grew too long.
 */

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
