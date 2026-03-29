package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const envConfigPath = "COE_CONFIG"

const (
	RuntimeModeDesktop = "desktop"
	RuntimeModeFcitx   = "fcitx"
)

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

func Default() Config {
	return Config{
		Runtime: RuntimeConfig{
			Mode:          RuntimeModeDesktop,
			TargetDesktop: "gnome",
			LogLevel:      "info",
		},
		Hotkey: HotkeyConfig{
			Name:                 "coe-trigger",
			PreferredAccelerator: "<Shift><Super>d",
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
	cfg.ASR.PromptFile = resolveConfigRelativePath(path, cfg.ASR.PromptFile)
	cfg.LLM.PromptFile = resolveConfigRelativePath(path, cfg.LLM.PromptFile)
	cfg.Dictionary.File = resolveConfigRelativePath(path, cfg.Dictionary.File)

	return cfg, nil
}

func NormalizeRuntimeMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return RuntimeModeDesktop
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

func WriteDefault(path string, overwrite bool) (bool, error) {
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

	data, err := yaml.Marshal(Default())
	if err != nil {
		return false, err
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return false, err
	}

	return true, nil
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
