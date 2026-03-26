package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"coe/internal/asr"
	"coe/internal/audio"
	"coe/internal/capabilities"
	"coe/internal/config"
	"coe/internal/control"
	"coe/internal/hotkey"
	"coe/internal/llm"
	"coe/internal/output"
	"coe/internal/pipeline"
)

type App struct {
	Config            config.Config
	Caps              capabilities.Capabilities
	Hotkey            hotkey.Service
	ExternalHotkey    *hotkey.ExternalTriggerService
	ControlSocketPath string
	Pipeline          pipeline.Orchestrator
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	caps, err := capabilities.Probe(ctx)
	if err != nil {
		return nil, err
	}

	recorder := audio.PWRecord{
		Binary:     cfg.Audio.RecorderBinary,
		SampleRate: cfg.Audio.SampleRate,
		Channels:   cfg.Audio.Channels,
		Format:     cfg.Audio.Format,
	}
	asrClient, err := asr.NewClient(cfg.ASR)
	if err != nil {
		return nil, err
	}
	corrector, err := llm.NewCorrector(cfg.LLM)
	if err != nil {
		return nil, err
	}
	clipboardBinary := cfg.Output.ClipboardBinary
	if binary := caps.Binaries["wl-copy"]; binary.Found {
		clipboardBinary = binary.Path
	}
	pasteBinary := cfg.Output.PasteBinary
	if pasteBinary == "" {
		if binary := caps.Binaries["ydotool"]; binary.Found {
			pasteBinary = binary.Path
		}
	}

	description := describeFeature(string(caps.Hotkey.Mode), caps.Hotkey.Detail)
	service := hotkey.Service(hotkey.PlannedService{Description: description})
	var external *hotkey.ExternalTriggerService
	var controlSocketPath string

	if caps.Hotkey.Mode == capabilities.ModeExternalBinding && cfg.Runtime.AllowExternalTrigger {
		external = hotkey.NewExternalTriggerService(description)
		service = external

		socketPath, err := control.ResolveSocketPath()
		if err != nil {
			return nil, err
		}
		controlSocketPath = socketPath
	}

	instance := &App{
		Config:            cfg,
		Caps:              caps,
		Hotkey:            service,
		ExternalHotkey:    external,
		ControlSocketPath: controlSocketPath,
		Pipeline: pipeline.Orchestrator{
			Recorder:  recorder,
			ASR:       asrClient,
			Corrector: corrector,
			Output: &output.Coordinator{
				ClipboardPlan:      describeFeature(string(caps.Clipboard.Mode), caps.Clipboard.Detail),
				PastePlan:          describeFeature(string(caps.Paste.Mode), caps.Paste.Detail),
				ClipboardBinary:    clipboardBinary,
				PasteBinary:        pasteBinary,
				EnableAutoPaste:    cfg.Output.EnableAutoPaste,
				UsePortalClipboard: caps.Clipboard.Mode == capabilities.ModePortal,
				UsePortalPaste:     caps.Paste.Mode == capabilities.ModePortal,
			},
		},
	}

	return instance, nil
}

func (a *App) Serve(ctx context.Context, w io.Writer) error {
	defer func() {
		if a.Pipeline.Output != nil {
			_ = a.Pipeline.Output.Close()
		}
	}()

	fmt.Fprintln(w, "coe skeleton starting")
	fmt.Fprintln(w, a.Caps.Report())
	fmt.Fprintf(w, "hotkey wiring: %s\n", a.Hotkey.Plan())
	fmt.Fprintf(w, "pipeline wiring: %s\n", a.Pipeline.Summary())
	if a.ControlSocketPath != "" {
		fmt.Fprintf(w, "control socket: %s\n", a.ControlSocketPath)
	}
	fmt.Fprintln(w, "runtime is scaffolded; waiting for signal")

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

	var captureSession audio.CaptureSession

	for {
		select {
		case <-ctx.Done():
			if captureSession != nil {
				stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				result, stopErr := captureSession.Stop(stopCtx)
				cancel()
				if stopErr != nil {
					fmt.Fprintf(w, "recording stop during shutdown failed: %v\n", stopErr)
				} else {
					fmt.Fprintf(w, "recording finalized during shutdown: bytes=%d duration=%s\n", result.ByteCount, result.Duration.Round(time.Millisecond))
				}
			}
			fmt.Fprintln(w, "shutting down")
			return nil
		case err := <-controlErrCh:
			if err != nil {
				return err
			}
			controlErrCh = nil
		case event, ok := <-events:
			if !ok {
				fmt.Fprintln(w, "hotkey service stopped")
				return nil
			}
			switch event.Type {
			case hotkey.Activated:
				if captureSession != nil {
					fmt.Fprintln(w, "recording already active; ignoring activate event")
					continue
				}

				session, err := a.Pipeline.Recorder.Start(ctx)
				if err != nil {
					fmt.Fprintf(w, "recording start failed: %v\n", err)
					continue
				}

				captureSession = session
				fmt.Fprintln(w, "recording started")
			case hotkey.Deactivated:
				if captureSession == nil {
					fmt.Fprintln(w, "recording is not active; ignoring deactivate event")
					continue
				}

				stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				result, err := captureSession.Stop(stopCtx)
				cancel()
				captureSession = nil
				if err != nil && result.ByteCount == 0 {
					fmt.Fprintf(w, "recording stop failed: %v\n", err)
					if result.Stderr != "" {
						fmt.Fprintf(w, "recording stderr: %q\n", result.Stderr)
					}
					continue
				}
				if err != nil {
					fmt.Fprintf(w, "recording stop returned warning: %v\n", err)
				}

				fmt.Fprintf(w, "recording stopped: bytes=%d duration=%s\n", result.ByteCount, result.Duration.Round(time.Millisecond))
				if result.Stderr != "" {
					fmt.Fprintf(w, "recording stderr: %q\n", result.Stderr)
				}

				processed, err := a.Pipeline.ProcessCapture(ctx, result)
				if err != nil {
					fmt.Fprintf(w, "pipeline processing failed: %v\n", err)
					continue
				}

				fmt.Fprintf(w, "pipeline result: transcript=%q corrected=%q\n", processed.Transcript, processed.Corrected)
				if processed.CorrectionWarning != "" {
					fmt.Fprintf(w, "correction warning: %s\n", processed.CorrectionWarning)
				}
				fmt.Fprintf(w, "output result: clipboard=%t(%s) paste=%t(%s)\n",
					processed.Output.ClipboardWritten,
					blankIfEmpty(processed.Output.ClipboardMethod, "none"),
					processed.Output.PasteExecuted,
					blankIfEmpty(processed.Output.PasteMethod, "none"),
				)
				if processed.Output.PasteWarning != "" {
					fmt.Fprintf(w, "paste warning: %s\n", processed.Output.PasteWarning)
				}
			default:
				fmt.Fprintf(w, "unknown trigger event: %s\n", event.Type)
			}
		}
	}
}

func (a *App) handleControl(_ context.Context, req control.Request) control.Response {
	if a.ExternalHotkey == nil {
		return control.Response{
			OK:      false,
			Message: "external trigger fallback is not active in this runtime",
		}
	}

	switch req.Command {
	case control.CommandPing:
		return control.Response{OK: true, Message: "pong", Active: a.ExternalHotkey.Active()}
	case control.CommandTriggerStart:
		changed := a.ExternalHotkey.TriggerStart()
		return control.Response{
			OK:      true,
			Message: pickMessage(changed, "trigger started", "trigger already active"),
			Active:  a.ExternalHotkey.Active(),
		}
	case control.CommandTriggerStop:
		changed := a.ExternalHotkey.TriggerStop()
		return control.Response{
			OK:      true,
			Message: pickMessage(changed, "trigger stopped", "trigger already inactive"),
			Active:  a.ExternalHotkey.Active(),
		}
	case control.CommandTriggerToggle:
		active := a.ExternalHotkey.TriggerToggle()
		return control.Response{
			OK:      true,
			Message: pickMessage(active, "trigger toggled on", "trigger toggled off"),
			Active:  active,
		}
	case control.CommandTriggerStatus:
		active := a.ExternalHotkey.Active()
		return control.Response{
			OK:      true,
			Message: pickMessage(active, "trigger active", "trigger inactive"),
			Active:  active,
		}
	default:
		return control.Response{
			OK:      false,
			Message: fmt.Sprintf("unsupported control command %q", req.Command),
			Active:  a.ExternalHotkey.Active(),
		}
	}
}

func describeFeature(mode, detail string) string {
	if detail == "" {
		return mode
	}
	return fmt.Sprintf("%s (%s)", mode, detail)
}

func pickMessage(condition bool, yes, no string) string {
	if condition {
		return yes
	}
	return no
}

func blankIfEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
