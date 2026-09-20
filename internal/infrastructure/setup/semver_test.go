package setup

import "testing"

func TestCompareOrdersVersions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		left     string
		right    string
		expected Relation
	}{
		{"an equal pair is the same", "1.2.3", "1.2.3", Same},
		{"a later major is newer", "2.0.0", "1.9.9", Newer},
		{"an earlier major is older", "1.9.9", "2.0.0", Older},
		{"the minor decides when the major matches", "1.3.0", "1.2.9", Newer},
		{"the patch decides when the rest matches", "1.2.4", "1.2.3", Newer},
		{"a pre-release suffix takes no part", "1.2.3-beta", "1.2.3", Same},
		{"a short version reads its missing fields as zero", "1.2", "1.2.0", Same},
		{"surrounding space is ignored", " 1.2.3 ", "1.2.3", Same},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := Compare(testCase.left, testCase.right); got != testCase.expected {
				t.Errorf("Compare(%q, %q) = %d, want %d",
					testCase.left, testCase.right, got, testCase.expected)
			}
		})
	}
}

// TestAMalformedVersionComparesAsAnEarlyOne holds the ruling that setup never
// stops over a version it cannot read. An unreadable field counts as zero, so
// the worst a bad string does is offer an update.
func TestAMalformedVersionComparesAsAnEarlyOne(t *testing.T) {
	t.Parallel()
	if got := Compare("1.0.0", "not a version"); got != Newer {
		t.Errorf("Compare against an unreadable version = %d, want %d", got, Newer)
	}
	if got := Compare("", ""); got != Same {
		t.Errorf("Compare of two empty versions = %d, want %d", got, Same)
	}
}
