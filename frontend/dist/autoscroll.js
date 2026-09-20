/*
 * The gentle self-reading cycle for long help content: hold still on open, read
 * down slowly, hold at the tail, rewind fast, repeat; it steps aside the moment
 * the reader takes over.
 *
 * Ported from PigeonPost (frontend/src/autoScroll.ts plus hooks/useAutoScroll.ts),
 * phase for phase and constant for constant. The pace belongs to the house rather
 * than to this window or to any one dialog in it: a surface that seems to want a
 * different pace means the pace is wrong everywhere.
 *
 * The cycle is NOT gated on prefers-reduced-motion. On Windows that query follows
 * the general animation-effects switch, which people turn off for performance
 * rather than for motion, so gating on it would silently remove the feature.
 * Anyone who does not want the page to read itself touches it, which is what the
 * suspension is for.
 */

// tickMs is the clock. Every hold counts down in whole ticks, so a wait and a
// movement have the same granularity.
const tickMs = 40

// startHoldMs is the stillness before the first descent: the reader orients
// before anything moves. It is the opening phase's wait rather than a special
// case, so it costs nothing extra to hold.
const startHoldMs = 5000

// The descent is one pixel every second tick. The divider is a countdown of
// ticks rather than a slower timer, so holds keep their granularity.
const descentPx = 1
const descentTicksPerStep = 2

// bottomHoldMs is long enough to finish reading the tail before the rewind takes
// it away.
const bottomHoldMs = 5000

// rewindPx is a reposition rather than a reading pass, so it travels fast. Never
// read at this pace; never rewind at the reading pace.
const rewindPx = 15

// topHoldMs is the breath before the next pass.
const topHoldMs = 2000

// manualHoldMs is the stillness wanted after any manual reading input before the
// cycle picks up again, from wherever the reader left it. Manual input suspends
// the cycle; it never switches it off.
const manualHoldMs = 2500

// The inputs that count as reading by hand. mousedown covers a press on the
// scrollbar as well as on the words; focusin covers the keyboard arriving, which
// is somebody about to read exactly as a scroll is.
const manualEvents = ['wheel', 'mousedown', 'touchstart', 'keydown', 'focusin']

// initialAutoScrollState opens in the top hold seeded with the start hold, so a
// fresh surface stands still before it first moves.
function initialAutoScrollState() {
    return {phase: 'pauseTop', waitMs: startHoldMs, ticksToStep: descentTicksPerStep}
}

// suspended is what a manual reading input puts the cycle into: a hold it
// resumes from at the reader's own position rather than restarting.
function suspended(cycle) {
    return {phase: 'manual', waitMs: manualHoldMs, ticksToStep: cycle.ticksToStep}
}

// autoScrollTick advances the cycle by one tick and says how far the surface
// should move, already clamped. Content that does not overflow consumes nothing,
// so attaching the cycle to a surface that currently fits is free and correct.
function autoScrollTick(cycle, view) {
    if (view.maxScrollTop <= 0) return {cycle: cycle, delta: 0}
    if (cycle.phase === 'down') return autoScrollDescend(cycle, view)
    if (cycle.phase === 'up') return autoScrollRewind(cycle, view)
    return autoScrollHold(cycle, view)
}

// autoScrollHold counts the current wait down; when it runs out, it chooses the
// direction to resume in: after the bottom hold the rewind; after a manual hold
// whatever is left from where the reader stopped, which for a reader already at
// the very end is the rewind; otherwise the reading pass.
function autoScrollHold(cycle, view) {
    const waitMs = cycle.waitMs - tickMs
    if (waitMs > 0) {
        return {cycle: {phase: cycle.phase, waitMs: waitMs, ticksToStep: cycle.ticksToStep}, delta: 0}
    }
    if (cycle.phase === 'pauseBottom') {
        return {cycle: {phase: 'up', waitMs: 0, ticksToStep: cycle.ticksToStep}, delta: 0}
    }
    if (cycle.phase === 'manual' && view.scrollTop >= view.maxScrollTop) {
        return {cycle: {phase: 'up', waitMs: 0, ticksToStep: cycle.ticksToStep}, delta: 0}
    }
    return {cycle: {phase: 'down', waitMs: 0, ticksToStep: descentTicksPerStep}, delta: 0}
}

// autoScrollDescend advances the reading pass by a pixel every second tick and
// hands over to the bottom hold on arrival.
function autoScrollDescend(cycle, view) {
    const ticksToStep = cycle.ticksToStep - 1
    if (ticksToStep > 0) {
        return {cycle: {phase: cycle.phase, waitMs: cycle.waitMs, ticksToStep: ticksToStep}, delta: 0}
    }
    const remaining = view.maxScrollTop - view.scrollTop
    if (remaining <= descentPx) {
        return {
            cycle: {phase: 'pauseBottom', waitMs: bottomHoldMs, ticksToStep: descentTicksPerStep},
            delta: Math.max(0, remaining),
        }
    }
    return {
        cycle: {phase: cycle.phase, waitMs: cycle.waitMs, ticksToStep: descentTicksPerStep},
        delta: descentPx,
    }
}

// autoScrollRewind travels back at the repositioning pace and hands over to the
// top hold on arrival.
function autoScrollRewind(cycle, view) {
    if (view.scrollTop <= rewindPx) {
        return {
            cycle: {phase: 'pauseTop', waitMs: topHoldMs, ticksToStep: descentTicksPerStep},
            delta: -view.scrollTop,
        }
    }
    return {cycle: cycle, delta: -rewindPx}
}

// autoScroll gives one scrollable element the cycle and answers a handle whose
// restart begins a fresh one. This window keeps its dialog in the page rather
// than building it each time, so the restart is what a mount does elsewhere: the
// start hold is owed to every opening, not only to the first.
function autoScroll(node) {
    let cycle = initialAutoScrollState()
    let reading = false
    manualEvents.forEach((type) => {
        node.addEventListener(type, () => { cycle = suspended(cycle) }, {passive: true})
    })
    window.setInterval(() => {
        // A surface nobody is looking at is FROZEN rather than suspended: the
        // tick returns before any wait is consumed, so the phase, the position
        // and the remaining hold are all exactly where they were.
        if (!reading) return
        const view = {scrollTop: node.scrollTop, maxScrollTop: node.scrollHeight - node.clientHeight}
        const answer = autoScrollTick(cycle, view)
        cycle = answer.cycle
        if (answer.delta !== 0) node.scrollTop = view.scrollTop + answer.delta
    }, tickMs)
    return {
        restart: () => {
            cycle = initialAutoScrollState()
            node.scrollTop = 0
            reading = true
        },
        stop: () => { reading = false },
    }
}
