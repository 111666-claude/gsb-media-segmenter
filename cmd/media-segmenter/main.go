package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"example.com/segmenter/segmenter"
)

func main() {
	sample := flag.String("sample", "gop", "sample name")
	flag.Parse()
	var frames []segmenter.Frame
	switch *sample {
	case "gop":
		frames = []segmenter.Frame{{DTS: 0, PTS: 0, Duration: 500, Size: 100, Key: true}, {DTS: 500, PTS: 500, Duration: 500, Size: 70}, {DTS: 1000, PTS: 1000, Duration: 500, Size: 60, Key: true}, {DTS: 1500, PTS: 1500, Duration: 500, Size: 50}}
	case "bframes":
		frames = []segmenter.Frame{{DTS: 0, PTS: 0, Duration: 500, Size: 90, Key: true}, {DTS: 500, PTS: 1000, Duration: 500, Size: 60}, {DTS: 1000, PTS: 500, Duration: 500, Size: 60, Key: true}}
	default:
		fmt.Fprintln(os.Stderr, "unknown sample")
		os.Exit(2)
	}
	data, _ := json.Marshal(segmenter.Build(frames, segmenter.Config{TargetMs: 1000, MinMs: 500, MaxMs: 2000}))
	fmt.Println(string(data))
}
