package ui

import "testing"

func TestPermStartingAtIsValidPermutation(t *testing.T) {
	const n = 20
	// Run repeatedly since the order is random; the invariants must always hold.
	for trial := 0; trial < 100; trial++ {
		for start := 0; start < n; start++ {
			perm := permStartingAt(n, start)

			if len(perm) != n {
				t.Fatalf("len = %d, want %d", len(perm), n)
			}
			if perm[0] != start {
				t.Fatalf("perm[0] = %d, want start %d", perm[0], start)
			}
			// Every challenge appears exactly once.
			seen := make([]bool, n)
			for _, v := range perm {
				if v < 0 || v >= n {
					t.Fatalf("value %d out of range", v)
				}
				if seen[v] {
					t.Fatalf("value %d appears twice", v)
				}
				seen[v] = true
			}
		}
	}
}
