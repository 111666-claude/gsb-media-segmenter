package segmenter

// 片段封口的等待状态。片段只能在合法随机访问点（关键帧）结束，
// 达到目标时长但当前帧不是关键帧时必须进入等待，而不是立即切分。
const (
	waitNone    = iota // 尚未达到目标时长，正常累积
	waitKey            // 已达到目标/最小时长，等待下一个关键帧封口
	waitOverMax        // 等待期间已超过最大时长仍无关键帧：保留状态、不丢帧，
	// 直到下一个关键帧或输入结束，避免在非随机访问点静默切分
)

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

// Build 按解码顺序（DTS）单遍扫描帧序列并按关键帧边界切分。
// B 帧只改变展示顺序（PTS），不改变帧的处理顺序；时长以 PTS 计算，
// 但等待状态一旦置位就锁存，重排帧的 PTS 回退不会把等待条件“冲掉”。
// work 为帧级计数，每帧恰好计一次；已封口的片段不再回头重算。
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
	wait := waitNone

	for i := range frames {
		frame := frames[i]
		result.Work++

		// 新片段首帧：记录解码/展示起点；从关键帧起头才是独立片段。
		if current.Bytes == 0 {
			current.StartDTS = frame.DTS
			startPTS = frame.PTS
			current.Independent = frame.Key
		}
		elapsed := frame.PTS - startPTS

		// 最大时长只提升等待级别，绝不强制在非关键帧处切分或丢帧。
		if wait != waitNone && elapsed > cfg.MaxMs {
			wait = waitOverMax
		}

		switch {
		case wait != waitNone && frame.Key:
			// 等到合法随机访问点：当前片段在关键帧之前封口，
			// 关键帧另起独立片段。时长按 DTS 边界计算。
			// 是否独立沿用当前片段首帧的关键帧标记（流中途开始时为 false）。
			current.EndDTS = frame.DTS
			current.DurationMs = frame.DTS - current.StartDTS
			result.Segments = append(result.Segments, current)
			current = Segment{Number: current.Number + 1}
			wait = waitNone

			// 该关键帧成为新片段的首帧。
			current.StartDTS = frame.DTS
			startPTS = frame.PTS
			current.Independent = true
			current.Bytes = frame.Size
			current.EndDTS = frame.DTS + frame.Duration

		case wait == waitNone && frame.Key && elapsed >= cfg.TargetMs && elapsed >= cfg.MinMs:
			// 目标时长恰好落在关键帧上：沿用既有口径，包含该帧封口。
			// 独立性不在这里翻转——它只由片段首帧是否为关键帧决定，
			// 首帧标记已在片段建立时写入 current.Independent。
			current.Bytes += frame.Size
			current.EndDTS = frame.DTS + frame.Duration
			current.DurationMs = elapsed
			result.Segments = append(result.Segments, current)
			current = Segment{Number: current.Number + 1}

		default:
			// 达到目标与最小时长但不是关键帧：锁存等待，帧照常累积不丢失。
			if wait == waitNone && elapsed >= cfg.TargetMs && elapsed >= cfg.MinMs {
				wait = waitKey
				if elapsed > cfg.MaxMs {
					wait = waitOverMax
				}
			}
			current.Bytes += frame.Size
			current.EndDTS = frame.DTS + frame.Duration
		}

		// 末尾不完整片段必须 flush；封口后为空则不产生空片段。
		if i == len(frames)-1 && current.Bytes > 0 {
			current.DurationMs = current.EndDTS - current.StartDTS
			result.Segments = append(result.Segments, current)
			current = Segment{Number: current.Number + 1}
		}
	}
	return result
}
