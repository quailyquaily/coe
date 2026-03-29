package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"coe/internal/asr"
	"coe/internal/audio"
	"coe/internal/capabilities"
	"coe/internal/config"
	"coe/internal/control"
	"coe/internal/dictionary"
	"coe/internal/focus"
	"coe/internal/hotkey"
	"coe/internal/i18n"
	dbusipc "coe/internal/ipc/dbus"
	"coe/internal/llm"
	"coe/internal/notify"
	"coe/internal/output"
	"coe/internal/pipeline"
	"coe/internal/platform/gnome"
	"coe/internal/prompts"
	"coe/internal/scene"
)

func New(ctx context.Context, cfg config.Config) (*App, error) {
	cfg.Runtime.Mode = config.NormalizeRuntimeMode(cfg.Runtime.Mode)
	if !config.IsSupportedRuntimeMode(cfg.Runtime.Mode) {
		return nil, fmt.Errorf("unsupported runtime.mode %q", cfg.Runtime.Mode)
	}

	caps, err := capabilities.Probe(ctx)
	if err != nil {
		return nil, err
	}
	desktopRuntime := cfg.Runtime.Mode == config.RuntimeModeDesktop

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
	localizer := i18n.NewFromEnvironment()
	sceneState, err := scene.NewState(scene.DefaultCatalog(), scene.IDGeneral)
	if err != nil {
		return nil, err
	}
	routerLLM := cfg.LLM
	routerLLM.Prompt = ""
	routerLLM.PromptFile = ""
	sceneRouter, err := llm.NewCorrectorWithTemplate(routerLLM, prompts.TemplateSceneRouter)
	if err != nil {
		return nil, err
	}
	resourceClosers := make([]io.Closer, 0, 1)
	if closer, ok := asrClient.(io.Closer); ok {
		resourceClosers = append(resourceClosers, closer)
	}
	if err := output.ValidatePasteShortcut(cfg.Output.PasteShortcut); err != nil {
		return nil, err
	}
	if err := output.ValidatePasteShortcut(cfg.Output.TerminalPasteShortcut); err != nil {
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

	var portalStateStore *output.PortalStateStore
	if desktopRuntime && cfg.Output.PersistPortalAccess && caps.Portals.RemoteDesktop.Version >= 2 {
		statePath, err := output.ResolvePortalStatePath()
		if err != nil {
			return nil, err
		}
		portalStateStore = output.NewPortalStateStore(statePath)
	}

	description := describeFeature(string(caps.Hotkey.Mode), caps.Hotkey.Detail)
	if cfg.Runtime.Mode == config.RuntimeModeFcitx {
		description = "fcitx module over D-Bus"
	}
	service := hotkey.Service(hotkey.PlannedService{Description: description})
	var external *hotkey.ExternalTriggerService
	var controlSocketPath string
	startupWarnings := make([]string, 0, 2)

	if cfg.Runtime.TargetDesktop == "gnome" {
		manager := gnome.NewShortcutManager()
		if desktopRuntime {
			if caps.Hotkey.Mode == capabilities.ModeExternalBinding {
				external = hotkey.NewExternalTriggerService(description)
				service = external

				socketPath, err := control.ResolveSocketPath()
				if err != nil {
					return nil, err
				}
				controlSocketPath = socketPath

				if err := manager.EnsureTriggerShortcut(ctx, cfg.Hotkey.Name, cfg.Hotkey.PreferredAccelerator); err != nil {
					startupWarnings = append(startupWarnings, fmt.Sprintf("GNOME custom shortcut setup failed: %v", err))
				}
			}
		} else if cfg.Runtime.Mode == config.RuntimeModeFcitx {
			if err := manager.RemoveTriggerShortcut(ctx, cfg.Hotkey.Name); err != nil {
				startupWarnings = append(startupWarnings, fmt.Sprintf("GNOME custom shortcut cleanup failed: %v", err))
			}
		}
	}

	notificationService := notify.Service(notify.Disabled{})
	if cfg.Notifications.EnableSystem {
		service, err := notify.ConnectSession("coe")
		if err != nil {
			startupWarnings = append(startupWarnings, fmt.Sprintf("system notifications unavailable: %v", err))
		} else {
			notificationService = service
		}
	}

	focusProvider := focus.Provider(focus.Disabled{})
	if desktopRuntime && cfg.Output.UseGNOMEFocusHelper && cfg.Runtime.TargetDesktop == "gnome" {
		provider, err := focus.ConnectGNOMESession()
		if err != nil {
			startupWarnings = append(startupWarnings, fmt.Sprintf("GNOME focus helper unavailable: %v", err))
		} else {
			focusProvider = provider
			resourceClosers = append(resourceClosers, provider)
		}
	}

	var personalDictionary *dictionary.Dictionary
	if path := strings.TrimSpace(cfg.Dictionary.File); path != "" {
		loaded, err := dictionary.Load(path)
		switch {
		case err == nil:
			personalDictionary = loaded
		case errors.Is(err, os.ErrNotExist):
			// Optional file: missing means disabled.
		default:
			startupWarnings = append(startupWarnings, fmt.Sprintf("personal dictionary unavailable: %v", err))
		}
	}

	generalPrompt, err := llm.ResolvePrompt(cfg.LLM, prompts.TemplateLLMCorrectionGeneral, prompts.LLMTemplateData{
		Dictionary: renderDictionaryPrompt(personalDictionary, scene.IDGeneral),
	})
	if err != nil {
		return nil, err
	}
	generalCorrector, err := llm.NewCorrectorWithResolvedPrompt(cfg.LLM, generalPrompt)
	if err != nil {
		return nil, err
	}
	terminalLLM := cfg.LLM
	terminalLLM.Prompt = ""
	terminalLLM.PromptFile = ""
	terminalPrompt, err := llm.ResolvePrompt(terminalLLM, prompts.TemplateLLMCorrectionTerminal, prompts.LLMTemplateData{
		Dictionary: renderDictionaryPrompt(personalDictionary, scene.IDTerminal),
	})
	if err != nil {
		return nil, err
	}
	terminalCorrector, err := llm.NewCorrectorWithResolvedPrompt(terminalLLM, terminalPrompt)
	if err != nil {
		return nil, err
	}

	instance := &App{
		Config:            cfg,
		Caps:              caps,
		Hotkey:            service,
		ExternalHotkey:    external,
		ControlSocketPath: controlSocketPath,
		Notifier:          notificationService,
		Localizer:         localizer,
		StartupWarnings:   startupWarnings,
		Dictionary:        personalDictionary,
		SceneState:        sceneState,
		SceneCorrectors: map[string]llm.Corrector{
			scene.IDGeneral:  generalCorrector,
			scene.IDTerminal: terminalCorrector,
		},
		SceneRouter:     sceneRouter,
		resourceClosers: resourceClosers,
		dictationState:  newDictationState(),
		runtimeCommands: make(chan runtimeCommand, 16),
		Pipeline: pipeline.Orchestrator{
			Recorder:  recorder,
			ASR:       asrClient,
			Corrector: generalCorrector,
			Output: &output.Coordinator{
				ClipboardPlan:         describeFeature(string(caps.Clipboard.Mode), caps.Clipboard.Detail),
				PastePlan:             describeFeature(string(caps.Paste.Mode), caps.Paste.Detail),
				ClipboardBinary:       clipboardBinary,
				PasteBinary:           pasteBinary,
				EnableAutoPaste:       cfg.Output.EnableAutoPaste,
				PasteShortcut:         cfg.Output.PasteShortcut,
				TerminalPasteShortcut: cfg.Output.TerminalPasteShortcut,
				UsePortalClipboard:    caps.Clipboard.Mode == capabilities.ModePortal,
				UsePortalPaste:        caps.Paste.Mode == capabilities.ModePortal,
				PersistPortalAccess:   cfg.Output.PersistPortalAccess && caps.Portals.RemoteDesktop.Version >= 2,
				FocusProvider:         focusProvider,
				PortalStateStore:      portalStateStore,
			},
		},
	}

	dictationBus, err := dbusipc.ConnectSession(instance)
	if err != nil {
		instance.StartupWarnings = append(instance.StartupWarnings, fmt.Sprintf("dictation D-Bus service unavailable: %v", err))
	} else {
		instance.DictationBus = dictationBus
		instance.resourceClosers = append(instance.resourceClosers, dictationBus)
	}

	return instance, nil
}

func describeFeature(mode, detail string) string {
	if detail == "" {
		return mode
	}
	return fmt.Sprintf("%s (%s)", mode, detail)
}

func renderDictionaryPrompt(personalDictionary *dictionary.Dictionary, sceneID string) string {
	if personalDictionary == nil {
		return ""
	}
	return personalDictionary.RenderPrompt(sceneID)
}
