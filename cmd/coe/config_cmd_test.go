package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"coe/internal/config"
)

func TestRunConfigSetPreservesComments(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, err := config.ResolvePath()
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}
	if _, err := config.WriteDefault(path, false); err != nil {
		t.Fatalf("WriteDefault() error = %v", err)
	}

	if err := runConfig(context.Background(), []string{"set", "runtime.mode", "desktop"}); err != nil {
		t.Fatalf("runConfig() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	for _, fragment := range []string{
		"# Runtime behavior.",
		"# Runtime mode.",
		"mode: desktop",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("updated config missing %q in:\n%s", fragment, text)
		}
	}
}

func TestRunConfigSetCreatesCommentedConfigWhenMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, err := config.ResolvePath()
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected config to be missing, stat err=%v", err)
	}

	if err := runConfig(context.Background(), []string{"set", "hotkey.trigger_mode", "hold"}); err != nil {
		t.Fatalf("runConfig() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Clean(path))
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
