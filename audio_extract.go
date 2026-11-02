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
	addRates := func(label, ext string, rates ...string) {
		for _, rate := range rates {
			out = append(out, audioPreset{Label: label + " — " + rate, Format: ext, Rate: rate})
		}
	}
	// Only advertise quality modes that the selected FFmpeg encoder can
	// implement truthfully. AAC and WMA use explicit bitrates, not a VBR
	// quality scale; MP3 and Vorbis expose their encoder quality modes.
	addRates("MP3", "mp3", "Best quality (VBR)", "128 kbps", "192 kbps", "320 kbps")
	addRates("M4A", "m4a", "256 kbps", "128 kbps", "192 kbps", "320 kbps")
	addRates("AAC", "aac", "256 kbps", "128 kbps", "192 kbps", "320 kbps")
	addRates("Opus", "opus", "160 kbps", "128 kbps", "192 kbps", "256 kbps")
	addRates("OGG (Vorbis)", "ogg", "Best quality (VBR)", "128 kbps", "192 kbps")
	addRates("WMA", "wma", "128 kbps", "192 kbps", "320 kbps")
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
	ext := strings.ToLower(strings.TrimSpace(preset.Format))
	switch ext {
	case "mp3", "m4a", "aac", "opus", "ogg", "wav", "flac", "wma":
	default:
		return "", fmt.Errorf("unsupported audio format %q", ext)
	}
	base := strings.TrimSuffix(input, filepath.Ext(input))
	// Reserve a unique destination atomically. A stat-then-write check can race
	// with another process; O_EXCL guarantees we never claim an existing file.
	var output string
	for n := 1; ; n++ {
		candidate := base + "_audio." + ext
		if n > 1 {
			candidate = fmt.Sprintf("%s_audio_%d.%s", base, n, ext)
		}
		f, reserveErr := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if reserveErr == nil {
			if closeErr := f.Close(); closeErr != nil {
				_ = os.Remove(candidate)
				return "", closeErr
			}
			output = candidate
			break
		}
		if !os.IsExist(reserveErr) {
			return "", fmt.Errorf("cannot reserve audio output file: %w", reserveErr)
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
	case "wma":
		args = append(args, "-c:a", "wmav2", "-f", "asf")
	default:
		_ = os.Remove(output)
		return "", fmt.Errorf("unsupported audio format %q", ext)
	}
	if preset.Rate != "" && preset.Rate != "Best quality (VBR)" {
		rate := strings.TrimSuffix(preset.Rate, " kbps") + "k"
		args = append(args, "-b:a", rate)
	} else {
		switch ext {
		case "mp3", "ogg":
			args = append(args, "-q:a", "0")
		case "m4a", "aac":
			args = append(args, "-b:a", "256k")
		case "opus":
			args = append(args, "-b:a", "160k")
		case "wma":
			args = append(args, "-b:a", "192k")
		}
	}
	args = append(args, "-progress", "pipe:1", "-nostats", output)
	cmd := hidden(exec.CommandContext(ctx, ffmpeg, args...))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = os.Remove(output)
		return "", err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		_ = os.Remove(output)
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
	st, statErr := os.Stat(output)
	if statErr != nil || st.IsDir() || st.Size() == 0 || !hasAudioStream(output) {
		_ = os.Remove(output)
		return "", fmt.Errorf("FFmpeg finished but the output file is missing, empty, or has no readable audio stream")
	}
	if progress != nil {
		progress(100)
	}
	return output, nil
}
