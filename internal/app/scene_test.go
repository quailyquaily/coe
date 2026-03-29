package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"coe/internal/dictionary"
	"coe/internal/focus"
	"coe/internal/i18n"
	"coe/internal/llm"
	"coe/internal/notify"
	"coe/internal/output"
	"coe/internal/pipeline"
	"coe/internal/scene"
)

type stubCorrector struct {
	text string
	err  error
}

func (s stubCorrector) Correct(context.Context, string) (llm.Result, error) {
	return llm.Result{Text: s.text}, s.err
}

func (s stubCorrector) Name() string {
	return "stub"
}

type stubNotifier struct{}

func (stubNotifier) Summary() string {
	return "test"
}

func (stubNotifier) Send(context.Context, notify.Message) error {
	return nil
}

func (stubNotifier) Close() error {
	return nil
}

type recordingNotifier struct {
	messages []notify.Message
}

func (n *recordingNotifier) Summary() string {
	return "test"
}

func (n *recordingNotifier) Send(_ context.Context, msg notify.Message) error {
	n.messages = append(n.messages, msg)
	return nil
}

func (n *recordingNotifier) Close() error {
	return nil
}

type stubFocusProvider struct {
	target focus.Target
	err    error
}

func (p stubFocusProvider) Focused(context.Context) (focus.Target, error) {
	return p.target, p.err
}

func (stubFocusProvider) Summary() string {
	return "stub"
}

func (stubFocusProvider) Close() error {
	return nil
}

func TestAttemptSceneCommandSwitchesScene(t *testing.T) {
	t.Parallel()

	sceneState, err := scene.NewState(scene.DefaultCatalog(), scene.IDGeneral)
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}

	instance := &App{
		Localizer:  i18n.NewForLocale("zh_CN.UTF-8"),
		SceneState: sceneState,
		SceneRouter: stubCorrector{
			text: `{"intent":"switch_scene","target_scene":"terminal","matched_alias":"终端"}`,
		},
	}

	outcome, err := instance.attemptSceneCommand(context.Background(), "切换场景到终端")
	if err != nil {
		t.Fatalf("attemptSceneCommand() error = %v", err)
	}
	if !outcome.Handled || !outcome.Changed {
		t.Fatalf("unexpected outcome %#v", outcome)
	}
	if got := instance.currentScene().ID; got != scene.IDTerminal {
		t.Fatalf("currentScene().ID = %q, want %q", got, scene.IDTerminal)
	}
}

func TestStatusIncludesSceneDetail(t *testing.T) {
	t.Parallel()

	sceneState, err := scene.NewState(scene.DefaultCatalog(), scene.IDTerminal)
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}

	instance := &App{
		SceneState:     sceneState,
		dictationState: newDictationState(),
	}

	status := instance.Status(context.Background())
	if status.Detail != "scene=terminal" {
		t.Fatalf("Status().Detail = %q, want %q", status.Detail, "scene=terminal")
	}
}

func TestListScenesReturnsLocalizedJSON(t *testing.T) {
	t.Parallel()

	sceneState, err := scene.NewState(scene.DefaultCatalog(), scene.IDTerminal)
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}

	instance := &App{
		Localizer:  i18n.NewForLocale("zh_CN.UTF-8"),
		SceneState: sceneState,
	}

	var scenes []scene.DisplayScene
	if err := json.Unmarshal([]byte(instance.ListScenes(context.Background())), &scenes); err != nil {
		t.Fatalf("ListScenes() JSON error = %v", err)
	}
	if len(scenes) != 2 {
		t.Fatalf("len(scenes) = %d, want 2", len(scenes))
	}
	if scenes[1].ID != scene.IDTerminal || !scenes[1].Current || scenes[1].DisplayName != "终端" {
		t.Fatalf("unexpected scenes %#v", scenes)
	}
}

func TestSwitchSceneChangesCurrentScene(t *testing.T) {
	t.Parallel()

	sceneState, err := scene.NewState(scene.DefaultCatalog(), scene.IDGeneral)
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}

	instance := &App{
		Localizer:  i18n.NewForLocale("en_US.UTF-8"),
		Notifier:   stubNotifier{},
		SceneState: sceneState,
	}

	if err := instance.SwitchScene(context.Background(), scene.IDTerminal); err != nil {
		t.Fatalf("SwitchScene() error = %v", err)
	}
	if got := instance.currentScene().ID; got != scene.IDTerminal {
		t.Fatalf("currentScene().ID = %q, want %q", got, scene.IDTerminal)
	}
}

func TestNormalizeForSceneUsesDictionary(t *testing.T) {
	t.Parallel()

	dict, err := dictionary.Parse([]byte(`
entries:
  - canonical: "systemctl"
    aliases: ["system control"]
    scenes: ["terminal"]
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	instance := &App{
		Dictionary: dict,
	}

	result := instance.normalizeForScene(pipeline.Result{
		Corrected: "please run system control now",
	}, scene.IDTerminal)
	if result.Corrected != "please run systemctl now" {
		t.Fatalf("Corrected = %q", result.Corrected)
	}
}

func TestAutoSwitchSceneUsesFocusedTerminal(t *testing.T) {
	t.Parallel()

	sceneState, err := scene.NewState(scene.DefaultCatalog(), scene.IDGeneral)
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}

	notifier := &recordingNotifier{}
	instance := &App{
		Localizer:  i18n.NewForLocale("en_US.UTF-8"),
		Notifier:   notifier,
		SceneState: sceneState,
		Pipeline: pipeline.Orchestrator{
			Output: &output.Coordinator{
				FocusProvider: stubFocusProvider{
					target: focus.Target{WMClass: "gnome-terminal-server"},
				},
			},
		},
	}

	current, target := instance.autoSwitchScene(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if target == nil || target.WMClass != "gnome-terminal-server" {
		t.Fatalf("target = %#v", target)
	}
	if current.ID != scene.IDTerminal {
		t.Fatalf("scene = %q, want %q", current.ID, scene.IDTerminal)
	}
	if instance.currentScene().ID != scene.IDTerminal {
		t.Fatalf("currentScene().ID = %q, want %q", instance.currentScene().ID, scene.IDTerminal)
	}
	if len(notifier.messages) != 1 {
		t.Fatalf("notification count = %d, want 1", len(notifier.messages))
	}
	if notifier.messages[0].Body != "Terminal" {
		t.Fatalf("notification body = %q, want %q", notifier.messages[0].Body, "Terminal")
	}
}

func TestAutoSwitchSceneSkipsOnFocusError(t *testing.T) {
	t.Parallel()

	sceneState, err := scene.NewState(scene.DefaultCatalog(), scene.IDTerminal)
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}

	notifier := &recordingNotifier{}
	instance := &App{
		Localizer:  i18n.NewForLocale("en_US.UTF-8"),
		Notifier:   notifier,
		SceneState: sceneState,
		Pipeline: pipeline.Orchestrator{
			Output: &output.Coordinator{
				FocusProvider: stubFocusProvider{
					err: fmt.Errorf("focus unavailable"),
				},
			},
		},
	}

	current, target := instance.autoSwitchScene(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if target != nil {
		t.Fatalf("target = %#v, want nil", target)
	}
	if current.ID != scene.IDTerminal {
		t.Fatalf("scene = %q, want %q", current.ID, scene.IDTerminal)
	}
	if len(notifier.messages) != 0 {
		t.Fatalf("notification count = %d, want 0", len(notifier.messages))
	}
}

func TestAutoSwitchSceneDoesNotNotifyWhenSceneUnchanged(t *testing.T) {
	t.Parallel()

	sceneState, err := scene.NewState(scene.DefaultCatalog(), scene.IDGeneral)
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}

	notifier := &recordingNotifier{}
	instance := &App{
		Localizer:  i18n.NewForLocale("en_US.UTF-8"),
		Notifier:   notifier,
		SceneState: sceneState,
		Pipeline: pipeline.Orchestrator{
			Output: &output.Coordinator{
				FocusProvider: stubFocusProvider{
					target: focus.Target{WMClass: "gedit"},
				},
			},
		},
	}

	current, target := instance.autoSwitchScene(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if target == nil || target.WMClass != "gedit" {
		t.Fatalf("target = %#v", target)
	}
	if current.ID != scene.IDGeneral {
		t.Fatalf("scene = %q, want %q", current.ID, scene.IDGeneral)
	}
	if len(notifier.messages) != 0 {
		t.Fatalf("notification count = %d, want 0", len(notifier.messages))
	}
}
