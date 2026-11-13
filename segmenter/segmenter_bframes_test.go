package segmenter

import "testing"

// B 帧重排：PTS 顺序与 DTS 顺序不一致时，处理顺序仍按解码顺序，
// 达到目标时长后必须锁存等待，在下一个关键帧（随机访问点）边界封口。
func TestBuildBFrameReordering(t *testing.T) {
	frames := []Frame{
		{DTS: 0, PTS: 0, Duration: 500, Size: 90, Key: true},
		{DTS: 500, PTS: 1000, Duration: 500, Size: 60},
		{DTS: 1000, PTS: 500, Duration: 500, Size: 60, Key: true},
	}
	got := Build(frames, Config{TargetMs: 1000, MinMs: 500, MaxMs: 2000})

	if got.Work != len(frames) {
		t.Fatalf("work = %d, want frame count %d", got.Work, len(frames))
	}
	if len(got.Segments) < 2 {
		t.Fatalf("expected at least 2 segments, got %+v", got.Segments)
	}
	first := got.Segments[0]
	if first.DurationMs != 1000 || !first.Independent {
		t.Fatalf("first segment = %+v, want duration 1000 independent", first)
	}
	if first.EndDTS != 1000 || first.Bytes != 150 {
		t.Fatalf("first segment boundary = %+v, want end_dts 1000 bytes 150", first)
	}
	second := got.Segments[1]
	if second.StartDTS != 1000 || !second.Independent || second.Bytes != 60 {
		t.Fatalf("second segment = %+v, want keyframe-started independent segment", second)
	}
	var total int64
	for _, s := range got.Segments {
		total += s.Bytes
	}
	if total != 210 {
		t.Fatalf("segment bytes total = %d, want 210", total)
	}
}

// 超过最大时长仍无关键帧：不得在非随机访问点切分、不得丢帧，
// 等到下一个关键帧再封口，等待期间的帧全部保留在当前片段内。
func TestBuildWaitsBeyondMaxDuration(t *testing.T) {
	frames := []Frame{
		{DTS: 0, PTS: 0, Duration: 500, Size: 10, Key: true},
		{DTS: 500, PTS: 500, Duration: 500, Size: 10},
		{DTS: 1000, PTS: 1000, Duration: 500, Size: 10},
		{DTS: 1500, PTS: 1500, Duration: 500, Size: 10},
		{DTS: 2000, PTS: 2000, Duration: 500, Size: 10, Key: true},
	}
	got := Build(frames, Config{TargetMs: 1000, MinMs: 500, MaxMs: 1000})

	if got.Work != 5 {
		t.Fatalf("work = %d, want 5", got.Work)
	}
	if len(got.Segments) != 2 {
		t.Fatalf("segments = %+v, want 2", got.Segments)
	}
	first := got.Segments[0]
	// 最大时长为 1000，但关键帧直到 DTS 2000 才出现：片段必须等下去。
	if first.DurationMs != 2000 || first.EndDTS != 2000 || first.Bytes != 40 {
		t.Fatalf("over-max segment = %+v, want duration 2000 bytes 40", first)
	}
	if !first.Independent || !got.Segments[1].Independent {
		t.Fatalf("both segments start on keyframes: %+v", got.Segments)
	}
	if first.Bytes+got.Segments[1].Bytes != 50 {
		t.Fatalf("bytes lost across segments: %+v", got.Segments)
	}
}

// 始终等不到第二个关键帧时，末尾不完整片段在 EOF flush，帧不丢失。
func TestBuildFlushTrailingSegmentWithoutKeyframe(t *testing.T) {
	frames := []Frame{
		{DTS: 0, PTS: 0, Duration: 500, Size: 10, Key: true},
		{DTS: 500, PTS: 500, Duration: 500, Size: 20},
		{DTS: 1000, PTS: 1000, Duration: 500, Size: 30},
	}
	got := Build(frames, Config{TargetMs: 1000, MinMs: 500, MaxMs: 2000})

	if len(got.Segments) != 1 {
		t.Fatalf("segments = %+v, want single flushed segment", got.Segments)
	}
	s := got.Segments[0]
	if s.Bytes != 60 || s.DurationMs != 1500 || s.EndDTS != 1500 || !s.Independent {
		t.Fatalf("flushed segment = %+v", s)
	}
}

// 最小时长未满足时，即使落在关键帧上也不能封口。
func TestBuildRespectsMinDuration(t *testing.T) {
	frames := []Frame{
		{DTS: 0, PTS: 0, Duration: 500, Size: 10, Key: true},
		{DTS: 500, PTS: 500, Duration: 500, Size: 10, Key: true},
		{DTS: 1000, PTS: 1000, Duration: 500, Size: 10, Key: true},
	}
	got := Build(frames, Config{TargetMs: 1000, MinMs: 700, MaxMs: 2000})

	if len(got.Segments) != 1 {
		t.Fatalf("segments = %+v, want 1 (early keyframe below min_ms)", got.Segments)
	}
	if got.Segments[0].DurationMs != 1000 {
		t.Fatalf("duration = %d, want 1000", got.Segments[0].DurationMs)
	}
}

// 空输入保持既有行为：空片段列表、work 为 0。
func TestBuildEmptyInput(t *testing.T) {
	got := Build(nil, Config{TargetMs: 1000, MinMs: 500, MaxMs: 2000})
	if len(got.Segments) != 0 || got.Work != 0 {
		t.Fatalf("empty input = %+v, want no segments and zero work", got)
	}
}

// 流首帧不是关键帧（中途接入）：首片段不能标记为独立，
// 在关键帧边界封口后，从关键帧起头的后续片段才独立；字节总量守恒。
func TestBuildNonKeyframeStartNotIndependent(t *testing.T) {
	frames := []Frame{
		{DTS: 0, PTS: 0, Duration: 500, Size: 5},
		{DTS: 500, PTS: 500, Duration: 500, Size: 6},
		{DTS: 1000, PTS: 1000, Duration: 500, Size: 8},
		{DTS: 1500, PTS: 1500, Duration: 500, Size: 7, Key: true},
	}
	got := Build(frames, Config{TargetMs: 1000, MinMs: 500, MaxMs: 2000})

	if len(got.Segments) != 2 {
		t.Fatalf("segments = %+v, want 2", got.Segments)
	}
	if got.Segments[0].Independent {
		t.Fatalf("first segment must not be independent: %+v", got.Segments[0])
	}
	if got.Segments[0].Bytes != 19 || got.Segments[1].Bytes != 7 {
		t.Fatalf("byte conservation broken: %+v", got.Segments)
	}
	if got.Segments[1].StartDTS != 1500 || !got.Segments[1].Independent {
		t.Fatalf("second segment should start on the keyframe: %+v", got.Segments[1])
	}
}
