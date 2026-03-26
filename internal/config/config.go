package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const envConfigPath = "COE_CONFIG"

type Config struct {
	Runtime RuntimeConfig `yaml:"runtime"`
	Hotkey  HotkeyConfig  `yaml:"hotkey"`
	Audio   AudioConfig   `yaml:"audio"`
	ASR     Provider      `yaml:"asr"`
	LLM     Provider      `yaml:"llm"`
	Output  OutputConfig  `yaml:"output"`
}

type RuntimeConfig struct {
	TargetDesktop        string `yaml:"target_desktop"`
	AllowExternalTrigger bool   `yaml:"allow_external_trigger"`
}

type HotkeyConfig struct {
	Name                 string `yaml:"name"`
	Description          string `yaml:"description"`
	PreferredAccelerator string `yaml:"preferred_accelerator"`
}

type AudioConfig struct {
	RecorderBinary string `yaml:"recorder_binary"`
	SampleRate     int    `yaml:"sample_rate"`
	Channels       int    `yaml:"channels"`
	Format         string `yaml:"format"`
}

type Provider struct {
	Kind      string `yaml:"kind"`
	Endpoint  string `yaml:"endpoint"`
	Model     string `yaml:"model"`
	APIKeyEnv string `yaml:"api_key_env"`
	Language  string `yaml:"language"`
	Prompt    string `yaml:"prompt"`
}

type OutputConfig struct {
	PreferredClipboardMode string `yaml:"preferred_clipboard_mode"`
	EnableAutoPaste        bool   `yaml:"enable_auto_paste"`
	PersistPortalAccess    bool   `yaml:"persist_portal_access"`
	ClipboardBinary        string `yaml:"clipboard_binary"`
	PasteBinary            string `yaml:"paste_binary"`
}

func Default() Config {
	return Config{
		Runtime: RuntimeConfig{
			TargetDesktop:        "gnome",
			AllowExternalTrigger: true,
		},
		Hotkey: HotkeyConfig{
			Name:                 "push-to-talk",
			Description:          "Press and hold to start dictation.",
			PreferredAccelerator: "<Ctrl><Alt>space",
		},
		Audio: AudioConfig{
			RecorderBinary: "pw-record",
			SampleRate:     16000,
			Channels:       1,
			Format:         "s16",
		},
		ASR: Provider{
			Kind:      "openai",
			Endpoint:  "https://api.openai.com/v1/audio/transcriptions",
			Model:     "gpt-4o-mini-transcribe",
			APIKeyEnv: "OPENAI_API_KEY",
			Language:  "zh",
			Prompt:    "",
		},
		LLM: Provider{
			Kind:      "openai",
			Endpoint:  "https://api.openai.com/v1/responses",
			Model:     "gpt-4o-mini",
			APIKeyEnv: "OPENAI_API_KEY",
			Prompt:    "",
		},
		Output: OutputConfig{
			PreferredClipboardMode: "portal",
			EnableAutoPaste:        true,
			PersistPortalAccess:    true,
			ClipboardBinary:        "wl-copy",
			PasteBinary:            "",
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

	return cfg, nil
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
