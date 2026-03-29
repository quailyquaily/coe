package dbus

import (
	"context"
	"errors"
	"testing"
)

type stubHandler struct {
	toggleCalls int
	startCalls  int
	stopCalls   int
	status      Status
	triggerKey  string
	sceneID     string
	sceneName   string
	scenesJSON  string
	switchScene string
	err         error
}

func (h *stubHandler) Toggle(context.Context) error {
	h.toggleCalls++
	return h.err
}

func (h *stubHandler) Start(context.Context) error {
	h.startCalls++
	return h.err
}

func (h *stubHandler) Stop(context.Context) error {
	h.stopCalls++
	return h.err
}

func (h *stubHandler) Status(context.Context) Status {
	return h.status
}

func (h *stubHandler) TriggerKey(context.Context) string {
	return h.triggerKey
}

func (h *stubHandler) CurrentScene(context.Context) (string, string) {
	return h.sceneID, h.sceneName
}

func (h *stubHandler) ListScenes(context.Context) string {
	return h.scenesJSON
}

func (h *stubHandler) SwitchScene(_ context.Context, sceneID string) error {
	h.switchScene = sceneID
	return h.err
}

func TestDictationObjectToggleDelegatesToHandler(t *testing.T) {
	handler := &stubHandler{}
	object := &dictationObject{handler: handler}

	if err := object.Toggle(); err != nil {
		t.Fatalf("Toggle() error = %v, want nil", err)
	}
	if handler.toggleCalls != 1 {
		t.Fatalf("toggleCalls = %d, want 1", handler.toggleCalls)
	}
}

func TestDictationObjectToggleReturnsDBusError(t *testing.T) {
	handler := &stubHandler{err: errors.New("boom")}
	object := &dictationObject{handler: handler}

	if err := object.Toggle(); err == nil {
		t.Fatal("Toggle() error = nil, want D-Bus error")
	}
}

func TestDictationObjectStatusReturnsSnapshot(t *testing.T) {
	handler := &stubHandler{
		status: Status{
			State:     "recording",
			SessionID: "session-1",
			Detail:    "capturing",
		},
	}
	object := &dictationObject{handler: handler}

	state, sessionID, detail, err := object.Status()
	if err != nil {
		t.Fatalf("Status() error = %v, want nil", err)
	}
	if state != handler.status.State || sessionID != handler.status.SessionID || detail != handler.status.Detail {
		t.Fatalf("Status() = (%q, %q, %q), want (%q, %q, %q)", state, sessionID, detail, handler.status.State, handler.status.SessionID, handler.status.Detail)
	}
}

func TestDictationObjectTriggerKeyReturnsHandlerValue(t *testing.T) {
	handler := &stubHandler{triggerKey: "<Shift><Super>d"}
	object := &dictationObject{handler: handler}

	triggerKey, err := object.TriggerKey()
	if err != nil {
		t.Fatalf("TriggerKey() error = %v, want nil", err)
	}
	if triggerKey != handler.triggerKey {
		t.Fatalf("TriggerKey() = %q, want %q", triggerKey, handler.triggerKey)
	}
}

func TestDictationObjectCurrentSceneReturnsHandlerValue(t *testing.T) {
	handler := &stubHandler{sceneID: "terminal", sceneName: "Terminal"}
	object := &dictationObject{handler: handler}

	sceneID, sceneName, err := object.CurrentScene()
	if err != nil {
		t.Fatalf("CurrentScene() error = %v, want nil", err)
	}
	if sceneID != handler.sceneID || sceneName != handler.sceneName {
		t.Fatalf("CurrentScene() = (%q, %q), want (%q, %q)", sceneID, sceneName, handler.sceneID, handler.sceneName)
	}
}

func TestDictationObjectListScenesReturnsHandlerValue(t *testing.T) {
	handler := &stubHandler{scenesJSON: `[{"id":"general"}]`}
	object := &dictationObject{handler: handler}

	scenesJSON, err := object.ListScenes()
	if err != nil {
		t.Fatalf("ListScenes() error = %v, want nil", err)
	}
	if scenesJSON != handler.scenesJSON {
		t.Fatalf("ListScenes() = %q, want %q", scenesJSON, handler.scenesJSON)
	}
}

func TestDictationObjectSwitchSceneDelegatesToHandler(t *testing.T) {
	handler := &stubHandler{}
	object := &dictationObject{handler: handler}

	if err := object.SwitchScene("terminal"); err != nil {
		t.Fatalf("SwitchScene() error = %v, want nil", err)
	}
	if handler.switchScene != "terminal" {
		t.Fatalf("switchScene = %q, want %q", handler.switchScene, "terminal")
	}
}
