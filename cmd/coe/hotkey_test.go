package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"coe/internal/config"
	"coe/internal/i18n"
)

func TestRunHotkeyPickWritesConfigAndRestarts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, err := config.ResolvePath()
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}
	if _, err := config.WriteDefault(path, false); err != nil {
		t.Fatalf("WriteDefault() error = %v", err)
	}

	originalRestart := runHotkeyRestart
	originalPick := pickHotkeyAccelerator
	originalOutput := hotkeyOutput
	defer func() {
		runHotkeyRestart = originalRestart
		pickHotkeyAccelerator = originalPick
		hotkeyOutput = originalOutput
	}()

	restarted := false
	runHotkeyRestart = func(context.Context) error {
		restarted = true
		return nil
	}
	pickHotkeyAccelerator = func(context.Context) (string, error) {
		return "ctrl+alt+space", nil
	}
	var output bytes.Buffer
	hotkeyOutput = &output

	if err := runHotkey(context.Background(), []string{"pick"}); err != nil {
		t.Fatalf("runHotkey() error = %v", err)
	}
	if !restarted {
		t.Fatal("expected restart to be triggered")
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Hotkey.PreferredAccelerator != "<Control><Alt>space" {
		t.Fatalf("PreferredAccelerator = %q, want %q", cfg.Hotkey.PreferredAccelerator, "<Control><Alt>space")
	}
	if !strings.Contains(output.String(), "hotkey.preferred_accelerator=<Control><Alt>space") {
		t.Fatalf("output = %q, want updated accelerator", output.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	for _, fragment := range []string{
		"# Hotkey metadata. This describes the intended shortcut, even when GNOME",
		"# Preferred accelerator. On GNOME fallback, Coe will try to register this",
		"preferred_accelerator: <Control><Alt>space",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("updated config missing %q in:\n%s", fragment, text)
		}
	}
}

func TestRunHotkeyPickRejectsForbiddenAccelerator(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	originalRestart := runHotkeyRestart
	originalPick := pickHotkeyAccelerator
	originalOutput := hotkeyOutput
	defer func() {
		runHotkeyRestart = originalRestart
		pickHotkeyAccelerator = originalPick
		hotkeyOutput = originalOutput
	}()

	restarted := false
	runHotkeyRestart = func(context.Context) error {
		restarted = true
		return nil
	}
	pickHotkeyAccelerator = func(context.Context) (string, error) {
		return "ctrl+c", nil
	}
	hotkeyOutput = &bytes.Buffer{}

	err := runHotkey(context.Background(), []string{"pick"})
	if err == nil {
		t.Fatal("runHotkey() error = nil, want error")
	}
	if restarted {
		t.Fatal("did not expect restart after validation error")
	}
}

func TestRunHotkeyPickSurfacesRestartError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, err := config.ResolvePath()
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}
	if _, err := config.WriteDefault(path, false); err != nil {
		t.Fatalf("WriteDefault() error = %v", err)
	}

	originalRestart := runHotkeyRestart
	originalPick := pickHotkeyAccelerator
	originalOutput := hotkeyOutput
	defer func() {
		runHotkeyRestart = originalRestart
		pickHotkeyAccelerator = originalPick
		hotkeyOutput = originalOutput
	}()

	runHotkeyRestart = func(context.Context) error {
		return errors.New("restart failed")
	}
	pickHotkeyAccelerator = func(context.Context) (string, error) {
		return "super+shift+d", nil
	}
	hotkeyOutput = &bytes.Buffer{}

	err = runHotkey(context.Background(), []string{"pick"})
	if err == nil || err.Error() != "restart failed" {
		t.Fatalf("runHotkey() error = %v, want restart failed", err)
	}
}

func TestRunHotkeyPickCanceled(t *testing.T) {
	originalRestart := runHotkeyRestart
	originalPick := pickHotkeyAccelerator
	originalOutput := hotkeyOutput
	defer func() {
		runHotkeyRestart = originalRestart
		pickHotkeyAccelerator = originalPick
		hotkeyOutput = originalOutput
	}()

	restarted := false
	runHotkeyRestart = func(context.Context) error {
		restarted = true
		return nil
	}
	pickHotkeyAccelerator = func(context.Context) (string, error) {
		return "", errHotkeyPickCanceled
	}
	hotkeyOutput = &bytes.Buffer{}

	err := runHotkey(context.Background(), []string{"pick"})
	if !errors.Is(err, errHotkeyPickCanceled) {
		t.Fatalf("runHotkey() error = %v, want errHotkeyPickCanceled", err)
	}
	if restarted {
		t.Fatal("did not expect restart after cancel")
	}
}

func TestHotkeyPickerTextsFor(t *testing.T) {
	t.Parallel()

	zh := hotkeyPickerTextsFor(i18n.NewForLocale("zh_CN.UTF-8"))
	if zh.Confirm != "确定" {
		t.Fatalf("zh Confirm = %q, want %q", zh.Confirm, "确定")
	}
	if zh.Heading != "请按下你的触发热键" {
		t.Fatalf("zh Heading = %q", zh.Heading)
	}

	ja := hotkeyPickerTextsFor(i18n.NewForLocale("ja_JP.UTF-8"))
	if ja.Cancel != "キャンセル" {
		t.Fatalf("ja Cancel = %q, want %q", ja.Cancel, "キャンセル")
	}
}
