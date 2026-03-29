package app

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"coe/internal/i18n"
	"coe/internal/notify"
	"coe/internal/pipeline"
)

const notificationTimeout = 3 * time.Second

type failureNotificationKind string

const (
	failureRecordingStart failureNotificationKind = "recording_start"
	failureRecordingStop  failureNotificationKind = "recording_stop"
	failureDictation      failureNotificationKind = "dictation"
)

func (a *App) emitNotification(logger *slog.Logger, msg notify.Message) {
	if a.Notifier == nil || msg.Title == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), notificationTimeout)
	defer cancel()

	if err := a.Notifier.Send(ctx, msg); err != nil {
		logger.Warn("notification warning", "error", err)
	}
}

func (a *App) emitServiceReadyNotification(logger *slog.Logger) {
	msg := a.notificationForServiceReady()
	if msg.Title == "" {
		return
	}

	service := a.Notifier
	shouldClose := false
	if service == nil || service.Summary() == "disabled" {
		connected, err := notify.ConnectSession("coe")
		if err != nil {
			logger.Warn("service-ready notification unavailable", "error", err)
			return
		}
		service = connected
		shouldClose = true
	}

	if shouldClose {
		defer func() {
			_ = service.Close()
		}()
	}

	ctx, cancel := context.WithTimeout(context.Background(), notificationTimeout)
	defer cancel()

	if err := service.Send(ctx, msg); err != nil {
		logger.Warn("service-ready notification warning", "error", err)
	}
}

func (a *App) notificationForStart() notify.Message {
	if !a.Config.Notifications.NotifyOnRecordingStart {
		return notify.Message{}
	}
	loc := a.Localizer

	return notify.Message{
		Title:   loc.Text(i18n.RecordingStartedTitle),
		Body:    loc.Text(i18n.RecordingStartedBody),
		Urgency: notify.UrgencyLow,
		Timeout: 2200 * time.Millisecond,
	}
}

func (a *App) notificationForServiceReady() notify.Message {
	loc := a.Localizer
	lines := []string{loc.Text(i18n.ServiceReadyBody)}
	if trigger := strings.TrimSpace(a.TriggerKey(context.Background())); trigger != "" {
		lines = append(lines, loc.Format(i18n.ServiceReadyTriggerLine, trigger))
	}

	return notify.Message{
		Title:   loc.Text(i18n.ServiceReadyTitle),
		Body:    strings.Join(lines, "\n"),
		Urgency: notify.UrgencyNormal,
		Timeout: 5000 * time.Millisecond,
	}
}

func (a *App) notificationForProcessing(result pipeline.Result, source string) notify.Message {
	if !a.Config.Notifications.NotifyOnComplete {
		return notify.Message{}
	}
	loc := a.Localizer

	if result.Transcript == "" {
		return notify.Message{
			Title:   loc.Text(i18n.NoSpeechDetectedTitle),
			Body:    normalizeBody(loc.LocalizeWarning(result.TranscriptWarning), loc.Text(i18n.NoSpeechDetectedFallback)),
			Urgency: notify.UrgencyNormal,
			Timeout: 4500 * time.Millisecond,
		}
	}

	status := loc.Text(i18n.DeliveryClipboard)
	if source == "fcitx-module" {
		status = loc.Text(i18n.DeliveryFcitx)
	} else if result.Output.PasteExecuted {
		status = loc.Text(i18n.DeliveryPasted)
	} else if result.Output.ClipboardWritten {
		status = loc.Text(i18n.DeliveryClipboard)
	} else {
		status = loc.Text(i18n.DeliveryNoAction)
	}
	if result.Output.PasteWarning != "" {
		status = status + " " + loc.Text(i18n.AutoPasteNeedsAttention)
	}

	text := strings.TrimSpace(result.Corrected)
	if text == "" {
		text = strings.TrimSpace(result.Transcript)
	}

	lines := []string{text, status}
	if result.CorrectionWarning != "" {
		lines = append(lines, loc.Text(i18n.CorrectionFallback))
	}

	return notify.Message{
		Title:   loc.Text(i18n.DictationCompleteTitle),
		Body:    strings.Join(compact(lines), "\n"),
		Urgency: notify.UrgencyNormal,
		Timeout: 5000 * time.Millisecond,
	}
}

func (a *App) notificationForFailure(kind failureNotificationKind, err error) notify.Message {
	if err == nil {
		return notify.Message{}
	}

	loc := a.Localizer
	title := loc.Text(i18n.DictationFailedTitle)
	switch kind {
	case failureRecordingStart:
		title = loc.Text(i18n.RecordingFailedToStartTitle)
	case failureRecordingStop:
		title = loc.Text(i18n.RecordingFailedTitle)
	case failureDictation:
		title = loc.Text(i18n.DictationFailedTitle)
	}

	return notify.Message{
		Title:   title,
		Body:    err.Error(),
		Urgency: notify.UrgencyCritical,
		Timeout: 6000 * time.Millisecond,
	}
}

func normalizeBody(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func compact(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
