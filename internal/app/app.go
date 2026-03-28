package app

import (
	"io"
	"sync/atomic"

	"coe/internal/capabilities"
	"coe/internal/config"
	"coe/internal/hotkey"
	dbusipc "coe/internal/ipc/dbus"
	"coe/internal/notify"
	"coe/internal/pipeline"
)

type App struct {
	Config            config.Config
	Caps              capabilities.Capabilities
	Hotkey            hotkey.Service
	ExternalHotkey    *hotkey.ExternalTriggerService
	ControlSocketPath string
	Notifier          notify.Service
	StartupWarnings   []string
	Pipeline          pipeline.Orchestrator
	DictationBus      *dbusipc.Service
	resourceClosers   []io.Closer
	dictationState    *dictationState
	runtimeCommands   chan runtimeCommand
	runtimeRunning    atomic.Bool
}
