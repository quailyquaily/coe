package app

import (
	"bytes"
	"context"
	"testing"
	"time"

	"coe/internal/asr"
	"coe/internal/audio"
	"coe/internal/config"
	"coe/internal/hotkey"
	"coe/internal/i18n"
	"coe/internal/llm"
	"coe/internal/pipeline"
	"coe/internal/scene"
)

type stubRuntimeRecorder struct {
	session audio.CaptureSession
}

func (r stubRuntimeRecorder) Start(context.Context) (audio.CaptureSession, error) {
	return r.session, nil
}

func (stubRuntimeRecorder) Summary() string {
	return "stub-recorder"
}

type stubRuntimeCaptureSession struct {
	result    audio.Result
	stopErr   error
	cancelErr error
	stopCalls int
}

func (s *stubRuntimeCaptureSession) Stop(context.Context) (audio.Result, error) {
	s.stopCalls++
	return s.result, s.stopErr
}

func (s *stubRuntimeCaptureSession) Cancel(context.Context) error {
	return s.cancelErr
}

type stubRuntimeASR struct {
	result asr.Result
	err    error
}

func (s stubRuntimeASR) Transcribe(context.Context, audio.Result) (asr.Result, error) {
	return s.result, s.err
}

func (stubRuntimeASR) Name() string {
	return "stub-asr"
}

type recordingSelectionEditor struct {
	inputs []llm.SelectionEditInput
	text   string
	err    error
}

func (e *recordingSelectionEditor) Edit(_ context.Context, input llm.SelectionEditInput) (llm.Result, error) {
	e.inputs = append(e.inputs, input)
	return llm.Result{Text: e.text}, e.err
}

func (e *recordingSelectionEditor) Name() string {
	return "recording-selection-editor"
}

func TestServeSelectionEditReturnsToIdleAfterSuccess(t *testing.T) {
	t.Parallel()

	sceneState, err := scene.NewState(scene.DefaultCatalog(), scene.IDGeneral)
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}

	capture := &stubRuntimeCaptureSession{
		result: audio.Result{
			Data:       []byte{0x00, 0x00, 0xff, 0x7f},
			ByteCount:  4,
			SampleRate: 16000,
			Channels:   1,
			Format:     "s16",
			StartedAt:  time.Now().Add(-150 * time.Millisecond),
			StoppedAt:  time.Now(),
			Duration:   150 * time.Millisecond,
		},
	}
	editor := &recordingSelectionEditor{text: "rewritten text"}

	instance := &App{
		Config: config.Config{
			Runtime: config.RuntimeConfig{
				Mode: config.RuntimeModeFcitx,
			},
		},
		Hotkey:    hotkey.PlannedService{Description: "test"},
		Notifier:  stubNotifier{},
		Localizer: i18n.NewForLocale("zh_CN.UTF-8"),
		SceneState: sceneState,
		SceneCorrectors: map[string]llm.Corrector{
			scene.IDGeneral: stubCorrector{text: "make it shorter"},
		},
		SceneEditors: map[string]llm.SelectionEditor{
			scene.IDGeneral: editor,
		},
		dictationState:  newDictationState(),
		runtimeCommands: make(chan runtimeCommand, 16),
		Pipeline: pipeline.Orchestrator{
			Recorder:  stubRuntimeRecorder{session: capture},
			ASR:       stubRuntimeASR{result: asr.Result{Text: "rewrite this"}},
			Corrector: stubCorrector{text: "make it shorter"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logOutput bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- instance.Serve(ctx, &logOutput)
	}()

	waitForRuntimeLoop(t, instance)

	changed, err := instance.triggerStartWithSelectionEditFrom("fcitx-module", "hello world")
	if err != nil {
		t.Fatalf("triggerStartWithSelectionEditFrom() error = %v", err)
	}
	if !changed {
		t.Fatal("triggerStartWithSelectionEditFrom() = false, want true")
	}

	changed, err = instance.triggerStopFrom("fcitx-module")
	if err != nil {
		t.Fatalf("triggerStopFrom() error = %v", err)
	}
	if !changed {
		t.Fatal("triggerStopFrom() = false, want true")
	}

	if capture.stopCalls != 1 {
		t.Fatalf("capture.stopCalls = %d, want 1\nlogs:\n%s", capture.stopCalls, logOutput.String())
	}
	if len(editor.inputs) != 1 {
		t.Fatalf("len(editor.inputs) = %d, want 1\nlogs:\n%s", len(editor.inputs), logOutput.String())
	}
	if editor.inputs[0].SelectedText != "hello world" {
		t.Fatalf("editor selected text = %q, want %q", editor.inputs[0].SelectedText, "hello world")
	}
	if editor.inputs[0].Instruction != "make it shorter" {
		t.Fatalf("editor instruction = %q, want %q", editor.inputs[0].Instruction, "make it shorter")
	}

	status := instance.Status(context.Background())
	if status.State != "idle" {
		t.Fatalf("Status().State = %q, want %q", status.State, "idle")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve() did not stop after context cancellation")
	}
}

func waitForRuntimeLoop(t *testing.T, instance *App) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if instance.runtimeRunning.Load() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("runtime loop did not start")
}
