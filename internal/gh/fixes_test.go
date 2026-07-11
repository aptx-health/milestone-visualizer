package gh

import (
	"testing"
)

func TestExtractFixes(t *testing.T) {
	cases := []struct {
		in   string
		want []int
	}{
		{
			"Fixes #42",
			[]int{42},
		},
		{
			"closes #42",
			[]int{42},
		},
		{
			"closes #42, closes #99",
			[]int{42, 99},
		},
		{
			"Fixes #1 fixes #2 closes #3",
			[]int{1, 2, 3},
		},
		{
			"no issue references here",
			nil,
		},
		{
			"",
			nil,
		},
		{
			"Fixes #42 #42 closes #42",
			[]int{42},
		},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := extractFixes(c.in)
			if len(got) != len(c.want) {
				t.Errorf("extractFixes(%q) returned %d results, want %d: %v", c.in, len(got), len(c.want), got)
			}
			for _, n := range c.want {
				found := false
				for _, g := range got {
					if g == n {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("extractFixes(%q) missing %d", c.in, n)
				}
			}
		})
	}
}
