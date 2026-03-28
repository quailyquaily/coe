package app

import (
	"testing"

	"coe/internal/config"
	"coe/internal/output"
	"coe/internal/pipeline"
)

func TestNotificationForProcessingWithPreviewAndPaste(t *testing.T) {
	t.Parallel()

	instance := &App{
		Config: config.Default(),
	}
	instance.Config.Notifications.ShowTextPreview = true

	msg := instance.notificationForProcessing(pipeline.Result{
		Transcript: "你好呀",
		Corrected:  "你好呀，哈喽！",
		Output: output.Delivery{
			ClipboardWritten: true,
			PasteExecuted:    true,
		},
	}, "ipc")

	if msg.Title != "Dictation complete" {
		t.Fatalf("unexpected title %q", msg.Title)
	}
	if msg.Body == "" {
		t.Fatal("expected notification body")
	}
	if got := msg.Body; got != "你好呀，哈喽！\nText copied and pasted into the focused app." {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestNotificationForProcessingWithoutTranscript(t *testing.T) {
	t.Parallel()

	instance := &App{
		Config: config.Default(),
	}
	instance.Config.Notifications.ShowTextPreview = true

	msg := instance.notificationForProcessing(pipeline.Result{
		TranscriptWarning: "ASR returned empty transcript; skipped correction and output",
	}, "ipc")

	if msg.Title != "No speech detected" {
		t.Fatalf("unexpected title %q", msg.Title)
	}
	if msg.Body == "" {
		t.Fatal("expected notification body")
	}
}

func TestNotificationForProcessingWithFcitxSource(t *testing.T) {
	t.Parallel()

	instance := &App{
		Config: config.Default(),
	}
	instance.Config.Notifications.ShowTextPreview = true

	msg := instance.notificationForProcessing(pipeline.Result{
		Transcript: "你好呀",
		Corrected:  "你好呀，哈喽！",
	}, "fcitx-module")

	if got := msg.Body; got != "你好呀，哈喽！\nText sent back through Fcitx." {
		t.Fatalf("unexpected body %q", got)
	}
}
