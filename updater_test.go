package main

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.3.1", "1.3.0", 1},
		{"1.3.1-dev", "v1.3.1", 0},
		{"v1.30.0", "v1.29.99", 1},
		{"v1.3.0", "v1.3.1", -1},
		{"v2.0.0", "v1.99.99", 1},
	}
	for _, tc := range cases {
		if got := compareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
