package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"coe/internal/audio"
	"coe/internal/control"
	"coe/internal/hotkey"
	"coe/internal/output"
)

func (a *App) Serve(ctx context.Context, w io.Writer) error {
	a.runtimeRunning.Store(true)
	defer a.runtimeRunning.Store(false)

	defer func() {
		for _, closer := range a.resourceClosers {
			_ = closer.Close()
		}
		if a.Notifier != nil {
			_ = a.Notifier.Close()
		}
		if a.Pipeline.Output != nil {
			_ = a.Pipeline.Output.Close()
		}
	}()

	logger := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: parseLogLevel(a.Config.Runtime.LogLevel),
	}))

	logger.Info("coe starting")
	logger.Info("runtime capabilities", "report", strings.TrimSpace(a.Caps.Report()))
	wiringAttrs := []any{
		"mode", a.Config.Runtime.Mode,
		"hotkey", a.Hotkey.Plan(),
		"pipeline", a.Pipeline.Summary(),
		"notifications", blankIfEmpty(a.Notifier.Summary(), "disabled"),
		"dictation_dbus", a.DictationBus != nil,
		"paste_shortcut", output.NormalizePasteShortcut(a.Config.Output.PasteShortcut),
		"terminal_paste_shortcut", output.NormalizePasteShortcut(a.Config.Output.TerminalPasteShortcut),
		"gnome_focus_helper", a.Config.Output.UseGNOMEFocusHelper,
	}
	if a.ControlSocketPath != "" {
		wiringAttrs = append(wiringAttrs, "control_socket", a.ControlSocketPath)
	}
	logger.Info("runtime wiring", wiringAttrs...)
	for _, warning := range a.StartupWarnings {
		logger.Warn("startup warning", "warning", warning)
	}
	logger.Info("runtime is scaffolded; waiting for signal")

	var controlErrCh chan error
	if a.ExternalHotkey != nil {
		server, err := control.NewServer(a.ControlSocketPath, a.handleControl)
		if err != nil {
			return err
		}

		controlErrCh = make(chan error, 1)
		go func() {
			controlErrCh <- server.Serve(ctx)
		}()
	}

	events, err := a.Hotkey.Events(ctx)
	if err != nil {
		return err
	}
	a.emitServiceReadyNotification(logger)

	var captureSession audio.CaptureSession
	var captureSource string

	handleStart := func(source string) runtimeCommandResponse {
		if captureSession != nil {
			return runtimeCommandResponse{Active: true}
		}

		session, err := a.Pipeline.Recorder.Start(ctx)
		if err != nil {
			message := fmt.Sprintf("recording start failed: %v", err)
			status := a.dictationState.Error(message)
			a.emitStateChanged(logger, status)
			a.emitDictationError(logger, status.SessionID, message)
			logger.Error("recording start failed", "error", err, "source", source)
			a.emitNotification(logger, a.notificationForFailure(failureRecordingStart, err))
			return runtimeCommandResponse{Err: err}
		}

		captureSession = session
		captureSource = source
		a.emitStateChanged(logger, a.dictationState.Recording("recording started"))
		logger.Info("recording started", "source", source)
		a.emitNotification(logger, a.notificationForStart())
		return runtimeCommandResponse{Active: true, Changed: true}
	}

	handleStop := func(source string) runtimeCommandResponse {
		if captureSession == nil {
			return runtimeCommandResponse{}
		}

		effectiveSource := captureSource
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		result, err := captureSession.Stop(stopCtx)
		cancel()
		captureSession = nil
		captureSource = ""
		if err != nil && result.ByteCount == 0 {
			message := fmt.Sprintf("recording stop failed: %v", err)
			status := a.dictationState.Error(message)
			a.emitStateChanged(logger, status)
			a.emitDictationError(logger, status.SessionID, message)
			stopAttrs := []any{"error", err, "source", effectiveSource}
			if result.Stderr != "" {
				stopAttrs = append(stopAttrs, "stderr", result.Stderr)
			}
			logger.Error("recording stop failed", stopAttrs...)
			a.emitNotification(logger, a.notificationForFailure(failureRecordingStop, err))
			return runtimeCommandResponse{Err: err}
		}
		if err != nil {
			logger.Warn("recording stop returned warning", "error", err, "source", effectiveSource)
		}

		a.emitStateChanged(logger, a.dictationState.Processing("processing audio"))
		recordingAttrs := []any{
			"bytes", result.ByteCount,
			"duration", result.Duration.Round(time.Millisecond),
			"source", effectiveSource,
		}
		activity := processedActivityPreview(result)
		if activity != "" {
			recordingAttrs = append(recordingAttrs, "audio_activity", activity)
		}
		if result.Stderr != "" {
			recordingAttrs = append(recordingAttrs, "stderr", result.Stderr)
		}
		logger.Info("recording stopped", recordingAttrs...)
		logger.Debug("capture processing started", "bytes", result.ByteCount, "source", effectiveSource)

		processor := a.Pipeline
		if effectiveSource == "fcitx-module" {
			processor.Output = nil
		}
		processed, err := processor.ProcessCapture(ctx, result)
		if err != nil {
			status := a.dictationState.Error(err.Error())
			a.emitStateChanged(logger, status)
			a.emitDictationError(logger, status.SessionID, err.Error())
			logger.Error("pipeline processing failed", "error", err, "source", effectiveSource)
			a.emitNotification(logger, a.notificationForFailure(failureDictation, err))
			return runtimeCommandResponse{Err: err}
		}

		pipelineAttrs := []any{
			"transcript", processed.Transcript,
			"corrected", processed.Corrected,
			"asr_duration", processed.ASRDuration.Round(time.Millisecond),
			"correction_duration", processed.CorrectionDuration.Round(time.Millisecond),
			"output_duration", processed.OutputDuration.Round(time.Millisecond),
			"total_duration", processed.TotalDuration.Round(time.Millisecond),
			"source", effectiveSource,
		}
		if processed.AudioActivity.Supported {
			pipelineAttrs = append(pipelineAttrs, "audio_activity", processed.AudioActivity.Summary())
		}
		logger.Info("pipeline result", pipelineAttrs...)
		logger.Debug(
			"asr stage completed",
			"duration", processed.ASRDuration.Round(time.Millisecond),
			"transcript_chars", len([]rune(processed.Transcript)),
			"warning", blankIfEmpty(processed.TranscriptWarning, "none"),
		)
		logger.Debug(
			"correction stage completed",
			"duration", processed.CorrectionDuration.Round(time.Millisecond),
			"corrected_chars", len([]rune(processed.Corrected)),
			"changed", processed.Corrected != "" && processed.Corrected != processed.Transcript,
			"warning", blankIfEmpty(processed.CorrectionWarning, "none"),
		)
		logger.Debug(
			"output stage completed",
			"duration", processed.OutputDuration.Round(time.Millisecond),
			"clipboard", processed.Output.ClipboardWritten,
			"clipboard_method", blankIfEmpty(processed.Output.ClipboardMethod, "none"),
			"clipboard_duration", processed.Output.ClipboardDuration.Round(time.Millisecond),
			"clipboard_warning", blankIfEmpty(processed.Output.ClipboardWarning, "none"),
			"paste", processed.Output.PasteExecuted,
			"paste_method", blankIfEmpty(processed.Output.PasteMethod, "none"),
			"paste_shortcut", blankIfEmpty(processed.Output.PasteShortcut, "none"),
			"paste_target", blankIfEmpty(processed.Output.PasteTarget, "unknown"),
			"paste_duration", processed.Output.PasteDuration.Round(time.Millisecond),
			"paste_warning", blankIfEmpty(processed.Output.PasteWarning, "none"),
		)
		logger.Debug(
			"pipeline stage timings",
			"asr_duration", processed.ASRDuration.Round(time.Millisecond),
			"correction_duration", processed.CorrectionDuration.Round(time.Millisecond),
			"output_duration", processed.OutputDuration.Round(time.Millisecond),
			"total_duration", processed.TotalDuration.Round(time.Millisecond),
		)
		if processed.TranscriptWarning != "" {
			logger.Warn("transcript warning", "warning", processed.TranscriptWarning)
		}
		if processed.CorrectionWarning != "" {
			logger.Warn("correction warning", "warning", processed.CorrectionWarning)
		}
		if strings.TrimSpace(processed.Corrected) == "" {
			message := firstNonEmpty(
				processed.TranscriptWarning,
				processed.CorrectionWarning,
				"dictation produced no text",
			)
			status := a.dictationState.Error(message)
			a.emitStateChanged(logger, status)
			a.emitDictationError(logger, status.SessionID, message)
		} else {
			status := a.dictationState.Completed("result ready")
			a.emitStateChanged(logger, status)
			a.emitResultReady(logger, status.SessionID, processed.Corrected)
		}
		logger.Info(
			"output result",
			"clipboard", processed.Output.ClipboardWritten,
			"clipboard_method", blankIfEmpty(processed.Output.ClipboardMethod, "none"),
			"clipboard_duration", processed.Output.ClipboardDuration.Round(time.Millisecond),
			"paste", processed.Output.PasteExecuted,
			"paste_method", blankIfEmpty(processed.Output.PasteMethod, "none"),
			"paste_shortcut", blankIfEmpty(processed.Output.PasteShortcut, "none"),
			"paste_target", blankIfEmpty(processed.Output.PasteTarget, "unknown"),
			"paste_duration", processed.Output.PasteDuration.Round(time.Millisecond),
		)
		logger.Debug(
			"output delivery details",
			"clipboard_method", blankIfEmpty(processed.Output.ClipboardMethod, "none"),
			"clipboard_duration", processed.Output.ClipboardDuration.Round(time.Millisecond),
			"clipboard_warning", blankIfEmpty(processed.Output.ClipboardWarning, "none"),
			"paste_method", blankIfEmpty(processed.Output.PasteMethod, "none"),
			"paste_shortcut", blankIfEmpty(processed.Output.PasteShortcut, "none"),
			"paste_target", blankIfEmpty(processed.Output.PasteTarget, "unknown"),
			"paste_duration", processed.Output.PasteDuration.Round(time.Millisecond),
			"paste_warning", blankIfEmpty(processed.Output.PasteWarning, "none"),
		)
		if processed.Output.PasteWarning != "" {
			logger.Warn("paste warning", "warning", processed.Output.PasteWarning)
		}
		a.emitNotification(logger, a.notificationForProcessing(processed, effectiveSource))
		return runtimeCommandResponse{Changed: true}
	}

	for {
		select {
		case <-ctx.Done():
			if captureSession != nil {
				stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				result, stopErr := captureSession.Stop(stopCtx)
				cancel()
				if stopErr != nil {
					logger.Warn("recording stop during shutdown failed", "error", stopErr)
				} else {
					logger.Info("recording finalized during shutdown", "bytes", result.ByteCount, "duration", result.Duration.Round(time.Millisecond))
				}
			}
			logger.Info("shutting down")
			return nil
		case err := <-controlErrCh:
			if err != nil {
				return err
			}
			controlErrCh = nil
		case command := <-a.runtimeCommands:
			var response runtimeCommandResponse
			switch command.Type {
			case runtimeCommandToggle:
				if captureSession == nil {
					response = handleStart(command.Source)
				} else {
					response = handleStop(command.Source)
				}
			case runtimeCommandStart:
				response = handleStart(command.Source)
			case runtimeCommandStop:
				response = handleStop(command.Source)
			default:
				response = runtimeCommandResponse{Err: fmt.Errorf("unsupported runtime command %q", command.Type)}
			}
			command.Reply <- response
		case event, ok := <-events:
			if !ok {
				logger.Info("hotkey service stopped")
				return nil
			}
			switch event.Type {
			case hotkey.Activated:
				_ = handleStart("hotkey")
			case hotkey.Deactivated:
				_ = handleStop("hotkey")
			default:
				logger.Warn("unknown trigger event", "type", event.Type)
			}
		}
	}
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func processedActivityPreview(result audio.Result) string {
	activity := audio.AnalyzeActivity(result, audio.DefaultActivityThresholds())
	if !activity.Supported {
		return ""
	}
	return activity.Summary()
}

func blankIfEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
