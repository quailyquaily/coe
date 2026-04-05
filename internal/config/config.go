package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const envConfigPath = "COE_CONFIG"
const defaultDictionaryRelativePath = "./dictionary.yaml"

const (
	RuntimeModeDesktop = "desktop"
	RuntimeModeFcitx   = "fcitx"

	FcitxTriggerModeToggle = "toggle"
	FcitxTriggerModeHold   = "hold"
)

var acceleratorModifierOrder = map[string]int{
	"Control": 0,
	"Alt":     1,
	"Shift":   2,
	"Super":   3,
}

var forbiddenAccelerators = map[string]string{
	"<Control>c":              "Ctrl+C is reserved by terminals for interrupt",
	"<Control>d":              "Ctrl+D is reserved by terminals for EOF",
	"<Control>z":              "Ctrl+Z is reserved by terminals for suspend",
	"<Control>q":              "Ctrl+Q is a common quit shortcut",
	"<Control>s":              "Ctrl+S is a common save shortcut",
	"<Alt>Tab":                "Alt+Tab is reserved for window switching",
	"<Alt><Shift>Tab":         "Shift+Alt+Tab is reserved for reverse window switching",
	"<Super>Tab":              "Super+Tab is reserved for shell switching",
	"<Shift><Super>Tab":       "Shift+Super+Tab is reserved for reverse shell switching",
	"<Alt>F4":                 "Alt+F4 is reserved for closing windows",
	"<Super>d":                "Super+D is reserved by GNOME",
	"<Super>l":                "Super+L is reserved for screen locking",
	"<Super>space":            "Super+Space is reserved for input source switching",
	"<Shift><Super>space":     "Shift+Super+Space is reserved for reverse input source switching",
	"<Control><Alt>Delete":    "Ctrl+Alt+Delete is reserved by the system",
	"<Control><Alt>BackSpace": "Ctrl+Alt+BackSpace is reserved by the system",
	"<Control><Shift>Escape":  "Ctrl+Shift+Escape is a common system shortcut",
	"<Super>Escape":           "Super+Escape is reserved by GNOME",
}

type Config struct {
	Runtime       RuntimeConfig       `yaml:"runtime"`
	Hotkey        HotkeyConfig        `yaml:"hotkey"`
	Audio         AudioConfig         `yaml:"audio"`
	ASR           ASRConfig           `yaml:"asr"`
	LLM           LLMConfig           `yaml:"llm"`
	Dictionary    DictionaryConfig    `yaml:"dictionary"`
	Output        OutputConfig        `yaml:"output"`
	Notifications NotificationsConfig `yaml:"notifications"`
}

type RuntimeConfig struct {
	Mode          string `yaml:"mode"`
	TargetDesktop string `yaml:"target_desktop"`
	LogLevel      string `yaml:"log_level"`
}

type HotkeyConfig struct {
	Name                 string `yaml:"name"`
	PreferredAccelerator string `yaml:"preferred_accelerator"`
	TriggerMode          string `yaml:"trigger_mode"`
}

type AudioConfig struct {
	RecorderBinary string `yaml:"recorder_binary"`
	SampleRate     int    `yaml:"sample_rate"`
	Channels       int    `yaml:"channels"`
	Format         string `yaml:"format"`
}

type ASRConfig struct {
	Provider   string `yaml:"provider"`
	Endpoint   string `yaml:"endpoint"`
	Model      string `yaml:"model"`
	Language   string `yaml:"language"`
	Prompt     string `yaml:"prompt"`
	PromptFile string `yaml:"prompt_file"`
	APIKey     string `yaml:"api_key"`
	APIKeyEnv  string `yaml:"api_key_env"`
	Binary     string `yaml:"binary"`
	ModelPath  string `yaml:"model_path"`
	Threads    int    `yaml:"threads"`
	UseGPU     bool   `yaml:"use_gpu"`
}

type LLMConfig struct {
	Provider     string `yaml:"provider"`
	Endpoint     string `yaml:"endpoint"`
	EndpointType string `yaml:"endpoint_type"`
	Model        string `yaml:"model"`
	APIKey       string `yaml:"api_key"`
	APIKeyEnv    string `yaml:"api_key_env"`
	Prompt       string `yaml:"prompt"`
	PromptFile   string `yaml:"prompt_file"`
}

type DictionaryConfig struct {
	File string `yaml:"file"`
}

type OutputConfig struct {
	EnableAutoPaste       bool   `yaml:"enable_auto_paste"`
	PasteShortcut         string `yaml:"paste_shortcut"`
	TerminalPasteShortcut string `yaml:"terminal_paste_shortcut"`
	UseGNOMEFocusHelper   bool   `yaml:"use_gnome_focus_helper"`
	PersistPortalAccess   bool   `yaml:"persist_portal_access"`
	ClipboardBinary       string `yaml:"clipboard_binary"`
	PasteBinary           string `yaml:"paste_binary"`
}

type NotificationsConfig struct {
	EnableSystem           bool `yaml:"enable_system"`
	NotifyOnComplete       bool `yaml:"notify_on_complete"`
	NotifyOnRecordingStart bool `yaml:"notify_on_recording_start"`
}

type InitResult struct {
	ConfigWritten     bool
	ConfigUpdated     bool
	DictionaryWritten bool
	DictionaryPath    string
}

func Default() Config {
	return Config{
		Runtime: RuntimeConfig{
			Mode:          RuntimeModeFcitx,
			TargetDesktop: "gnome",
			LogLevel:      "info",
		},
		Hotkey: HotkeyConfig{
			Name:                 "coe-trigger",
			PreferredAccelerator: "<Shift><Super>d",
			TriggerMode:          FcitxTriggerModeToggle,
		},
		Audio: AudioConfig{
			RecorderBinary: "pw-record",
			SampleRate:     16000,
			Channels:       1,
			Format:         "s16",
		},
		ASR: ASRConfig{
			Provider:   "openai",
			Endpoint:   "https://api.openai.com/v1/audio/transcriptions",
			Model:      "gpt-4o-mini-transcribe",
			Language:   "zh",
			Prompt:     "",
			PromptFile: "",
			APIKey:     "",
			APIKeyEnv:  "OPENAI_API_KEY",
			Binary:     "whisper-cli",
			ModelPath:  "",
			Threads:    0,
			UseGPU:     false,
		},
		LLM: LLMConfig{
			Provider:     "openai",
			Endpoint:     "https://api.openai.com/v1",
			EndpointType: "chat",
			Model:        "gpt-4o-mini",
			APIKeyEnv:    "OPENAI_API_KEY",
			Prompt:       "",
			PromptFile:   "",
		},
		Dictionary: DictionaryConfig{
			File: "",
		},
		Output: OutputConfig{
			EnableAutoPaste:       true,
			PasteShortcut:         "ctrl+v",
			TerminalPasteShortcut: "ctrl+shift+v",
			UseGNOMEFocusHelper:   true,
			PersistPortalAccess:   true,
			ClipboardBinary:       "wl-copy",
			PasteBinary:           "",
		},
		Notifications: NotificationsConfig{
			EnableSystem:           true,
			NotifyOnComplete:       false,
			NotifyOnRecordingStart: false,
		},
	}
}

func ResolvePath() (string, error) {
	if path := os.Getenv(envConfigPath); path != "" {
		return path, nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "coe", "config.yaml"), nil
}

func ResolveEnvPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "coe", "env"), nil
}

func DefaultDictionaryPath(configPath string) string {
	return dictionaryPathForConfig(configPath)
}

func LoadEnvFile() error {
	path, err := ResolveEnvPath()
	if err != nil {
		return err
	}
	return loadEnvFile(path)
}

func LoadOrDefault(path string) (Config, error) {
	cfg, err := Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}

	return cfg, err
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	cfg.Runtime.Mode = NormalizeRuntimeMode(cfg.Runtime.Mode)
	if !IsSupportedRuntimeMode(cfg.Runtime.Mode) {
		return Config{}, errors.New("unsupported runtime.mode: " + cfg.Runtime.Mode)
	}
	cfg.Hotkey.TriggerMode = NormalizeFcitxTriggerMode(cfg.Hotkey.TriggerMode)
	if !IsSupportedFcitxTriggerMode(cfg.Hotkey.TriggerMode) {
		return Config{}, errors.New("unsupported hotkey.trigger_mode: " + cfg.Hotkey.TriggerMode)
	}
	cfg.ASR.PromptFile = resolveConfigRelativePath(path, cfg.ASR.PromptFile)
	cfg.LLM.PromptFile = resolveConfigRelativePath(path, cfg.LLM.PromptFile)
	cfg.Dictionary.File = resolveConfigRelativePath(path, cfg.Dictionary.File)

	return cfg, nil
}

func NormalizeRuntimeMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return RuntimeModeFcitx
	}
	return value
}

func IsSupportedRuntimeMode(value string) bool {
	switch NormalizeRuntimeMode(value) {
	case RuntimeModeDesktop, RuntimeModeFcitx:
		return true
	default:
		return false
	}
}

func NormalizeFcitxTriggerMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return FcitxTriggerModeToggle
	}
	return value
}

func NormalizePreferredAccelerator(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("hotkey.preferred_accelerator cannot be empty")
	}

	modifiers, key, err := parseAccelerator(value)
	if err != nil {
		return "", err
	}
	allowsModifierless := allowsModifierlessAcceleratorKey(key)
	if len(modifiers) == 0 && !allowsModifierless {
		return "", errors.New("hotkey.preferred_accelerator must include at least one modifier")
	}

	hasPrimaryModifier := false
	for _, modifier := range modifiers {
		if modifier == "Control" || modifier == "Alt" || modifier == "Super" {
			hasPrimaryModifier = true
			break
		}
	}
	if len(modifiers) > 0 && !hasPrimaryModifier && !allowsModifierless {
		return "", errors.New("hotkey.preferred_accelerator must include Control, Alt, or Super")
	}
	if key == "Return" || key == "Escape" {
		return "", fmt.Errorf("hotkey.preferred_accelerator cannot use %s", key)
	}

	sort.Slice(modifiers, func(i, j int) bool {
		return acceleratorModifierOrder[modifiers[i]] < acceleratorModifierOrder[modifiers[j]]
	})

	var b strings.Builder
	for _, modifier := range modifiers {
		b.WriteString("<")
		b.WriteString(modifier)
		b.WriteString(">")
	}
	b.WriteString(key)
	normalized := b.String()

	if reason, forbidden := forbiddenAccelerators[normalized]; forbidden {
		return "", fmt.Errorf("hotkey.preferred_accelerator %q is not allowed: %s", normalized, reason)
	}

	return normalized, nil
}

func parseAccelerator(value string) ([]string, string, error) {
	if strings.HasPrefix(value, "<") {
		return parseBracketAccelerator(value)
	}
	if !strings.Contains(value, "+") {
		key, err := normalizeAcceleratorKey(value)
		if err != nil {
			return nil, "", err
		}
		return nil, key, nil
	}
	return parsePlusAccelerator(value)
}

func parseBracketAccelerator(value string) ([]string, string, error) {
	rest := strings.TrimSpace(value)
	var modifiers []string

	for strings.HasPrefix(rest, "<") {
		end := strings.Index(rest, ">")
		if end <= 1 {
			return nil, "", fmt.Errorf("invalid accelerator %q", value)
		}
		modifier, err := normalizeAcceleratorModifier(rest[1:end])
		if err != nil {
			return nil, "", err
		}
		modifiers = append(modifiers, modifier)
		rest = strings.TrimSpace(rest[end+1:])
	}

	key, err := normalizeAcceleratorKey(rest)
	if err != nil {
		return nil, "", err
	}
	return uniqueModifiers(modifiers), key, nil
}

func parsePlusAccelerator(value string) ([]string, string, error) {
	parts := strings.Split(value, "+")
	if len(parts) < 2 {
		return nil, "", fmt.Errorf("invalid accelerator %q", value)
	}

	modifiers := make([]string, 0, len(parts)-1)
	for _, part := range parts[:len(parts)-1] {
		modifier, err := normalizeAcceleratorModifier(part)
		if err != nil {
			return nil, "", err
		}
		modifiers = append(modifiers, modifier)
	}

	key, err := normalizeAcceleratorKey(parts[len(parts)-1])
	if err != nil {
		return nil, "", err
	}
	return uniqueModifiers(modifiers), key, nil
}

func normalizeAcceleratorModifier(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ctrl", "control":
		return "Control", nil
	case "alt":
		return "Alt", nil
	case "shift":
		return "Shift", nil
	case "super", "meta", "win", "windows":
		return "Super", nil
	default:
		return "", fmt.Errorf("unsupported modifier %q", strings.TrimSpace(value))
	}
}

func normalizeAcceleratorKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("accelerator key cannot be empty")
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return "", fmt.Errorf("unsupported key %q", value)
	}

	if len(value) == 1 {
		r := rune(value[0])
		switch {
		case unicode.IsLetter(r):
			return strings.ToLower(value), nil
		case unicode.IsDigit(r):
			return value, nil
		case unicode.IsPrint(r) && !unicode.IsSpace(r):
			return value, nil
		}
	}

	lowered := strings.ToLower(value)
	switch lowered {
	case "enter", "return":
		return "Return", nil
	case "esc", "escape":
		return "Escape", nil
	case "space", "spacebar":
		return "space", nil
	case "tab":
		return "Tab", nil
	case "backspace":
		return "BackSpace", nil
	case "delete", "del":
		return "Delete", nil
	case "insert", "ins":
		return "Insert", nil
	case "home":
		return "Home", nil
	case "end":
		return "End", nil
	case "pageup", "page_up":
		return "Page_Up", nil
	case "pagedown", "page_down":
		return "Page_Down", nil
	case "left":
		return "Left", nil
	case "right":
		return "Right", nil
	case "up":
		return "Up", nil
	case "down":
		return "Down", nil
	case "print":
		return "Print", nil
	}

	if len(lowered) >= 2 && lowered[0] == 'f' {
		n, err := strconv.Atoi(lowered[1:])
		if err == nil && n >= 1 && n <= 24 {
			return fmt.Sprintf("F%d", n), nil
		}
	}

	return value, nil
}

func allowsModifierlessAcceleratorKey(key string) bool {
	if len(key) < 2 || key[0] != 'F' {
		return false
	}
	n, err := strconv.Atoi(key[1:])
	if err != nil {
		return false
	}
	return n >= 1 && n <= 12
}

func uniqueModifiers(modifiers []string) []string {
	seen := make(map[string]bool, len(modifiers))
	result := make([]string, 0, len(modifiers))
	for _, modifier := range modifiers {
		if seen[modifier] {
			continue
		}
		seen[modifier] = true
		result = append(result, modifier)
	}
	return result
}

func IsSupportedFcitxTriggerMode(value string) bool {
	switch NormalizeFcitxTriggerMode(value) {
	case FcitxTriggerModeToggle, FcitxTriggerModeHold:
		return true
	default:
		return false
	}
}

func InitDefault(path string, overwrite bool) (InitResult, error) {
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return ensureStarterDictionary(path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return InitResult{}, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return InitResult{}, err
	}

	cfg := Default()
	cfg.Dictionary.File = defaultDictionaryRelativePath

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return InitResult{}, err
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return InitResult{}, err
	}
	dictionaryPath := dictionaryPathForConfig(path)
	dictionaryWritten, err := writeDefaultDictionary(dictionaryPath, overwrite)
	if err != nil {
		return InitResult{}, err
	}

	return InitResult{
		ConfigWritten:     true,
		DictionaryWritten: dictionaryWritten,
		DictionaryPath:    dictionaryPath,
	}, nil
}

func WriteDefault(path string, overwrite bool) (bool, error) {
	result, err := InitDefault(path, overwrite)
	return result.ConfigWritten, err
}

func dictionaryPathForConfig(path string) string {
	return filepath.Join(filepath.Dir(path), "dictionary.yaml")
}

func writeDefaultDictionary(path string, overwrite bool) (bool, error) {
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return false, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}

	if err := os.WriteFile(path, []byte(defaultDictionaryYAML()), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func defaultDictionaryYAML() string {
	return strings.Join([]string{
		"entries:",
		"  - canonical: \"Coe\"",
		"    aliases: [\"扣诶\"]",
		"",
		"  - canonical: \"systemctl\"",
		"    aliases: [\"system control\", \"system c t l\"]",
		"    scenes: [\"terminal\"]",
		"",
	}, "\n")
}

func ensureStarterDictionary(path string) (InitResult, error) {
	cfg, err := Load(path)
	if err != nil {
		return InitResult{}, err
	}

	configUpdated := false
	dictionaryPath := strings.TrimSpace(cfg.Dictionary.File)
	if dictionaryPath == "" {
		cfg.Dictionary.File = defaultDictionaryRelativePath
		if err := Save(path, cfg); err != nil {
			return InitResult{}, err
		}
		configUpdated = true
		dictionaryPath = dictionaryPathForConfig(path)
	}

	dictionaryWritten, err := writeDefaultDictionary(dictionaryPath, false)
	if err != nil {
		return InitResult{}, err
	}

	return InitResult{
		ConfigUpdated:     configUpdated,
		DictionaryWritten: dictionaryWritten,
		DictionaryPath:    dictionaryPath,
	}, nil
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func SetValue(cfg *Config, key, value string) error {
	switch strings.TrimSpace(key) {
	case "runtime.mode":
		normalized := NormalizeRuntimeMode(value)
		if !IsSupportedRuntimeMode(normalized) {
			return errors.New("unsupported runtime.mode: " + value)
		}
		cfg.Runtime.Mode = normalized
		return nil
	case "hotkey.trigger_mode":
		normalized := NormalizeFcitxTriggerMode(value)
		if !IsSupportedFcitxTriggerMode(normalized) {
			return errors.New("unsupported hotkey.trigger_mode: " + value)
		}
		cfg.Hotkey.TriggerMode = normalized
		return nil
	case "hotkey.preferred_accelerator":
		normalized, err := NormalizePreferredAccelerator(value)
		if err != nil {
			return err
		}
		cfg.Hotkey.PreferredAccelerator = normalized
		return nil
	default:
		return errors.New("unsupported config key: " + key)
	}
}

func loadEnvFile(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid env line %d in %s", i+1, path)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("invalid env line %d in %s: empty key", i+1, path)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = strings.Trim(value, "\"")
			} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
				value = strings.Trim(value, "'")
			}
		}

		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return nil
}

func resolveConfigRelativePath(configPath, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || filepath.IsAbs(trimmed) {
		return trimmed
	}
	return filepath.Join(filepath.Dir(configPath), trimmed)
}
