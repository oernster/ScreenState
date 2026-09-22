package domain

import (
	"errors"
	"strings"
	"testing"
)

// rankedEntries returns one entry per rank, each with one placement holding
// that rank, in the order given.
func rankedEntries(t *testing.T, ranks ...int) []Entry {
	t.Helper()
	paths := []string{stellodyPath, `C:\Windows\notepad.exe`, `C:\Programs\Terminal.exe`}
	if len(ranks) > len(paths) {
		t.Fatalf("only %d applications to rank", len(paths))
	}
	entries := make([]Entry, 0, len(ranks))
	for at, rank := range ranks {
		application := mustIdentity(t, KindPath, paths[at])
		entries = append(entries, Entry{Application: application, Running: true}.
			WithPlacement(placementOn(t, leftMonitorID).WithRank(rank)))
	}
	return entries
}

func TestAProfileWithNoRanksHasNoStackingOrderAndNoFault(t *testing.T) {
	t.Parallel()
	ranked, err := StackingOf(rankedEntries(t, NoRank, NoRank))
	if ranked || err != nil {
		t.Fatalf("got %v, %v; want no order and no fault (FR-088)", ranked, err)
	}
}

func TestDistinctPositiveRanksAreAStackingOrder(t *testing.T) {
	t.Parallel()
	ranked, err := StackingOf(rankedEntries(t, 2, 7, 1))
	if !ranked || err != nil {
		t.Fatalf("got %v, %v; want an order, gaps allowed (FR-086)", ranked, err)
	}
}

func TestRanksThatCannotBeUsedSayWhy(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		ranks []int
		want  error
		words string
	}{
		"shared":       {ranks: []int{1, 1}, want: ErrRankShared, words: "two placements share rank 1"},
		"not positive": {ranks: []int{1, -2}, want: ErrRankNotPositive, words: "holds -2"},
		"missing":      {ranks: []int{1, NoRank, 2}, want: ErrRankMissing, words: "1 of 3 hold none"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ranked, err := StackingOf(rankedEntries(t, test.ranks...))
			if ranked || !errors.Is(err, test.want) {
				t.Fatalf("got %v, %v; want %v (DATA-007)", ranked, err, test.want)
			}
			if !strings.Contains(err.Error(), test.words) {
				t.Fatalf("%q does not say %q", err, test.words)
			}
		})
	}
}

func TestCompactingKeepsTheOrderAndNumbersFromOne(t *testing.T) {
	t.Parallel()
	entries := rankedEntries(t, 9, 4, 12)
	compacted := CompactRanks(entries)
	for at, want := range []int{2, 1, 3} {
		if got := compacted[at].Placements[0].Rank; got != want {
			t.Fatalf("entry %d holds rank %d, want %d", at, got, want)
		}
	}
	if entries[0].Placements[0].Rank != 9 {
		t.Fatal("compacting changed the entries it was given")
	}
}

func TestCompactingLeavesRanksThatCannotBeUsedAlone(t *testing.T) {
	t.Parallel()
	compacted := CompactRanks(rankedEntries(t, 5, 5))
	if compacted[0].Placements[0].Rank != 5 || compacted[1].Placements[0].Rank != 5 {
		t.Fatalf("ranks with no order in them were renumbered: %+v", compacted)
	}
}

func TestStrippingRemovesEveryRankAndOnlyFromTheCopy(t *testing.T) {
	t.Parallel()
	entries := rankedEntries(t, 1, 2)
	stripped := StripRanks(entries)
	if ranked, err := StackingOf(stripped); ranked || err != nil {
		t.Fatalf("stripped entries still rank: %v, %v", ranked, err)
	}
	if entries[1].Placements[0].Rank != 2 {
		t.Fatal("stripping changed the entries it was given")
	}
}
