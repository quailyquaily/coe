package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteDefaultAndLoad(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")

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
	if cfg.Runtime.Mode != RuntimeModeDesktop {
		t.Fatalf("unexpected runtime mode %q", cfg.Runtime.Mode)
	}
	if cfg.Audio.RecorderBinary != "pw-record" {
		t.Fatalf("unexpected recorder %q", cfg.Audio.RecorderBinary)
	}
	if !cfg.Notifications.EnableSystem {
		t.Fatal("expected system notifications to be enabled by default")
	}
	if cfg.Notifications.ShowTextPreview {
		t.Fatal("expected notification text preview to be disabled by default")
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
	if !cfg.Output.UseGNOMEFocusHelper {
		t.Fatal("expected GNOME focus helper to be enabled by default")
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

func TestSetValueRejectsUnsupportedKey(t *testing.T) {
	t.Parallel()

	cfg := Default()
	if err := SetValue(&cfg, "llm.model", "x"); err == nil {
		t.Fatal("expected unsupported config key to fail")
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
