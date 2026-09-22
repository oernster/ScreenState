package domain

import (
	"errors"
	"fmt"
	"sort"
)

// A profile's stacking order: which of its windows is drawn over which,
// recorded as a rank on each placement (FR-081, DATA-006). A rank is relative
// to the profile's own windows rather than a place among every window on the
// desktop, because the other windows of the next session are different ones.
//
// Ranks are checked when they are used, never when a profile is read: ranks
// that cannot be used cost the profile its stacking order and nothing else
// (DATA-007), whereas refusing the profile would cost the whole restore for the
// sake of the order alone.

// NoRank is the rank of a placement for which no stacking order was recorded,
// which is every placement of a profile saved before the order was recorded.
const NoRank = 0

// firstRank is the rank of the window on top.
const firstRank = 1

// Why the ranks of a profile cannot be used (DATA-007).
var (
	// ErrRankMissing is a profile where some placements hold a rank and others
	// do not.
	ErrRankMissing = errors.New("some placements hold a rank and others do not")
	// ErrRankNotPositive is a rank below the first.
	ErrRankNotPositive = errors.New("a rank must be a positive whole number")
	// ErrRankShared is two placements holding the same rank, which says two
	// windows are each drawn over the other.
	ErrRankShared = errors.New("two placements share rank")
)

// WithRank returns a copy of the placement holding the given rank.
func (placement Placement) WithRank(rank int) Placement {
	placement.Rank = rank
	return placement
}

// StackingOf answers whether the entries record a stacking order a restore can
// put back (FR-083). It answers false with no error where no placement holds a
// rank, which is an answer rather than a fault: the order a restore finds is
// then kept (FR-088). It answers false with the reason where ranks are there
// but cannot be used (DATA-007).
func StackingOf(entries []Entry) (bool, error) {
	ranked, unranked := 0, 0
	held := make(map[int]bool)
	for _, entry := range entries {
		for _, placement := range entry.Placements {
			if placement.Rank == NoRank {
				unranked++
				continue
			}
			if placement.Rank < firstRank {
				return false, fmt.Errorf("%w; %s holds %d",
					ErrRankNotPositive, entry.Application, placement.Rank)
			}
			if held[placement.Rank] {
				return false, fmt.Errorf("%w %d", ErrRankShared, placement.Rank)
			}
			held[placement.Rank] = true
			ranked++
		}
	}
	if ranked == 0 {
		return false, nil
	}
	if unranked > 0 {
		return false, fmt.Errorf("%w; %d of %d hold none",
			ErrRankMissing, unranked, ranked+unranked)
	}
	return true, nil
}

// CompactRanks returns a copy of the entries whose ranks run 1, 2, 3 in the
// order they already had, so that each is a rank among the placements of the
// profile being saved rather than among the windows the capture read (FR-081).
// Entries whose ranks cannot be used are returned as they are, since there is
// no order in them to keep.
func CompactRanks(entries []Entry) []Entry {
	copied := copyEntries(entries)
	if ranked, _ := StackingOf(copied); !ranked {
		return copied
	}
	type slot struct{ entry, placement int }
	var slots []slot
	for at, entry := range copied {
		for index := range entry.Placements {
			slots = append(slots, slot{entry: at, placement: index})
		}
	}
	rankOf := func(one slot) int { return copied[one.entry].Placements[one.placement].Rank }
	sort.Slice(slots, func(one, two int) bool { return rankOf(slots[one]) < rankOf(slots[two]) })
	for position, one := range slots {
		placements := copied[one.entry].Placements
		placements[one.placement] = placements[one.placement].WithRank(firstRank + position)
	}
	return copied
}

// StripRanks returns a copy of the entries holding no ranks, for a capture
// that could not read the stacking order in full (FR-082).
func StripRanks(entries []Entry) []Entry {
	copied := copyEntries(entries)
	for _, entry := range copied {
		for index := range entry.Placements {
			entry.Placements[index] = entry.Placements[index].WithRank(NoRank)
		}
	}
	return copied
}

// copyEntries copies the entries down to their placements, so a caller holding
// the originals cannot have them changed underneath.
func copyEntries(entries []Entry) []Entry {
	copied := make([]Entry, len(entries))
	for at, entry := range entries {
		entry.Placements = append([]Placement(nil), entry.Placements...)
		copied[at] = entry
	}
	return copied
}
