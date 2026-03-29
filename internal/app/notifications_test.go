package app

import (
	"errors"
	"testing"

	"coe/internal/config"
	"coe/internal/i18n"
	"coe/internal/output"
	"coe/internal/pipeline"
)

func TestNotificationForProcessingDisabledByDefault(t *testing.T) {
	t.Parallel()

	instance := &App{
		Config: config.Default(),
	}

	msg := instance.notificationForProcessing(pipeline.Result{
		Transcript: "你好呀",
		Corrected:  "你好呀，哈喽！",
		Output: output.Delivery{
			ClipboardWritten: true,
			PasteExecuted:    true,
		},
	}, "ipc")

	if msg.Title != "" || msg.Body != "" {
		t.Fatalf("expected completion notification to be disabled by default, got %#v", msg)
	}
}

func TestNotificationForProcessingWithPaste(t *testing.T) {
	t.Parallel()

	instance := &App{
		Config: config.Default(),
	}
	instance.Config.Notifications.NotifyOnComplete = true

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
	if got := msg.Body; got != "你好呀，哈喽！\nText copied and pasted into the focused app." {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestNotificationForServiceReady(t *testing.T) {
	t.Parallel()

	instance := &App{
		Config: config.Default(),
	}

	msg := instance.notificationForServiceReady()
	if msg.Title != "Coe service ready" {
		t.Fatalf("unexpected title %q", msg.Title)
	}
	if got := msg.Body; got != "Background service is running and ready for dictation.\nTrigger: <Shift><Super>d" {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestNotificationForServiceReadyLocalized(t *testing.T) {
	t.Parallel()

	instance := &App{
		Config:    config.Default(),
		Localizer: i18n.NewForLocale("zh_CN.UTF-8"),
	}

	msg := instance.notificationForServiceReady()
	if msg.Title != "Coe 服务已就绪" {
		t.Fatalf("unexpected title %q", msg.Title)
	}
	if got := msg.Body; got != "后台服务已启动，可以开始听写。\n触发键：<Shift><Super>d" {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestNotificationForProcessingWithoutTranscript(t *testing.T) {
	t.Parallel()

	instance := &App{
		Config: config.Default(),
	}
	instance.Config.Notifications.NotifyOnComplete = true

	msg := instance.notificationForProcessing(pipeline.Result{
		TranscriptWarning: "ASR returned empty transcript; skipped correction and output",
	}, "ipc")

	if msg.Title != "No speech detected" {
		t.Fatalf("unexpected title %q", msg.Title)
	}
	if got := msg.Body; got != "ASR returned empty text, so correction and output were skipped." {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestNotificationForProcessingWithFcitxSource(t *testing.T) {
	t.Parallel()

	instance := &App{
		Config: config.Default(),
	}
	instance.Config.Notifications.NotifyOnComplete = true

	msg := instance.notificationForProcessing(pipeline.Result{
		Transcript: "你好呀",
		Corrected:  "你好呀，哈喽！",
	}, "fcitx-module")

	if got := msg.Body; got != "你好呀，哈喽！\nText sent back through Fcitx." {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestNotificationForFailureLocalized(t *testing.T) {
	t.Parallel()

	instance := &App{
		Localizer: i18n.NewForLocale("ja_JP.UTF-8"),
	}

	msg := instance.notificationForFailure(failureDictation, errors.New("boom"))
	if msg.Title != "音声入力に失敗しました" {
		t.Fatalf("unexpected title %q", msg.Title)
	}
	if msg.Body != "boom" {
		t.Fatalf("unexpected body %q", msg.Body)
	}
}
