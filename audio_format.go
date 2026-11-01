package main

import (
	"fmt"
	"strings"
)

// audioFormatArgs validates the requested output format before a queue item is
// accepted. Conversion formats always require a working FFmpeg + FFprobe pair.
func audioFormatArgs(format string) ([]string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" || format == "original" || format == "best" {
		return []string{"-f", "bestaudio/b"}, nil
	}
	if _, err := ffmpegReady(); err != nil {
		return nil, fmt.Errorf("%s conversion requires FFmpeg and FFprobe. Open Settings → Install FFmpeg, then retry", strings.ToUpper(format))
	}
	wanted := map[string]string{
		"mp3": "MP3", "m4a": "M4A", "aac": "AAC", "opus": "Opus", "wav": "WAV", "flac": "FLAC",
	}
	label, ok := wanted[format]
	if !ok {
		return nil, fmt.Errorf("unsupported audio format %q", format)
	}
	for _, option := range audioOptions(true) {
		if strings.EqualFold(option.Label, label) {
			return append([]string(nil), option.Args...), nil
		}
	}
	return nil, fmt.Errorf("audio format %s is not available", label)
}
