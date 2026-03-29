package app

import (
	"context"
	"encoding/json"
	"testing"

	"coe/internal/i18n"
	"coe/internal/llm"
	"coe/internal/notify"
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
