package segmenter

type Frame struct {
	DTS      int64 `json:"dts"`
	PTS      int64 `json:"pts"`
	Duration int64 `json:"duration"`
	Size     int64 `json:"size"`
	Key      bool  `json:"key"`
}

type Config struct {
	TargetMs int64 `json:"target_ms"`
	MinMs    int64 `json:"min_ms"`
	MaxMs    int64 `json:"max_ms"`
}

type Segment struct {
	Number      int   `json:"number"`
	StartDTS    int64 `json:"start_dts"`
	EndDTS      int64 `json:"end_dts"`
	DurationMs  int64 `json:"duration_ms"`
	Bytes       int64 `json:"bytes"`
	Independent bool  `json:"independent"`
}

type Result struct {
	Segments []Segment `json:"segments"`
	Work     int       `json:"work"`
}

func Build(frames []Frame, cfg Config) Result {
	if cfg.TargetMs <= 0 {
		cfg.TargetMs = 1000
	}
	if cfg.MinMs <= 0 {
		cfg.MinMs = cfg.TargetMs / 2
	}
	if cfg.MaxMs <= 0 {
		cfg.MaxMs = cfg.TargetMs * 2
	}
	result := Result{}
	current := Segment{Number: 1}
	var startPTS int64
	for i, frame := range frames {
		result.Work++
		if current.Bytes == 0 {
			current.StartDTS = frame.DTS
			startPTS = frame.PTS
		}
		current.Bytes += frame.Size
		current.EndDTS = frame.DTS + frame.Duration
		elapsed := frame.PTS - startPTS
		if current.Bytes > 0 && elapsed >= cfg.TargetMs && frame.Key && elapsed >= cfg.MinMs {
			current.DurationMs = elapsed
			current.Independent = true
			result.Segments = append(result.Segments, current)
			current = Segment{Number: current.Number + 1}
		}
		if i == len(frames)-1 && current.Bytes > 0 {
			current.DurationMs = current.EndDTS - current.StartDTS
			result.Segments = append(result.Segments, current)
		}
	}
	return result
}
