package app

import (
	"io"
	"sync/atomic"

	"coe/internal/capabilities"
	"coe/internal/config"
	"coe/internal/dictionary"
	"coe/internal/hotkey"
	"coe/internal/i18n"
	dbusipc "coe/internal/ipc/dbus"
	"coe/internal/llm"
	"coe/internal/notify"
	"coe/internal/pipeline"
	"coe/internal/scene"
)

type App struct {
	Config            config.Config
	Caps              capabilities.Capabilities
	Hotkey            hotkey.Service
	ExternalHotkey    *hotkey.ExternalTriggerService
	ControlSocketPath string
	Notifier          notify.Service
	Localizer         i18n.Localizer
	StartupWarnings   []string
	Dictionary        *dictionary.Dictionary
	Pipeline          pipeline.Orchestrator
	SceneState        *scene.State
	SceneCorrectors   map[string]llm.Corrector
	SceneRouter       llm.Corrector
	DictationBus      *dbusipc.Service
	resourceClosers   []io.Closer
	dictationState    *dictationState
	runtimeCommands   chan runtimeCommand
	runtimeRunning    atomic.Bool
}
