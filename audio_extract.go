package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type audioPreset struct {
	Label  string
	Format string
	Rate   string
}

func audioPresets() []audioPreset {
	var out []audioPreset
	for _, f := range []struct{ label, ext string }{
		{"MP3", "mp3"}, {"M4A", "m4a"}, {"AAC", "aac"}, {"Opus", "opus"}, {"OGG (Vorbis)", "ogg"},
	} {
		for _, r := range []string{"Best quality (VBR)", "128 kbps", "192 kbps", "320 kbps"} {
			out = append(out, audioPreset{Label: f.label + " — " + r, Format: f.ext, Rate: r})
		}
	}
	out = append(out, audioPreset{Label: "WAV — Lossless", Format: "wav"}, audioPreset{Label: "FLAC — Lossless", Format: "flac"})
	return out
}

func isSupportedMediaPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".m4v", ".mkv", ".webm", ".mov", ".avi", ".mp3", ".m4a", ".aac", ".ogg", ".opus", ".wav", ".flac", ".wma":
		return true
	default:
		return false
	}
}

func hasAudioStream(path string) bool {
	if !isSupportedMediaPath(path) {
		return false
	}
	ffprobe := findTool("ffprobe.exe")
	if ffprobe == "" {
		return false
	}
	cmd := hidden(exec.Command(ffprobe, "-v", "error", "-select_streams", "a:0", "-show_entries", "stream=codec_type", "-of", "csv=p=0", path))
	b, err := cmd.Output()
	return err == nil && strings.Contains(strings.ToLower(string(b)), "audio")
}

func mediaDuration(path string) float64 {
	ffprobe := findTool("ffprobe.exe")
	if ffprobe == "" {
		return 0
	}
	cmd := hidden(exec.Command(ffprobe, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path))
	b, err := cmd.Output()
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	return v
}

// convertAudioFile writes a separate audio file; the downloaded source is never modified.
func convertAudioFile(ctx context.Context, input string, preset audioPreset, progress func(float64)) (string, error) {
	ffmpeg := findTool("ffmpeg.exe")
	if ffmpeg == "" {
		return "", fmt.Errorf("FFmpeg is not installed")
	}
	if !hasAudioStream(input) {
		return "", fmt.Errorf("the downloaded file has no readable audio stream")
	}
	ext := preset.Format
	base := strings.TrimSuffix(input, filepath.Ext(input))
	output := base + "_audio." + ext
	if _, err := os.Stat(output); err == nil {
		for n := 2; ; n++ {
			candidate := fmt.Sprintf("%s_audio_%d.%s", base, n, ext)
			if _, e := os.Stat(candidate); os.IsNotExist(e) {
				output = candidate
				break
			}
		}
	}
	args := []string{"-hide_banner", "-y", "-i", input, "-map", "0:a:0", "-vn"}
	switch ext {
	case "mp3":
		args = append(args, "-c:a", "libmp3lame")
	case "m4a":
		args = append(args, "-c:a", "aac", "-f", "ipod")
	case "aac":
		args = append(args, "-c:a", "aac", "-f", "adts")
	case "opus":
		args = append(args, "-c:a", "libopus")
	case "ogg":
		args = append(args, "-c:a", "libvorbis")
	case "wav":
		args = append(args, "-c:a", "pcm_s16le")
	case "flac":
		args = append(args, "-c:a", "flac")
	default:
		return "", fmt.Errorf("unsupported audio format %q", ext)
	}
	if preset.Rate != "" && preset.Rate != "Best quality (VBR)" {
		rate := strings.TrimSuffix(preset.Rate, " kbps") + "k"
		args = append(args, "-b:a", rate)
	} else if ext == "mp3" || ext == "ogg" || ext == "opus" {
		args = append(args, "-q:a", "0")
	}
	args = append(args, "-progress", "pipe:1", "-nostats", output)
	cmd := hidden(exec.CommandContext(ctx, ffmpeg, args...))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	duration := mediaDuration(input)
	s := bufio.NewScanner(stdout)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(line, "out_time_ms=") && duration > 0 && progress != nil {
			us, _ := strconv.ParseFloat(strings.TrimPrefix(line, "out_time_ms="), 64)
			p := us / (duration * 1000000) * 100
			if p < 0 {
				p = 0
			}
			if p > 99 {
				p = 99
			}
			progress(p)
		}
	}
	if err := s.Err(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = os.Remove(output)
		return "", err
	}
	if err := cmd.Wait(); err != nil {
		_ = os.Remove(output)
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("FFmpeg conversion failed: %s", msg)
	}
	if progress != nil {
		progress(100)
	}
	return output, nil
}
