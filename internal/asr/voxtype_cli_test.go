package asr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"coe/internal/audio"
)

func TestVoxtypeCLIClientTranscribe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binary := filepath.Join(dir, "voxtype")
	script := `#!/bin/sh
if [ "$1" != "transcribe" ]; then
  echo "unexpected subcommand: $1" >&2
  exit 2
fi
if [ ! -f "$2" ]; then
  echo "missing audio file: $2" >&2
  exit 3
fi
if [ "$3" != "--engine" ] || [ "$4" != "omnilingual" ]; then
  echo "unexpected engine args: $3 $4" >&2
  exit 4
fi
printf 'Loading audio file: %s\n' "$2"
printf 'Audio format: 16000 Hz, 1 channel(s), Int\n'
printf 'Processing 2 samples (0.00s)...\n'
printf '\n你好，世界。\n'
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := VoxtypeCLIClient{
		Binary: binary,
		Engine: "omnilingual",
	}

	result, err := client.Transcribe(context.Background(), audio.Result{
		Data:       []byte{0x00, 0x00, 0x00, 0x00},
		SampleRate: 16000,
		Channels:   1,
		Format:     "s16",
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Text != "你好，世界。" {
		t.Fatalf("result.Text = %q", result.Text)
	}
	if result.Warning != "" {
		t.Fatalf("result.Warning = %q, want empty", result.Warning)
	}
}

func TestVoxtypeCLIClientReturnsWarningForNoSpeech(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binary := filepath.Join(dir, "voxtype")
	script := `#!/bin/sh
printf 'Loading audio file: %s\n' "$2"
printf 'Processing 2 samples (0.00s)...\n'
printf 'No speech detected, skipping transcription.\n'
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := VoxtypeCLIClient{Binary: binary}
	result, err := client.Transcribe(context.Background(), audio.Result{
		Data:       []byte{0x00, 0x00},
		SampleRate: 16000,
		Channels:   1,
		Format:     "s16",
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Text != "" {
		t.Fatalf("result.Text = %q, want empty", result.Text)
	}
	if !strings.Contains(result.Warning, "no speech") {
		t.Fatalf("result.Warning = %q", result.Warning)
	}
}

func TestParseVoxtypeTranscribeOutputKeepsMultilineText(t *testing.T) {
	t.Parallel()

	stdout := strings.Join([]string{
		"Loading audio file: capture.wav",
		"Audio format: 16000 Hz, 1 channel(s), Int",
		"Processing 32000 samples (2.00s)...",
		"",
		"第一段。",
		"",
		"第二段。",
	}, "\n")

	text, warning := parseVoxtypeTranscribeOutput(stdout, "")
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
	if text != "第一段。\n\n第二段。" {
		t.Fatalf("text = %q", text)
	}
}

func TestVoxtypeCLIClientReturnsErrorOnFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binary := filepath.Join(dir, "voxtype")
	script := "#!/bin/sh\necho 'fatal: unsupported engine' >&2\nexit 9\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := VoxtypeCLIClient{Binary: binary}
	_, err := client.Transcribe(context.Background(), audio.Result{
		Data:       []byte{0x00, 0x00},
		SampleRate: 16000,
		Channels:   1,
		Format:     "s16",
	})
	if err == nil {
		t.Fatal("Transcribe() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "fatal: unsupported engine") {
		t.Fatalf("error = %q", err)
	}
}
