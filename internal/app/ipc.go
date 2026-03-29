package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"coe/internal/config"
	"coe/internal/control"
	dbusipc "coe/internal/ipc/dbus"
	"coe/internal/scene"
)

func (a *App) handleControl(_ context.Context, req control.Request) control.Response {
	switch req.Command {
	case control.CommandPing:
		active := a.triggerActive()
		return control.Response{OK: true, Message: "pong", Active: active}
	case control.CommandTriggerStart:
		changed, err := a.triggerStart()
		if err != nil {
			return control.Response{OK: false, Message: err.Error()}
		}
		return control.Response{
			OK:      true,
			Message: pickMessage(changed, "trigger started", "trigger already active"),
			Active:  a.triggerActive(),
		}
	case control.CommandTriggerStop:
		changed, err := a.triggerStop()
		if err != nil {
			return control.Response{OK: false, Message: err.Error()}
		}
		return control.Response{
			OK:      true,
			Message: pickMessage(changed, "trigger stopped", "trigger already inactive"),
			Active:  a.triggerActive(),
		}
	case control.CommandTriggerToggle:
		active, err := a.triggerToggle()
		if err != nil {
			return control.Response{OK: false, Message: err.Error()}
		}
		return control.Response{
			OK:      true,
			Message: pickMessage(active, "trigger toggled on", "trigger toggled off"),
			Active:  active,
		}
	case control.CommandTriggerStatus:
		active := a.triggerActive()
		return control.Response{
			OK:      true,
			Message: pickMessage(active, "trigger active", "trigger inactive"),
			Active:  active,
		}
	default:
		return control.Response{
			OK:      false,
			Message: fmt.Sprintf("unsupported control command %q", req.Command),
			Active:  a.triggerActive(),
		}
	}
}

func (a *App) Toggle(context.Context) error {
	_, err := a.triggerToggleFrom("fcitx-module")
	return err
}

func (a *App) Start(context.Context) error {
	_, err := a.triggerStartFrom("fcitx-module")
	return err
}

func (a *App) Stop(context.Context) error {
	_, err := a.triggerStopFrom("fcitx-module")
	return err
}

func (a *App) Status(context.Context) dbusipc.Status {
	if a.dictationState == nil {
		return a.withSceneDetail(dbusipc.Status{State: "idle"})
	}
	return a.withSceneDetail(a.dictationState.Snapshot())
}

func (a *App) TriggerKey(context.Context) string {
	if value := strings.TrimSpace(a.Config.Hotkey.PreferredAccelerator); value != "" {
		return value
	}
	return config.Default().Hotkey.PreferredAccelerator
}

func (a *App) CurrentScene(context.Context) (string, string) {
	current := a.currentScene()
	return current.ID, a.sceneDisplayName(current)
}

func (a *App) ListScenes(context.Context) string {
	payload, err := json.Marshal(scene.ListDisplayScenes(a.SceneState, a.Localizer))
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func (a *App) SwitchScene(_ context.Context, sceneID string) error {
	_, _, err := a.switchSceneByID(sceneID)
	return err
}

func (a *App) triggerToggle() (bool, error) {
	return a.triggerToggleFrom("ipc")
}

func (a *App) triggerToggleFrom(source string) (bool, error) {
	response, err := a.executeRuntimeCommand(context.Background(), runtimeCommandToggle, source)
	if err != nil {
		return false, err
	}
	return response.Active, nil
}

func (a *App) triggerStart() (bool, error) {
	return a.triggerStartFrom("ipc")
}

func (a *App) triggerStartFrom(source string) (bool, error) {
	response, err := a.executeRuntimeCommand(context.Background(), runtimeCommandStart, source)
	if err != nil {
		return false, err
	}
	return response.Changed, nil
}

func (a *App) triggerStop() (bool, error) {
	return a.triggerStopFrom("ipc")
}

func (a *App) triggerStopFrom(source string) (bool, error) {
	response, err := a.executeRuntimeCommand(context.Background(), runtimeCommandStop, source)
	if err != nil {
		return false, err
	}
	return response.Changed, nil
}

func (a *App) triggerActive() bool {
	status := a.Status(context.Background())
	return status.State == "recording"
}

func (a *App) emitStateChanged(logger *slog.Logger, status dbusipc.Status) {
	status = a.withSceneDetail(status)
	if a.DictationBus == nil {
		return
	}
	if err := a.DictationBus.EmitStateChanged(status); err != nil {
		logger.Warn("dictation D-Bus state emit failed", "error", err)
	}
}

func (a *App) emitResultReady(logger *slog.Logger, sessionID, text string) {
	if a.DictationBus == nil {
		return
	}
	if err := a.DictationBus.EmitResultReady(sessionID, text); err != nil {
		logger.Warn("dictation D-Bus result emit failed", "error", err)
	}
}

func (a *App) emitDictationError(logger *slog.Logger, sessionID, message string) {
	if a.DictationBus == nil {
		return
	}
	if err := a.DictationBus.EmitError(sessionID, message); err != nil {
		logger.Warn("dictation D-Bus error emit failed", "error", err)
	}
}

func (a *App) emitSceneChanged(logger *slog.Logger, currentScene scene.Scene) {
	if a.DictationBus == nil {
		return
	}
	if err := a.DictationBus.EmitSceneChanged(currentScene.ID, a.sceneDisplayName(currentScene)); err != nil {
		logger.Warn("dictation D-Bus scene emit failed", "error", err)
	}
}

func (a *App) withSceneDetail(status dbusipc.Status) dbusipc.Status {
	current := a.currentScene()
	if current.ID == "" {
		return status
	}

	detail := strings.TrimSpace(status.Detail)
	sceneDetail := "scene=" + current.ID
	switch {
	case detail == "":
		status.Detail = sceneDetail
	case strings.Contains(detail, sceneDetail):
		status.Detail = detail
	default:
		status.Detail = detail + "; " + sceneDetail
	}
	return status
}

func pickMessage(condition bool, yes, no string) string {
	if condition {
		return yes
	}
	return no
}

func (a *App) executeRuntimeCommand(ctx context.Context, kind runtimeCommandType, source string) (runtimeCommandResponse, error) {
	if !a.runtimeRunning.Load() {
		return runtimeCommandResponse{}, errors.New("dictation runtime loop is not ready")
	}

	command := runtimeCommand{
		Type:   kind,
		Source: source,
		Reply:  make(chan runtimeCommandResponse, 1),
	}

	select {
	case a.runtimeCommands <- command:
	case <-ctx.Done():
		return runtimeCommandResponse{}, ctx.Err()
	}

	select {
	case response := <-command.Reply:
		if response.Err != nil {
			return response, response.Err
		}
		return response, nil
	case <-ctx.Done():
		return runtimeCommandResponse{}, ctx.Err()
	}
}
