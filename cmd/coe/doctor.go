package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"coe/internal/capabilities"
	"coe/internal/config"
	"coe/internal/focus"
	dbusipc "coe/internal/ipc/dbus"
	"coe/internal/platform/gnome"

	godbus "github.com/godbus/dbus/v5"
)

type doctorCheck struct {
	Name    string
	OK      bool
	Detail  string
	Problem string
}

type doctorServiceStatus struct {
	LoadState   string
	ActiveState string
	SubState    string
	Err         error
}

type doctorDictationStatus struct {
	Reachable        bool
	State            string
	SessionID        string
	Detail           string
	TriggerKey       string
	TriggerMode      string
	CurrentSceneID   string
	CurrentSceneName string
	Err              error
}

type doctorFocusHelperStatus struct {
	Installed bool
	Reachable bool
	Target    string
	Err       error
}

func runDoctor(ctx context.Context) error {
	cfg := config.Default()
	configPath, configErr := config.ResolvePath()
	configLoaded := false
	configExists := false
	if configErr == nil {
		if _, err := os.Stat(configPath); err == nil {
			configExists = true
		}
		loaded, loadErr := config.LoadOrDefault(configPath)
		if loadErr != nil {
			configErr = loadErr
		} else {
			cfg = loaded
			configLoaded = true
		}
	}

	caps, err := capabilities.Probe(ctx)
	if err != nil {
		return err
	}

	serviceStatus := probeUserService(ctx)
	dictationStatus := probeDictationDBus(ctx)
	focusHelperStatus := probeFocusHelper(ctx)
	checks := buildDoctorChecks(ctx, cfg, caps, configPath, configLoaded, configExists, configErr, serviceStatus, dictationStatus, focusHelperStatus)
	printDoctorChecks(os.Stdout, checks)
	return nil
}

func describeRuntimeMode(mode string) string {
	switch config.NormalizeRuntimeMode(mode) {
	case config.RuntimeModeFcitx:
		return "fcitx module over D-Bus"
	case config.RuntimeModeDesktop:
		return "desktop shortcut + clipboard/paste path"
	default:
		return "unknown"
	}
}

func buildDoctorChecks(
	ctx context.Context,
	cfg config.Config,
	caps capabilities.Capabilities,
	configPath string,
	configLoaded, configExists bool,
	configErr error,
	serviceStatus doctorServiceStatus,
	dictationStatus doctorDictationStatus,
	focusHelperStatus doctorFocusHelperStatus,
) []doctorCheck {
	checks := []doctorCheck{
		{
			Name:    "Config file",
			OK:      configErr == nil,
			Detail:  configFileDetail(configPath, cfg, configLoaded, configExists, configErr),
			Problem: "config file could not be resolved or loaded",
		},
		{
			Name:    "Runtime mode",
			OK:      config.IsSupportedRuntimeMode(cfg.Runtime.Mode),
			Detail:  fmt.Sprintf("mode=%s; integration=%s", cfg.Runtime.Mode, describeRuntimeMode(cfg.Runtime.Mode)),
			Problem: "runtime.mode is not supported",
		},
		{
			Name:    "Session D-Bus",
			OK:      caps.DBusSession,
			Detail:  fmt.Sprintf("session_type=%s; desktop=%s; profile=%s", nonEmpty(caps.SessionType, "unknown"), nonEmpty(caps.Desktop, "unknown"), caps.Profile),
			Problem: "session D-Bus is not available",
		},
		{
			Name:    "User service",
			OK:      serviceStatus.Err == nil && serviceStatus.LoadState != "not-found" && serviceStatus.ActiveState == "active",
			Detail:  serviceStatusDetail(serviceStatus),
			Problem: "coe.service is not active",
		},
		{
			Name:    "Coe D-Bus service",
			OK:      dictationStatus.Reachable,
			Detail:  dictationStatusDetail(dictationStatus),
			Problem: "Coe D-Bus service is not reachable",
		},
		{
			Name:    "Current scene",
			OK:      dictationStatus.Reachable && strings.TrimSpace(dictationStatus.CurrentSceneID) != "",
			Detail:  fmt.Sprintf("scene=%s; display_name=%s", nonEmpty(dictationStatus.CurrentSceneID, "missing"), nonEmpty(dictationStatus.CurrentSceneName, "missing")),
			Problem: "the Coe daemon did not report a current scene",
		},
		{
			Name:    "Audio capture",
			OK:      caps.Audio.Mode != capabilities.ModeUnavailable,
			Detail:  fmt.Sprintf("plan=%s", doctorFeatureDetail(caps.Audio)),
			Problem: "audio capture is unavailable; pw-record is missing",
		},
	}
	asrCheck := validateASRConfig(cfg.ASR)
	llmCheck := validateLLMConfig(cfg.LLM)
	checks = append(checks, asrCheck, llmCheck)

	switch config.NormalizeRuntimeMode(cfg.Runtime.Mode) {
	case config.RuntimeModeFcitx:
		checks = append(checks,
			doctorCheck{
				Name:    "Fcitx5 binary",
				OK:      caps.Fcitx.Binary.Found,
				Detail:  fmt.Sprintf("path=%s", nonEmpty(caps.Fcitx.Binary.Path, "missing")),
				Problem: "fcitx5 is not installed",
			},
			doctorCheck{
				Name:    "Fcitx5 process",
				OK:      caps.Fcitx.Running,
				Detail:  boolDetail(caps.Fcitx.Running, "process is running", "process is not running"),
				Problem: "fcitx5 is not running",
			},
			doctorCheck{
				Name:    "Fcitx addon config",
				OK:      caps.Fcitx.AddonConfigPath != "",
				Detail:  fmt.Sprintf("path=%s", nonEmpty(caps.Fcitx.AddonConfigPath, "missing")),
				Problem: "the Coe Fcitx addon config is missing",
			},
			doctorCheck{
				Name:    "Fcitx module library",
				OK:      caps.Fcitx.ModulePath != "",
				Detail:  fmt.Sprintf("path=%s", nonEmpty(caps.Fcitx.ModulePath, "missing")),
				Problem: "the Coe Fcitx module library is missing",
			},
			doctorCheck{
				Name:    "Fcitx module init",
				OK:      caps.Fcitx.InitOK,
				Detail:  fcitxInitDetail(caps.Fcitx),
				Problem: "the Coe Fcitx module has not reported init ok yet",
			},
			doctorCheck{
				Name:    "Dictation trigger key",
				OK:      dictationStatus.Reachable && strings.TrimSpace(dictationStatus.TriggerKey) != "",
				Detail:  fmt.Sprintf("configured=%s; daemon=%s", cfg.Hotkey.PreferredAccelerator, nonEmpty(dictationStatus.TriggerKey, "missing")),
				Problem: "the Coe daemon did not report a trigger key",
			},
			doctorCheck{
				Name:    "Fcitx trigger mode",
				OK:      dictationStatus.Reachable && config.IsSupportedFcitxTriggerMode(dictationStatus.TriggerMode),
				Detail:  fmt.Sprintf("configured=%s; daemon=%s", config.NormalizeFcitxTriggerMode(cfg.Hotkey.TriggerMode), nonEmpty(dictationStatus.TriggerMode, "missing")),
				Problem: "the Coe daemon did not report a supported Fcitx trigger mode",
			},
		)
	default:
		shortcutCheck := doctorCheck{
			Name:    "Desktop shortcut",
			OK:      true,
			Detail:  "not required for this desktop path",
			Problem: "",
		}
		if caps.Hotkey.Mode == capabilities.ModeExternalBinding {
			shortcutCheck = probeDesktopShortcut(ctx, cfg)
		}

		focusCheck := doctorCheck{
			Name:    "GNOME focus helper",
			OK:      true,
			Detail:  "disabled by config",
			Problem: "",
		}
		if cfg.Output.UseGNOMEFocusHelper && cfg.Runtime.TargetDesktop == "gnome" {
			focusCheck = doctorCheck{
				Name:    "GNOME focus helper",
				OK:      focusHelperStatus.Installed && focusHelperStatus.Reachable,
				Detail:  focusHelperDetail(focusHelperStatus),
				Problem: "GNOME focus helper is enabled but not reachable",
			}
		}

		checks = append(checks,
			doctorCheck{
				Name:    "Wayland session",
				OK:      caps.SessionType == "wayland",
				Detail:  fmt.Sprintf("session_type=%s", nonEmpty(caps.SessionType, "unknown")),
				Problem: "desktop mode is only polished on Wayland",
			},
			doctorCheck{
				Name:    "GNOME desktop",
				OK:      strings.Contains(caps.Desktop, "gnome"),
				Detail:  fmt.Sprintf("desktop=%s", nonEmpty(caps.Desktop, "unknown")),
				Problem: "desktop mode is only fully supported on GNOME today",
			},
			shortcutCheck,
			focusCheck,
			doctorCheck{
				Name:    "Hotkey path",
				OK:      caps.Hotkey.Mode != capabilities.ModeUnavailable,
				Detail:  fmt.Sprintf("plan=%s", doctorFeatureDetail(caps.Hotkey)),
				Problem: "no usable hotkey path was detected",
			},
			doctorCheck{
				Name:    "Clipboard path",
				OK:      caps.Clipboard.Mode != capabilities.ModeUnavailable,
				Detail:  fmt.Sprintf("plan=%s", doctorFeatureDetail(caps.Clipboard)),
				Problem: "no usable clipboard path was detected",
			},
			doctorCheck{
				Name:    "Paste path",
				OK:      !cfg.Output.EnableAutoPaste || caps.Paste.Mode != capabilities.ModeUnavailable,
				Detail:  fmt.Sprintf("plan=%s; auto_paste=%t", doctorFeatureDetail(caps.Paste), cfg.Output.EnableAutoPaste),
				Problem: "auto-paste is enabled, but no usable paste path was detected",
			},
		)
	}

	return checks
}

func printDoctorChecks(w io.Writer, checks []doctorCheck) {
	maxLabelWidth := 0
	prefixWidth := doctorCheckPrefixWidth(len(checks))
	for i, check := range checks {
		label := doctorCheckLabel(i, len(checks), check.Name)
		if len(label) > maxLabelWidth {
			maxLabelWidth = len(label)
		}
	}

	problems := make([]string, 0, len(checks))
	for i, check := range checks {
		status := "OK"
		if !check.OK {
			status = "FAIL"
			if check.Problem != "" {
				problems = append(problems, check.Problem)
			} else {
				problems = append(problems, check.Name)
			}
		}

		label := doctorCheckLabel(i, len(checks), check.Name)
		fmt.Fprintf(w, "%-*s  %6s\n", maxLabelWidth, label, status)
		if check.Detail != "" {
			fmt.Fprintf(w, "%*s%s\n", prefixWidth, "", check.Detail)
		}
	}

	fmt.Fprintln(w)
	if len(problems) == 0 {
		fmt.Fprintln(w, "Summary: healthy. No problems found.")
		return
	}

	fmt.Fprintln(w, "Summary: issues found.")
	for _, problem := range uniqueStrings(problems) {
		fmt.Fprintf(w, "- %s\n", problem)
	}
}

func doctorCheckLabel(index, total int, name string) string {
	width := len(fmt.Sprintf("%d", total))
	return fmt.Sprintf("[%*d/%d] %s", width, index+1, total, name)
}

func doctorCheckPrefixWidth(total int) int {
	width := len(fmt.Sprintf("%d", total))
	return len(fmt.Sprintf("[%*d/%d] ", width, 0, total))
}

func doctorFeatureDetail(plan capabilities.FeaturePlan) string {
	if plan.Detail == "" {
		return string(plan.Mode)
	}
	return fmt.Sprintf("%s (%s)", plan.Mode, plan.Detail)
}

func validateASRConfig(cfg config.ASRConfig) doctorCheck {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "stub"
	}

	switch provider {
	case "stub":
		return doctorCheck{Name: "ASR provider", OK: true, Detail: "provider=stub; transcription disabled", Problem: ""}
	case "openai":
		keySource, keyOK := providerAPIKeySource(cfg.APIKey, cfg.APIKeyEnv)
		modelOK := strings.TrimSpace(cfg.Model) != ""
		endpoint := nonEmpty(strings.TrimSpace(cfg.Endpoint), "https://api.openai.com/v1/audio/transcriptions")
		ok := modelOK && keyOK
		problem := ""
		if !modelOK {
			problem = "ASR provider=openai but model is empty"
		} else if !keyOK {
			problem = "ASR provider=openai but no API key is available"
		}
		return doctorCheck{
			Name:    "ASR provider",
			OK:      ok,
			Detail:  fmt.Sprintf("provider=openai; endpoint=%s; model=%s; api_key=%s", endpoint, nonEmpty(cfg.Model, "missing"), keySource),
			Problem: problem,
		}
	case "whispercpp", "whisper.cpp":
		binaryPath, binaryOK := resolveOptionalBinary(cfg.Binary, "whisper-cli")
		modelPath := strings.TrimSpace(cfg.ModelPath)
		modelOK := fileExists(modelPath)
		problem := ""
		if !binaryOK {
			problem = "ASR provider=whispercpp but whisper-cli was not found"
		} else if !modelOK {
			problem = "ASR provider=whispercpp but model_path is missing or unreadable"
		}
		return doctorCheck{
			Name:    "ASR provider",
			OK:      binaryOK && modelOK,
			Detail:  fmt.Sprintf("provider=whispercpp; binary=%s; model_path=%s; threads=%d; use_gpu=%t", nonEmpty(binaryPath, "missing"), nonEmpty(modelPath, "missing"), cfg.Threads, cfg.UseGPU),
			Problem: problem,
		}
	case "sensevoice":
		endpoint := strings.TrimSpace(cfg.Endpoint)
		if endpoint == "" {
			endpoint = "http://127.0.0.1:50000/api/v1/asr"
		}
		return doctorCheck{
			Name:    "ASR provider",
			OK:      true,
			Detail:  fmt.Sprintf("provider=sensevoice; endpoint=%s; language=%s", endpoint, nonEmpty(cfg.Language, "auto")),
			Problem: "",
		}
	default:
		return doctorCheck{
			Name:    "ASR provider",
			OK:      false,
			Detail:  fmt.Sprintf("provider=%s", provider),
			Problem: fmt.Sprintf("ASR provider %q is not supported", provider),
		}
	}
}

func validateLLMConfig(cfg config.LLMConfig) doctorCheck {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "stub"
	}

	switch provider {
	case "stub":
		return doctorCheck{Name: "LLM provider", OK: true, Detail: "provider=stub; cleanup disabled", Problem: ""}
	case "openai":
		endpointType := normalizeEndpointType(cfg.EndpointType)
		keySource, keyOK := providerAPIKeySource(cfg.APIKey, cfg.APIKeyEnv)
		modelOK := strings.TrimSpace(cfg.Model) != ""
		ok := keyOK && modelOK && (endpointType == "chat" || endpointType == "responses")
		problem := ""
		switch {
		case !modelOK:
			problem = "LLM provider=openai but model is empty"
		case !keyOK:
			problem = "LLM provider=openai but no API key is available"
		case endpointType != "chat" && endpointType != "responses":
			problem = "LLM endpoint_type must be chat or responses"
		}
		return doctorCheck{
			Name:    "LLM provider",
			OK:      ok,
			Detail:  fmt.Sprintf("provider=openai; endpoint_type=%s; endpoint=%s; model=%s; api_key=%s", endpointType, llmEndpointValue(cfg), nonEmpty(cfg.Model, "missing"), keySource),
			Problem: problem,
		}
	default:
		return doctorCheck{
			Name:    "LLM provider",
			OK:      false,
			Detail:  fmt.Sprintf("provider=%s", provider),
			Problem: fmt.Sprintf("LLM provider %q is not supported", provider),
		}
	}
}

func fcitxInitDetail(status capabilities.FcitxStatus) string {
	switch {
	case status.InitOK:
		return fmt.Sprintf("marker=%s; status=init ok", status.LogPath)
	case status.LogPresent:
		return fmt.Sprintf("marker=%s; status=present but missing init ok", status.LogPath)
	default:
		return "marker=missing"
	}
}

func configFileDetail(path string, cfg config.Config, loaded, exists bool, err error) string {
	if err != nil {
		return err.Error()
	}
	source := "default values"
	if loaded && exists {
		source = "loaded from disk"
	}
	if path == "" {
		path = "unknown"
	}
	return fmt.Sprintf("path=%s; source=%s; runtime.mode=%s", path, source, cfg.Runtime.Mode)
}

func boolDetail(value bool, okText, notOKText string) string {
	if value {
		return okText
	}
	return notOKText
}

func serviceStatusDetail(status doctorServiceStatus) string {
	if status.Err != nil {
		return status.Err.Error()
	}
	return fmt.Sprintf("load=%s; active=%s; sub=%s", nonEmpty(status.LoadState, "unknown"), nonEmpty(status.ActiveState, "unknown"), nonEmpty(status.SubState, "unknown"))
}

func dictationStatusDetail(status doctorDictationStatus) string {
	if status.Err != nil {
		return status.Err.Error()
	}
	return fmt.Sprintf(
		"service=%s; state=%s; session_id=%s; trigger_key=%s; trigger_mode=%s; scene=%s",
		dbusipc.DictationServiceName,
		nonEmpty(status.State, "unknown"),
		nonEmpty(status.SessionID, "none"),
		nonEmpty(status.TriggerKey, "missing"),
		nonEmpty(status.TriggerMode, "missing"),
		nonEmpty(status.CurrentSceneID, "missing"),
	)
}

func focusHelperDetail(status doctorFocusHelperStatus) string {
	detail := fmt.Sprintf("installed=%t; reachable=%t", status.Installed, status.Reachable)
	if status.Target != "" {
		detail += "; target=" + status.Target
	}
	if status.Err != nil {
		detail += "; error=" + status.Err.Error()
	}
	return detail
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func probeUserService(ctx context.Context) doctorServiceStatus {
	cmd := exec.CommandContext(ctx, "systemctl", "--user", "show", "coe.service", "--property=LoadState,ActiveState,SubState", "--value")
	out, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if text == "" {
			text = err.Error()
		}
		return doctorServiceStatus{Err: fmt.Errorf("systemctl --user show coe.service failed: %s", text)}
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	status := doctorServiceStatus{}
	if len(lines) > 0 {
		status.LoadState = strings.TrimSpace(lines[0])
	}
	if len(lines) > 1 {
		status.ActiveState = strings.TrimSpace(lines[1])
	}
	if len(lines) > 2 {
		status.SubState = strings.TrimSpace(lines[2])
	}
	return status
}

func probeDictationDBus(ctx context.Context) doctorDictationStatus {
	conn, err := godbus.ConnectSessionBus()
	if err != nil {
		return doctorDictationStatus{Err: fmt.Errorf("connect session bus failed: %w", err)}
	}
	defer conn.Close()

	obj := conn.Object(dbusipc.DictationServiceName, godbus.ObjectPath(dbusipc.DictationObjectPath))
	status := doctorDictationStatus{}
	if err := obj.CallWithContext(ctx, dbusipc.DictationInterface+".Status", 0).Store(&status.State, &status.SessionID, &status.Detail); err != nil {
		return doctorDictationStatus{Err: fmt.Errorf("call %s.Status failed: %w", dbusipc.DictationInterface, err)}
	}
	if err := obj.CallWithContext(ctx, dbusipc.DictationInterface+".TriggerKey", 0).Store(&status.TriggerKey); err != nil {
		return doctorDictationStatus{Err: fmt.Errorf("call %s.TriggerKey failed: %w", dbusipc.DictationInterface, err)}
	}
	if err := obj.CallWithContext(ctx, dbusipc.DictationInterface+".TriggerMode", 0).Store(&status.TriggerMode); err != nil {
		return doctorDictationStatus{Err: fmt.Errorf("call %s.TriggerMode failed: %w", dbusipc.DictationInterface, err)}
	}
	if err := obj.CallWithContext(ctx, dbusipc.DictationInterface+".CurrentScene", 0).Store(&status.CurrentSceneID, &status.CurrentSceneName); err != nil {
		return doctorDictationStatus{Err: fmt.Errorf("call %s.CurrentScene failed: %w", dbusipc.DictationInterface, err)}
	}
	status.Reachable = true
	return status
}

func probeFocusHelper(ctx context.Context) doctorFocusHelperStatus {
	status := doctorFocusHelperStatus{
		Installed: fileExists(filepathJoinHome(".local/share/gnome-shell/extensions/coe-focus-helper@mistermorph.com/metadata.json")),
	}

	provider, err := focus.ConnectGNOMESession()
	if err != nil {
		status.Err = err
		return status
	}
	defer provider.Close()

	target, err := provider.Focused(ctx)
	if err != nil {
		status.Err = err
		return status
	}
	status.Reachable = true
	status.Target = target.Summary()
	return status
}

func probeDesktopShortcut(ctx context.Context, cfg config.Config) doctorCheck {
	manager := gnome.NewShortcutManager()
	shortcut, found, err := manager.LookupTriggerShortcut(ctx, cfg.Hotkey.Name)
	if err != nil {
		return doctorCheck{
			Name:    "Desktop shortcut",
			OK:      false,
			Detail:  err.Error(),
			Problem: "failed to inspect the GNOME custom shortcut fallback",
		}
	}
	if !found {
		return doctorCheck{
			Name:    "Desktop shortcut",
			OK:      false,
			Detail:  fmt.Sprintf("name=%s; binding=%s", cfg.Hotkey.Name, cfg.Hotkey.PreferredAccelerator),
			Problem: "the GNOME custom shortcut fallback was not found",
		}
	}
	return doctorCheck{
		Name:    "Desktop shortcut",
		OK:      true,
		Detail:  fmt.Sprintf("name=%s; binding=%s; command=%s", shortcut.Name, shortcut.Binding, shortcut.Command),
		Problem: "",
	}
}

func providerAPIKeySource(explicit, envName string) (string, bool) {
	if strings.TrimSpace(explicit) != "" {
		return "config", true
	}
	envName = strings.TrimSpace(envName)
	if envName == "" {
		envName = "OPENAI_API_KEY"
	}
	if strings.TrimSpace(os.Getenv(envName)) != "" {
		return "env:" + envName, true
	}
	return "missing", false
}

func resolveOptionalBinary(configured, fallback string) (string, bool) {
	value := strings.TrimSpace(configured)
	if value == "" {
		value = fallback
	}
	if value == "" {
		return "", false
	}
	if strings.ContainsRune(value, os.PathSeparator) {
		if fileExists(value) {
			return value, true
		}
		return value, false
	}
	path, err := exec.LookPath(value)
	if err != nil {
		return "", false
	}
	return path, true
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func normalizeEndpointType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "response", "responses":
		return "responses"
	case "chat":
		return "chat"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func llmEndpointValue(cfg config.LLMConfig) string {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint != "" {
		return endpoint
	}
	if normalizeEndpointType(cfg.EndpointType) == "chat" {
		return "https://api.openai.com/v1"
	}
	return "https://api.openai.com/v1/responses"
}

func filepathJoinHome(relative string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, relative)
}
