package app

import (
	"context"
	"fmt"
	"log/slog"

	"coe/internal/llm"
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
