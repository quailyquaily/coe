package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDefaultAndLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	written, err := WriteDefault(path, false)
	if err != nil {
		t.Fatalf("WriteDefault() error = %v", err)
	}
	if !written {
		t.Fatal("WriteDefault() reported not written")
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Runtime.TargetDesktop != "gnome" {
		t.Fatalf("unexpected target desktop %q", cfg.Runtime.TargetDesktop)
	}
	if cfg.Runtime.Mode != RuntimeModeFcitx {
		t.Fatalf("unexpected runtime mode %q", cfg.Runtime.Mode)
	}
	if cfg.Audio.RecorderBinary != "pw-record" {
		t.Fatalf("unexpected recorder %q", cfg.Audio.RecorderBinary)
	}
	if cfg.ASR.Binary != "" {
		t.Fatalf("unexpected ASR binary %q", cfg.ASR.Binary)
	}
	if cfg.ASR.TimeoutSeconds != 60 {
		t.Fatalf("unexpected ASR timeout %d", cfg.ASR.TimeoutSeconds)
	}
	if cfg.LLM.TimeoutSeconds != 45 {
		t.Fatalf("unexpected LLM timeout %d", cfg.LLM.TimeoutSeconds)
	}
	if !cfg.Notifications.EnableSystem {
		t.Fatal("expected system notifications to be enabled by default")
	}
	if cfg.Notifications.NotifyOnComplete {
		t.Fatal("expected completion notifications to be disabled by default")
	}
	if cfg.Runtime.LogLevel != "info" {
		t.Fatalf("unexpected log level %q", cfg.Runtime.LogLevel)
	}
	if cfg.Output.PasteShortcut != "ctrl+v" {
		t.Fatalf("unexpected paste shortcut %q", cfg.Output.PasteShortcut)
	}
	if cfg.Output.TerminalPasteShortcut != "ctrl+shift+v" {
		t.Fatalf("unexpected terminal paste shortcut %q", cfg.Output.TerminalPasteShortcut)
	}
	if cfg.Hotkey.TriggerMode != FcitxTriggerModeToggle {
		t.Fatalf("unexpected hotkey trigger mode %q", cfg.Hotkey.TriggerMode)
	}
	if !cfg.Output.UseGNOMEFocusHelper {
		t.Fatal("expected GNOME focus helper to be enabled by default")
	}
	if cfg.Dictionary.File != filepath.Join(dir, "dictionary.yaml") {
		t.Fatalf("unexpected dictionary path %q", cfg.Dictionary.File)
	}

	configData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	configText := string(configData)
	for _, fragment := range []string{
		"# Runtime behavior.",
		"# Automatic speech recognition provider.",
		"# Personal dictionary used during LLM correction and post-correction normalization.",
		"binary: \"\"",
		"timeout_seconds: 60",
		"timeout_seconds: 45",
		"file: \"./dictionary.yaml\"",
	} {
		if !strings.Contains(configText, fragment) {
			t.Fatalf("config template missing %q in:\n%s", fragment, configText)
		}
	}

	dictionaryData, err := os.ReadFile(filepath.Join(dir, "dictionary.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(dictionary) error = %v", err)
	}
	dictionaryText := string(dictionaryData)
	for _, fragment := range []string{
		"canonical: \"Coe\"",
		"aliases: [\"扣诶\"]",
		"canonical: \"systemctl\"",
		"aliases: [\"system control\", \"system c t l\"]",
		"scenes: [\"terminal\"]",
	} {
		if !strings.Contains(dictionaryText, fragment) {
			t.Fatalf("dictionary example missing %q in:\n%s", fragment, dictionaryText)
		}
	}
}

func TestInitDefaultBackfillsStarterDictionaryForExistingConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	source := "# keep runtime comment\nruntime:\n  # keep mode comment\n  mode: desktop\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	result, err := InitDefault(path, false)
	if err != nil {
		t.Fatalf("InitDefault() error = %v", err)
	}
	if result.ConfigWritten {
		t.Fatal("expected existing config to remain in place")
	}
	if !result.ConfigUpdated {
		t.Fatal("expected config to be updated with default dictionary path")
	}
	if !result.DictionaryWritten {
		t.Fatal("expected starter dictionary to be written")
	}
	if result.DictionaryPath != filepath.Join(dir, "dictionary.yaml") {
		t.Fatalf("DictionaryPath = %q", result.DictionaryPath)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Dictionary.File != filepath.Join(dir, "dictionary.yaml") {
		t.Fatalf("Dictionary.File = %q", cfg.Dictionary.File)
	}
	updatedData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	updatedText := string(updatedData)
	for _, fragment := range []string{
		"# keep runtime comment",
		"# keep mode comment",
		"dictionary:",
		"file: ./dictionary.yaml",
	} {
		if !strings.Contains(updatedText, fragment) {
			t.Fatalf("updated config missing %q in:\n%s", fragment, updatedText)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "dictionary.yaml")); err != nil {
		t.Fatalf("starter dictionary missing: %v", err)
	}
}

func TestInitDefaultCreatesMissingCustomDictionaryWithoutChangingConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	customDictionary := filepath.Join(dir, "custom-dictionary.yaml")
	data := []byte("dictionary:\n  file: ./custom-dictionary.yaml\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	result, err := InitDefault(path, false)
	if err != nil {
		t.Fatalf("InitDefault() error = %v", err)
	}
	if result.ConfigWritten || result.ConfigUpdated {
		t.Fatalf("unexpected config mutation: %+v", result)
	}
	if !result.DictionaryWritten {
		t.Fatal("expected custom starter dictionary to be written")
	}
	if result.DictionaryPath != customDictionary {
		t.Fatalf("DictionaryPath = %q, want %q", result.DictionaryPath, customDictionary)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Dictionary.File != customDictionary {
		t.Fatalf("Dictionary.File = %q, want %q", cfg.Dictionary.File, customDictionary)
	}
}

func TestLoadRejectsUnsupportedRuntimeMode(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("runtime:\n  mode: strange\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected unsupported runtime mode to fail")
	}
}

func TestLoadRejectsUnsupportedFcitxTriggerMode(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("hotkey:\n  trigger_mode: press-and-pray\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected unsupported fcitx trigger mode to fail")
	}
}

func TestLoadResolvesPromptFilesRelativeToConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte("asr:\n  prompt_file: prompts/asr.tmpl\nllm:\n  prompt_file: prompts/llm.tmpl\ndictionary:\n  file: prompts/dictionary.yaml\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ASR.PromptFile != filepath.Join(dir, "prompts", "asr.tmpl") {
		t.Fatalf("ASR.PromptFile = %q", cfg.ASR.PromptFile)
	}
	if cfg.LLM.PromptFile != filepath.Join(dir, "prompts", "llm.tmpl") {
		t.Fatalf("LLM.PromptFile = %q", cfg.LLM.PromptFile)
	}
	if cfg.Dictionary.File != filepath.Join(dir, "prompts", "dictionary.yaml") {
		t.Fatalf("Dictionary.File = %q", cfg.Dictionary.File)
	}
}

func TestSetValueRuntimeMode(t *testing.T) {
	t.Parallel()

	cfg := Default()
	if err := SetValue(&cfg, "runtime.mode", "fcitx"); err != nil {
		t.Fatalf("SetValue() error = %v", err)
	}
	if cfg.Runtime.Mode != RuntimeModeFcitx {
		t.Fatalf("runtime.mode = %q, want %q", cfg.Runtime.Mode, RuntimeModeFcitx)
	}
}

func TestSetValueFcitxTriggerMode(t *testing.T) {
	t.Parallel()

	cfg := Default()
	if err := SetValue(&cfg, "hotkey.trigger_mode", "hold"); err != nil {
		t.Fatalf("SetValue() error = %v", err)
	}
	if cfg.Hotkey.TriggerMode != FcitxTriggerModeHold {
		t.Fatalf("hotkey.trigger_mode = %q, want %q", cfg.Hotkey.TriggerMode, FcitxTriggerModeHold)
	}
}

func TestNormalizePreferredAccelerator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "normalizes bracket format",
			input: "<super><shift>D",
			want:  "<Shift><Super>d",
		},
		{
			name:  "normalizes plus format",
			input: "ctrl+alt+space",
			want:  "<Control><Alt>space",
		},
		{
			name:  "allows bare function key",
			input: "F11",
			want:  "F11",
		},
		{
			name:  "allows shift function key without primary modifier",
			input: "<shift>F8",
			want:  "<Shift>F8",
		},
		{
			name:    "rejects missing primary modifier",
			input:   "<Shift>k",
			wantErr: true,
		},
		{
			name:    "rejects bare non function key",
			input:   "k",
			wantErr: true,
		},
		{
			name:    "rejects unsupported bare function key",
			input:   "F13",
			wantErr: true,
		},
		{
			name:    "rejects terminal ctrl c",
			input:   "ctrl+c",
			wantErr: true,
		},
		{
			name:    "rejects escape",
			input:   "alt+esc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizePreferredAccelerator(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("NormalizePreferredAccelerator() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizePreferredAccelerator() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizePreferredAccelerator() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetValuePreferredAccelerator(t *testing.T) {
	t.Parallel()

	cfg := Default()
	if err := SetValue(&cfg, "hotkey.preferred_accelerator", "super+shift+d"); err != nil {
		t.Fatalf("SetValue() error = %v", err)
	}
	if cfg.Hotkey.PreferredAccelerator != "<Shift><Super>d" {
		t.Fatalf("hotkey.preferred_accelerator = %q, want %q", cfg.Hotkey.PreferredAccelerator, "<Shift><Super>d")
	}
}

func TestSetValuePreferredAcceleratorAllowsBareFunctionKey(t *testing.T) {
	t.Parallel()

	cfg := Default()
	if err := SetValue(&cfg, "hotkey.preferred_accelerator", "F11"); err != nil {
		t.Fatalf("SetValue() error = %v", err)
	}
	if cfg.Hotkey.PreferredAccelerator != "F11" {
		t.Fatalf("hotkey.preferred_accelerator = %q, want %q", cfg.Hotkey.PreferredAccelerator, "F11")
	}
}

func TestSetValueRejectsUnsupportedKey(t *testing.T) {
	t.Parallel()

	cfg := Default()
	if err := SetValue(&cfg, "llm.model", "x"); err == nil {
		t.Fatal("expected unsupported config key to fail")
	}
}

func TestUpdateValuePreservesComments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	source := strings.Join([]string{
		"# top comment",
		"runtime:",
		"  # runtime mode comment",
		"  mode: desktop",
		"",
		"hotkey:",
		"  # trigger comment",
		"  trigger_mode: toggle",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	normalized, err := UpdateValue(path, "runtime.mode", "fcitx")
	if err != nil {
		t.Fatalf("UpdateValue() error = %v", err)
	}
	if normalized != "fcitx" {
		t.Fatalf("normalized = %q, want %q", normalized, "fcitx")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	for _, fragment := range []string{
		"# top comment",
		"# runtime mode comment",
		"# trigger comment",
		"mode: fcitx",
		"trigger_mode: toggle",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("updated config missing %q in:\n%s", fragment, text)
		}
	}
}

func TestUpdateValueCreatesCommentedDefaultConfigWhenMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	normalized, err := UpdateValue(path, "hotkey.trigger_mode", "hold")
	if err != nil {
		t.Fatalf("UpdateValue() error = %v", err)
	}
	if normalized != "hold" {
		t.Fatalf("normalized = %q, want %q", normalized, "hold")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	for _, fragment := range []string{
		"# Runtime behavior.",
		"# Trigger semantics used by the Fcitx5 module when runtime.mode is `fcitx`.",
		"trigger_mode: hold",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("config missing %q in:\n%s", fragment, text)
		}
	}
}

func TestLoadEnvFileLoadsMissingKeysFromEnvFile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("SENSEVOICE_TOKEN", "")
	if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
		t.Fatalf("Unsetenv(OPENAI_API_KEY) error = %v", err)
	}
	if err := os.Unsetenv("SENSEVOICE_TOKEN"); err != nil {
		t.Fatalf("Unsetenv(SENSEVOICE_TOKEN) error = %v", err)
	}

	envDir := filepath.Join(tempDir, "coe")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	data := []byte("# comment\nOPENAI_API_KEY=file-key\nexport SENSEVOICE_TOKEN=\"quoted-token\"\n")
	if err := os.WriteFile(filepath.Join(envDir, "env"), data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := LoadEnvFile(); err != nil {
		t.Fatalf("LoadEnvFile() error = %v", err)
	}
	if got := os.Getenv("OPENAI_API_KEY"); got != "file-key" {
		t.Fatalf("OPENAI_API_KEY = %q, want %q", got, "file-key")
	}
	if got := os.Getenv("SENSEVOICE_TOKEN"); got != "quoted-token" {
		t.Fatalf("SENSEVOICE_TOKEN = %q, want %q", got, "quoted-token")
	}
}

func TestLoadEnvFileDoesNotOverrideExistingEnv(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("OPENAI_API_KEY", "shell-key")

	envDir := filepath.Join(tempDir, "coe")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	data := []byte("OPENAI_API_KEY=file-key\n")
	if err := os.WriteFile(filepath.Join(envDir, "env"), data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := LoadEnvFile(); err != nil {
		t.Fatalf("LoadEnvFile() error = %v", err)
	}
	if got := os.Getenv("OPENAI_API_KEY"); got != "shell-key" {
		t.Fatalf("OPENAI_API_KEY = %q, want %q", got, "shell-key")
	}
}
