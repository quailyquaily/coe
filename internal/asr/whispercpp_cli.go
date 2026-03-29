package asr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"coe/internal/audio"
	"coe/internal/prompts"
)

type WhisperCPPCLIClient struct {
	Binary     string
	ModelPath  string
	Language   string
	Prompt     string
	PromptFile string
	Threads    int
	UseGPU     bool
}

func (c WhisperCPPCLIClient) Name() string {
	modelName := filepath.Base(strings.TrimSpace(c.ModelPath))
	if modelName == "." || modelName == "" {
		modelName = "unknown-model"
	}
	return "whisper.cpp-cli-" + modelName
}

func (c WhisperCPPCLIClient) Transcribe(ctx context.Context, capture audio.Result) (Result, error) {
	modelPath := strings.TrimSpace(c.ModelPath)
	if modelPath == "" {
		return Result{}, fmt.Errorf("whisper.cpp ASR requires asr.model_path")
	}

	wav, err := audio.EncodeWAV(capture)
	if err != nil {
		return Result{}, err
	}

	tmpDir, err := os.MkdirTemp("", "coe-whispercpp-*")
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
		binary = "whisper-cli"
	}
	prompt, err := prompts.ResolveASR(c.Prompt, c.PromptFile, prompts.ASRTemplateData{
		Provider: "whispercpp",
		Model:    filepath.Base(strings.TrimSpace(c.ModelPath)),
		Language: strings.TrimSpace(c.Language),
	})
	if err != nil {
		return Result{}, err
	}

	args := []string{
		"--model", modelPath,
		"--file", inputPath,
		"--threads", fmt.Sprintf("%d", chooseWhisperThreads(c.Threads)),
		"--no-prints",
		"--no-timestamps",
	}
	if language := strings.TrimSpace(c.Language); language != "" {
		args = append(args, "--language", language)
	} else {
		args = append(args, "--language", "auto")
	}
	if prompt != "" {
		args = append(args, "--prompt", prompt)
	}
	if !c.UseGPU {
		args = append(args, "--no-gpu")
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text != "" {
			return Result{}, fmt.Errorf("whisper-cli failed: %w (%s)", err, text)
		}
		return Result{}, fmt.Errorf("whisper-cli failed: %w", err)
	}
	if text == "" {
		return Result{Warning: "whisper-cli returned empty text"}, nil
	}

	return Result{Text: text}, nil
}

func chooseWhisperThreads(configured int) int {
	if configured > 0 {
		return configured
	}
	if n := runtime.GOMAXPROCS(0); n > 0 {
		return n
	}
	return 1
}
