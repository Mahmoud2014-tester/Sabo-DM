package main

import "testing"

func TestAdaptiveSegmentCount(t *testing.T) {
	const mib = int64(1 << 20)
	tests := []struct {
		name   string
		size   int64
		ranges bool
		max    int
		want   int
	}{
		{"non range server", 256 * mib, false, 32, 1},
		{"small file", mib - 1, true, 32, 1},
		{"one target chunk", 8 * mib, true, 32, 1},
		{"two target chunks", 8*mib + 1, true, 32, 2},
		{"user cap one", 256 * mib, true, 1, 1},
		{"user cap eight", 256 * mib, true, 8, 8},
		{"upper cap thirty two", 1024 * mib, true, 64, 32},
		{"lower bound for invalid cap", 64 * mib, true, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adaptiveSegmentCount(tt.size, tt.ranges, tt.max); got != tt.want {
				t.Fatalf("adaptiveSegmentCount(%d, %v, %d) = %d, want %d", tt.size, tt.ranges, tt.max, got, tt.want)
			}
		})
	}
}

func TestBuildSegmentsCoversFileExactly(t *testing.T) {
	for _, size := range []int64{1, 2, 3, 10, 1024, 8 << 20, (1 << 30) + 17} {
		for _, n := range []int{1, 2, 3, 8, 16, 32, 64} {
			segments := buildSegments(size, n)
			if len(segments) == 0 || len(segments) > 32 {
				t.Fatalf("size=%d n=%d produced %d segments", size, n, len(segments))
			}
			next := int64(0)
			var total int64
			for i, seg := range segments {
				if seg.Start != next || seg.End < seg.Start || seg.Done != 0 {
					t.Fatalf("size=%d n=%d segment %d is invalid: %+v", size, n, i, seg)
				}
				total += seg.End - seg.Start + 1
				next = seg.End + 1
			}
			if total != size || next != size {
				t.Fatalf("size=%d n=%d covers %d bytes, final offset %d", size, n, total, next)
			}
		}
	}
}

func TestValidResumeState(t *testing.T) {
	valid := &State{Size: 10, Segments: []Segment{{Start: 0, End: 4, Done: 3}, {Start: 5, End: 9, Done: 0}}}
	if !validResumeState(valid, 10, true) {
		t.Fatal("expected contiguous resume state to be valid")
	}
	invalid := []*State{
		{Size: 10, Segments: []Segment{{Start: 0, End: 4, Done: 0}, {Start: 4, End: 9, Done: 0}}}, // overlap
		{Size: 10, Segments: []Segment{{Start: 0, End: 9, Done: 11}}},                             // progress overflow
		{Size: 10, Segments: []Segment{{Start: 1, End: 9, Done: 0}}},                              // missing first byte
		{Size: 10, Segments: []Segment{{Start: 0, End: 10, Done: 0}}},                             // beyond EOF
	}
	for i, st := range invalid {
		if validResumeState(st, 10, true) {
			t.Errorf("invalid resume state %d was accepted", i)
		}
	}
	if validResumeState(valid, 10, false) {
		t.Fatal("multi-segment resume state must be rejected when ranges are unavailable")
	}
}
