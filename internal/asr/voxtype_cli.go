package asr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"coe/internal/audio"
)

type VoxtypeCLIClient struct {
	Binary string
	Engine string
}

func (c VoxtypeCLIClient) Name() string {
	engine := strings.TrimSpace(c.Engine)
	if engine == "" {
		return "voxtype-cli"
	}
	return "voxtype-cli-" + engine
}

func (c VoxtypeCLIClient) Transcribe(ctx context.Context, capture audio.Result) (Result, error) {
	wav, err := audio.EncodeWAV(capture)
	if err != nil {
		return Result{}, err
	}

	tmpDir, err := os.MkdirTemp("", "coe-voxtype-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "capture.wav")
	if err := os.WriteFile(inputPath, wav, 0o600); err != nil {
		return Result{}, err
	}

	binary := strings.TrimSpace(c.Binary)
	if binary == "" {
		binary = "voxtype"
	}

	args := []string{"transcribe", inputPath}
	if engine := strings.TrimSpace(c.Engine); engine != "" {
		args = append(args, "--engine", engine)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := joinVoxtypeDetails(stdout.String(), stderr.String())
		if detail != "" {
			return Result{}, fmt.Errorf("voxtype failed: %w (%s)", err, truncateForWarning(detail, 240))
		}
		return Result{}, fmt.Errorf("voxtype failed: %w", err)
	}

	text, warning := parseVoxtypeTranscribeOutput(stdout.String(), stderr.String())
	if text == "" {
		return Result{Warning: warning}, nil
	}
	return Result{Text: text, Warning: warning}, nil
}

func parseVoxtypeTranscribeOutput(stdout, stderr string) (string, string) {
	lines := strings.Split(normalizeVoxtypeNewlines(stdout), "\n")
	lastStatusLine := -1
	noSpeech := false
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if isVoxtypeStatusLine(line) {
			lastStatusLine = i
			if line == "No speech detected, skipping transcription." {
				noSpeech = true
			}
		}
	}

	text := strings.TrimSpace(stdout)
	if lastStatusLine >= 0 {
		text = strings.TrimSpace(strings.Join(lines[lastStatusLine+1:], "\n"))
	}
	if text != "" {
		return text, ""
	}

	if noSpeech {
		return "", "voxtype reported no speech detected"
	}

	parts := []string{"voxtype returned empty text"}
	if detail := strings.TrimSpace(stdout); detail != "" {
		parts = append(parts, "stdout="+truncateForWarning(detail, 240))
	}
	if detail := strings.TrimSpace(stderr); detail != "" {
		parts = append(parts, "stderr="+truncateForWarning(detail, 240))
	}
	return "", strings.Join(parts, "; ")
}

func normalizeVoxtypeNewlines(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

func isVoxtypeStatusLine(line string) bool {
	switch {
	case strings.HasPrefix(line, "Loading audio file:"):
		return true
	case strings.HasPrefix(line, "Audio format:"):
		return true
	case strings.HasPrefix(line, "Resampling from "):
		return true
	case strings.HasPrefix(line, "Processing "):
		return true
	case strings.HasPrefix(line, "VAD: "):
		return true
	case line == "No speech detected, skipping transcription.":
		return true
	default:
		return false
	}
}

func joinVoxtypeDetails(stdout, stderr string) string {
	parts := make([]string, 0, 2)
	if value := strings.TrimSpace(stdout); value != "" {
		parts = append(parts, "stdout="+value)
	}
	if value := strings.TrimSpace(stderr); value != "" {
		parts = append(parts, "stderr="+value)
	}
	return strings.Join(parts, "; ")
}
