/*
 * The Guide's words, held apart from the panel that draws them so the panel
 * stays a renderer and the words stay one readable document.
 *
 * Two jobs, in this order. It NAMES the furniture, each entry carrying the REAL
 * picture this window draws, so a control somebody has just met can be
 * identified. Then it states the handful of rules the window cannot say for
 * itself: what is kept locally, what this product will never do to a window and
 * what cannot be undone.
 *
 * Never a description in words where the control is a picture; never a
 * similar-looking stand-in for one. A guide showing something other than the
 * icon is worse than no guide.
 *
 * It is deliberately short. Anything a control says for itself is left to the
 * control; a guide nobody finishes explains nothing.
 *
 * Nothing here writes the product's name down: where the words need it, a
 * `%s` stands in and the panel fills it from the state.
 */

const guideSections = [
    {
        heading: 'The bar along the top',
        intro: 'The mark and the name sit at the left, with the way back to the list beside them. The other two are held at the far end. Hover any of them to see its name.',
        entries: [
            {
                art: 'profile.png', name: 'Back to the profiles',
                text: 'returns to the list from wherever you are, so no panel is a dead end.',
            },
            {
                art: 'dark-mode.png', name: 'Light or dark',
                text: 'the toggle shows the mode it will switch INTO, so the moon appears while the window is light.',
            },
            {
                art: 'help', name: 'Help',
                text: 'drops a menu: this guide, the report of the last restore, About, the licence and the check for a newer version.',
            },
        ],
    },
    {
        heading: 'The profiles',
        intro: 'A profile is a list of applications and where their windows belong. The list is down the left; what the selected one arranges is down the right.',
        entries: [
            {
                art: 'profile.png', name: 'A profile',
                text: 'one arrangement of your desktop, with how many applications it covers underneath its name.',
            },
            {
                art: 'badge', name: 'Default',
                text: 'the profile applied after you sign in. Press the marking on any other profile to move it; press it on the marked one to leave none marked, after which nothing is applied at sign-in.',
            },
            {
                art: 'remove', name: 'Remove',
                text: 'takes one application out of the selected profile. The window itself is not touched.',
            },
        ],
    },
    {
        heading: 'The buttons along the bottom',
        intro: 'The donation button is at the left and stays there. The rest belong to whatever the window is showing, so they change as you move about. On the profile list they are these.',
        entries: [
            {
                art: 'donate.png', name: 'Donate',
                text: '%s is free and stays free: no paid tier, no licence key, no feature held back. If it saves you time and you would like to put something in, this opens %u in your browser.',
            },
        ],
        rules: [
            {title: 'Capture the desktop', text: 'reads every window you have open and offers them for review. Nothing is written until you name it and confirm.'},
            {title: 'Rename and Delete', text: 'act on the selected profile. A delete names it and asks first.'},
            {title: 'Apply', text: 'puts the windows of the selected profile back where it says they go, now. Anything the profile does not name is minimised out of the way; nothing is ever closed.'},
            {title: 'Close', text: 'puts the window away. %s keeps running in the notification area.'},
            {title: 'Quit', text: 'ends it altogether, after which nothing is arranged at your next sign-in until you start it again.'},
        ],
    },
    {
        heading: 'What it will and will not do',
        rules: [
            {
                title: 'It never closes a window and never ends a program',
                text: 'a restore moves windows and starts applications that are not running. Nothing it does can lose you unsaved work.',
            },
            {
                title: 'Everything is kept on this machine, under your own account',
                text: 'the profiles, the log and the settings all live together in your local application data. No administrator rights are needed to install it or to run it.',
            },
            {
                title: 'It makes exactly one kind of connection; you can turn it off',
                text: 'the update check asks a public release feed one question and carries nothing about you or your desktop. Turn it off in the settings and %s makes no network connection at all.',
            },
            {
                title: 'Deleting a profile cannot be undone',
                text: 'the confirmation names the profile because that is the only warning there is.',
            },
        ],
    },
    {
        heading: 'If a restore did not do what you expected',
        paragraphs: [
            'Help, then the report, lists every application the last restore satisfied and every one it could not, each with the reason. An application that was never started, a display that is not connected and a window that never appeared are all named there rather than left for you to work out.',
        ],
    },
]
