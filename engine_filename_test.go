package main

import "testing"

func TestIsOpaqueFilename(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"8ec6149d-ccd0-4680-b220-89389674179e.zip", true},
		{"a1b2c3d4e5f678901234567890abcdef.bin", true},
		{"Shamela-Win-1448.8.iso", false},
		{"archive.zip", false},
		{"download", false},
	}
	for _, tc := range cases {
		if got := isOpaqueFilename(tc.name); got != tc.want {
			t.Errorf("isOpaqueFilename(%q)=%v, want %v", tc.name, got, tc.want)
		}
	}
}
