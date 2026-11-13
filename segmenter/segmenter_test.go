package segmenter

import "testing"

func TestBuildKeyframeSegments(t *testing.T) {
	got := Build([]Frame{{DTS: 0, PTS: 0, Duration: 500, Size: 10, Key: true}, {DTS: 500, PTS: 500, Duration: 500, Size: 12}, {DTS: 1000, PTS: 1000, Duration: 500, Size: 8, Key: true}}, Config{TargetMs: 1000, MinMs: 500, MaxMs: 2000})
	if len(got.Segments) != 1 || got.Segments[0].DurationMs != 1000 {
		t.Fatalf("unexpected segments: %+v", got)
	}
}
