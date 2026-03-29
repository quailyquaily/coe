package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"coe/internal/focus"
	"coe/internal/llm"
	"coe/internal/pipeline"
	"coe/internal/scene"
)

type sceneCommandOutcome struct {
	Attempted    bool
	Handled      bool
	Changed      bool
	Scene        scene.Scene
	MatchedAlias string
}

func (a *App) currentScene() scene.Scene {
	if a.SceneState == nil {
		return scene.Scene{ID: scene.IDGeneral}
	}
	return a.SceneState.Current()
}

func (a *App) sceneDisplayName(value scene.Scene) string {
	if value.ID == "" {
		return ""
	}
	return value.DisplayName(a.Localizer)
}

func (a *App) focusTarget(ctx context.Context) (*focus.Target, error) {
	if a.Pipeline.Output == nil {
		return nil, fmt.Errorf("output coordinator is not configured")
	}

	target, err := a.Pipeline.Output.FocusedTarget(ctx)
	if err != nil {
		return nil, err
	}
	return &target, nil
}

func (a *App) autoSwitchScene(ctx context.Context, logger *slog.Logger) (scene.Scene, *focus.Target) {
	current := a.currentScene()
	if a.SceneState == nil {
		return current, nil
	}

	target, err := a.focusTarget(ctx)
	if err != nil {
		return current, nil
	}

	targetSceneID := scene.IDGeneral
	if focus.LooksLikeTerminal(*target) {
		targetSceneID = scene.IDTerminal
	}

	changed, current, err := a.SceneState.SwitchTo(targetSceneID)
	if err != nil {
		logger.Warn("scene auto-switch failed", "error", err, "focus_target", target.Summary())
		return a.currentScene(), target
	}
	if changed {
		a.emitSceneChanged(logger, current)
		a.emitSceneSwitchedNotification(logger, a.sceneDisplayName(current))
		logger.Info(
			"scene auto-switched",
			"scene", current.ID,
			"display_name", a.sceneDisplayName(current),
			"focus_target", target.Summary(),
		)
	}

	return current, target
}

func (a *App) correctorForScene(id string) llm.Corrector {
	if corrector, ok := a.SceneCorrectors[id]; ok {
		return corrector
	}
	if corrector, ok := a.SceneCorrectors[scene.IDGeneral]; ok {
		return corrector
	}
	return a.Pipeline.Corrector
}

func (a *App) attemptSceneCommand(ctx context.Context, correctedText string) (sceneCommandOutcome, error) {
	if a.SceneState == nil || a.SceneRouter == nil || !scene.LooksLikeSwitchCommand(correctedText) {
		return sceneCommandOutcome{}, nil
	}

	outcome := sceneCommandOutcome{Attempted: true}
	input, err := scene.BuildRouteInput(a.currentScene(), a.SceneState.List(), a.Localizer, correctedText)
	if err != nil {
		return outcome, err
	}

	routed, err := a.SceneRouter.Correct(ctx, input)
	if err != nil {
		return outcome, err
	}

	response, target, err := scene.ParseRouteOutput(routed.Text, a.SceneState)
	if err != nil {
		return outcome, nil
	}

	changed, current, err := a.SceneState.SwitchTo(target.ID)
	if err != nil {
		return outcome, err
	}

	outcome.Handled = true
	outcome.Changed = changed
	outcome.Scene = current
	outcome.MatchedAlias = response.MatchedAlias
	return outcome, nil
}

func (a *App) normalizeForScene(result pipeline.Result, sceneID string) pipeline.Result {
	if a.Dictionary == nil {
		return result
	}
	text := strings.TrimSpace(result.Corrected)
	if text == "" {
		return result
	}

	normalized := strings.TrimSpace(a.Dictionary.Normalize(sceneID, text))
	if normalized != "" {
		result.Corrected = normalized
	}
	return result
}

func (a *App) switchSceneByID(sceneID string) (bool, scene.Scene, error) {
	if a.SceneState == nil {
		return false, scene.Scene{}, fmt.Errorf("scene state is not configured")
	}

	changed, current, err := a.SceneState.SwitchTo(sceneID)
	if err != nil {
		return false, scene.Scene{}, err
	}

	logger := slog.Default()
	if changed {
		a.emitSceneChanged(logger, current)
	}
	a.emitSceneSwitchedNotification(logger, a.sceneDisplayName(current))
	return changed, current, nil
}
