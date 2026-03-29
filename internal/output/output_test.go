package output

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"coe/internal/focus"
)

func TestDeliverWritesClipboard(t *testing.T) {
	dir := t.TempDir()
	clipboardSink := filepath.Join(dir, "clipboard.txt")
	clipboardBin := filepath.Join(dir, "fake-wl-copy.sh")

	script := "#!/bin/sh\ncat > \"$COE_CLIPBOARD_SINK\"\n"
	if err := os.WriteFile(clipboardBin, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("COE_CLIPBOARD_SINK", clipboardSink)

	coord := &Coordinator{
		ClipboardPlan:   "command",
		PastePlan:       "unavailable",
		ClipboardBinary: clipboardBin,
	}

	delivery, err := coord.Deliver(context.Background(), "hello clipboard")
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if !delivery.ClipboardWritten {
		t.Fatal("expected clipboard to be written")
	}

	data, err := os.ReadFile(clipboardSink)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "hello clipboard" {
		t.Fatalf("clipboard contents = %q, want %q", got, "hello clipboard")
	}
}

func TestDeliverRunsYdotoolPaste(t *testing.T) {
	dir := t.TempDir()
	clipboardSink := filepath.Join(dir, "clipboard.txt")
	pasteSink := filepath.Join(dir, "paste.txt")
	clipboardBin := filepath.Join(dir, "fake-wl-copy.sh")
	pasteBin := filepath.Join(dir, "ydotool")

	if err := os.WriteFile(clipboardBin, []byte("#!/bin/sh\ncat > \"$COE_CLIPBOARD_SINK\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(clipboard) error = %v", err)
	}
	if err := os.WriteFile(pasteBin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$COE_PASTE_SINK\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(paste) error = %v", err)
	}

	t.Setenv("COE_CLIPBOARD_SINK", clipboardSink)
	t.Setenv("COE_PASTE_SINK", pasteSink)

	coord := &Coordinator{
		ClipboardPlan:   "command",
		PastePlan:       "command",
		ClipboardBinary: clipboardBin,
		PasteBinary:     pasteBin,
		EnableAutoPaste: true,
	}

	delivery, err := coord.Deliver(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if !delivery.PasteExecuted {
		t.Fatal("expected paste to be executed")
	}
	if delivery.PasteShortcut != "ctrl+v" {
		t.Fatalf("paste shortcut = %q, want %q", delivery.PasteShortcut, "ctrl+v")
	}

	data, err := os.ReadFile(pasteSink)
	if err != nil {
		t.Fatalf("ReadFile(paste) error = %v", err)
	}
	got := strings.TrimSpace(string(data))
	want := "key\n29:1\n47:1\n47:0\n29:0"
	if got != want {
		t.Fatalf("paste command args = %q, want %q", got, want)
	}
}

func TestDeliverPrefersPortalClipboard(t *testing.T) {
	portal := &fakePortalSession{}
	coord := &Coordinator{
		ClipboardPlan:      "portal",
		PastePlan:          "unavailable",
		UsePortalClipboard: true,
		PortalFactory: func(_ context.Context, req PortalRequest) (PortalSession, error) {
			if !req.Clipboard || req.Paste || req.Persist || req.RestoreToken != "" {
				t.Fatalf("unexpected portal request: %+v", req)
			}
			return portal, nil
		},
	}

	delivery, err := coord.Deliver(context.Background(), "hello portal")
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if !delivery.ClipboardWritten || delivery.ClipboardMethod != "portal" {
		t.Fatalf("unexpected delivery result: %+v", delivery)
	}
	if portal.clipboard != "hello portal" {
		t.Fatalf("portal clipboard = %q, want %q", portal.clipboard, "hello portal")
	}
}

func TestDeliverFallsBackToCommandWhenPortalClipboardFails(t *testing.T) {
	dir := t.TempDir()
	clipboardSink := filepath.Join(dir, "clipboard.txt")
	clipboardBin := filepath.Join(dir, "fake-wl-copy.sh")

	if err := os.WriteFile(clipboardBin, []byte("#!/bin/sh\ncat > \"$COE_CLIPBOARD_SINK\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("COE_CLIPBOARD_SINK", clipboardSink)

	coord := &Coordinator{
		ClipboardPlan:      "portal",
		PastePlan:          "unavailable",
		ClipboardBinary:    clipboardBin,
		UsePortalClipboard: true,
		PortalFactory: func(_ context.Context, _ PortalRequest) (PortalSession, error) {
			return &fakePortalSession{clipboardErr: fmt.Errorf("portal unavailable")}, nil
		},
	}

	delivery, err := coord.Deliver(context.Background(), "fallback text")
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if delivery.ClipboardMethod != filepath.Base(clipboardBin) {
		t.Fatalf("clipboard method = %q, want %q", delivery.ClipboardMethod, filepath.Base(clipboardBin))
	}
}

func TestDeliverPrefersPortalPaste(t *testing.T) {
	dir := t.TempDir()
	clipboardSink := filepath.Join(dir, "clipboard.txt")
	clipboardBin := filepath.Join(dir, "fake-wl-copy.sh")

	if err := os.WriteFile(clipboardBin, []byte("#!/bin/sh\ncat > \"$COE_CLIPBOARD_SINK\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("COE_CLIPBOARD_SINK", clipboardSink)

	portal := &fakePortalSession{}
	coord := &Coordinator{
		ClipboardPlan:   "command",
		PastePlan:       "portal",
		ClipboardBinary: clipboardBin,
		EnableAutoPaste: true,
		UsePortalPaste:  true,
		PortalFactory: func(_ context.Context, req PortalRequest) (PortalSession, error) {
			if req.Clipboard || !req.Paste || req.Persist || req.RestoreToken != "" {
				t.Fatalf("unexpected portal request: %+v", req)
			}
			return portal, nil
		},
	}

	delivery, err := coord.Deliver(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if !delivery.PasteExecuted || delivery.PasteMethod != "portal" {
		t.Fatalf("unexpected delivery result: %+v", delivery)
	}
	if portal.pasteCalls != 1 {
		t.Fatalf("portal paste calls = %d, want 1", portal.pasteCalls)
	}
	if portal.lastShortcut != "ctrl+v" {
		t.Fatalf("portal last shortcut = %q, want %q", portal.lastShortcut, "ctrl+v")
	}
}

func TestDeliverKeepsClipboardSuccessWhenPortalPasteFails(t *testing.T) {
	dir := t.TempDir()
	clipboardSink := filepath.Join(dir, "clipboard.txt")
	clipboardBin := filepath.Join(dir, "fake-wl-copy.sh")

	if err := os.WriteFile(clipboardBin, []byte("#!/bin/sh\ncat > \"$COE_CLIPBOARD_SINK\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("COE_CLIPBOARD_SINK", clipboardSink)

	coord := &Coordinator{
		ClipboardPlan:   "command",
		PastePlan:       "portal",
		ClipboardBinary: clipboardBin,
		EnableAutoPaste: true,
		UsePortalPaste:  true,
		PortalFactory: func(_ context.Context, _ PortalRequest) (PortalSession, error) {
			return &fakePortalSession{pasteErr: fmt.Errorf("permission denied")}, nil
		},
	}

	delivery, err := coord.Deliver(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if !delivery.ClipboardWritten {
		t.Fatal("expected clipboard success to be preserved")
	}
	if delivery.PasteExecuted {
		t.Fatal("expected paste to remain unexecuted")
	}
	if delivery.PasteWarning == "" {
		t.Fatal("expected paste warning")
	}
}

func TestDeliverRetriesInvalidPortalSession(t *testing.T) {
	t.Parallel()

	first := &fakePortalSession{clipboardErr: fmt.Errorf("Invalid session")}
	second := &fakePortalSession{}
	factoryCalls := 0

	coord := &Coordinator{
		ClipboardPlan:      "portal",
		PastePlan:          "unavailable",
		UsePortalClipboard: true,
		PortalFactory: func(_ context.Context, _ PortalRequest) (PortalSession, error) {
			factoryCalls++
			if factoryCalls == 1 {
				return first, nil
			}
			return second, nil
		},
	}

	delivery, err := coord.Deliver(context.Background(), "retry me")
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if !delivery.ClipboardWritten || delivery.ClipboardMethod != "portal" {
		t.Fatalf("unexpected delivery result: %+v", delivery)
	}
	if factoryCalls != 2 {
		t.Fatalf("factory calls = %d, want 2", factoryCalls)
	}
	if second.clipboard != "retry me" {
		t.Fatalf("portal clipboard = %q, want %q", second.clipboard, "retry me")
	}
}

func TestDeliverUsesTerminalPasteShortcutForFocusedTerminal(t *testing.T) {
	dir := t.TempDir()
	clipboardSink := filepath.Join(dir, "clipboard.txt")
	pasteSink := filepath.Join(dir, "paste.txt")
	clipboardBin := filepath.Join(dir, "fake-wl-copy.sh")
	pasteBin := filepath.Join(dir, "ydotool")

	if err := os.WriteFile(clipboardBin, []byte("#!/bin/sh\ncat > \"$COE_CLIPBOARD_SINK\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(clipboard) error = %v", err)
	}
	if err := os.WriteFile(pasteBin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$COE_PASTE_SINK\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(paste) error = %v", err)
	}

	t.Setenv("COE_CLIPBOARD_SINK", clipboardSink)
	t.Setenv("COE_PASTE_SINK", pasteSink)

	coord := &Coordinator{
		ClipboardPlan:         "command",
		PastePlan:             "command",
		ClipboardBinary:       clipboardBin,
		PasteBinary:           pasteBin,
		EnableAutoPaste:       true,
		PasteShortcut:         "ctrl+v",
		TerminalPasteShortcut: "ctrl+shift+v",
		FocusProvider: fixedFocusProvider{
			target: focus.Target{AppID: "org.gnome.Console"},
		},
	}

	delivery, err := coord.Deliver(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if delivery.PasteShortcut != "ctrl+shift+v" {
		t.Fatalf("paste shortcut = %q, want %q", delivery.PasteShortcut, "ctrl+shift+v")
	}
	if delivery.PasteTarget != "org.gnome.Console" {
		t.Fatalf("paste target = %q, want %q", delivery.PasteTarget, "org.gnome.Console")
	}

	data, err := os.ReadFile(pasteSink)
	if err != nil {
		t.Fatalf("ReadFile(paste) error = %v", err)
	}
	got := strings.TrimSpace(string(data))
	want := "key\n29:1\n42:1\n47:1\n47:0\n42:0\n29:0"
	if got != want {
		t.Fatalf("paste command args = %q, want %q", got, want)
	}
}

func TestDeliverWithTargetUsesProvidedFocusSample(t *testing.T) {
	dir := t.TempDir()
	clipboardSink := filepath.Join(dir, "clipboard.txt")
	pasteSink := filepath.Join(dir, "paste.txt")
	clipboardBin := filepath.Join(dir, "fake-wl-copy.sh")
	pasteBin := filepath.Join(dir, "ydotool")

	if err := os.WriteFile(clipboardBin, []byte("#!/bin/sh\ncat > \"$COE_CLIPBOARD_SINK\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(clipboard) error = %v", err)
	}
	if err := os.WriteFile(pasteBin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$COE_PASTE_SINK\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(paste) error = %v", err)
	}

	t.Setenv("COE_CLIPBOARD_SINK", clipboardSink)
	t.Setenv("COE_PASTE_SINK", pasteSink)

	coord := &Coordinator{
		ClipboardPlan:         "command",
		PastePlan:             "command",
		ClipboardBinary:       clipboardBin,
		PasteBinary:           pasteBin,
		EnableAutoPaste:       true,
		PasteShortcut:         "ctrl+v",
		TerminalPasteShortcut: "ctrl+shift+v",
	}

	target := &focus.Target{WMClass: "gnome-terminal-server"}
	delivery, err := coord.DeliverWithTarget(context.Background(), "hello", target)
	if err != nil {
		t.Fatalf("DeliverWithTarget() error = %v", err)
	}
	if delivery.PasteShortcut != "ctrl+shift+v" {
		t.Fatalf("paste shortcut = %q, want %q", delivery.PasteShortcut, "ctrl+shift+v")
	}
	if delivery.PasteTarget != "gnome-terminal-server" {
		t.Fatalf("paste target = %q, want %q", delivery.PasteTarget, "gnome-terminal-server")
	}
}

func TestLooksLikeTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target focus.Target
		want   bool
	}{
		{name: "gnome terminal", target: focus.Target{WMClass: "gnome-terminal-server"}, want: true},
		{name: "konsole", target: focus.Target{WMClass: "konsole"}, want: true},
		{name: "xfce4 terminal", target: focus.Target{WMClass: "xfce4-terminal"}, want: true},
		{name: "tilix", target: focus.Target{WMClass: "Tilix"}, want: true},
		{name: "warp", target: focus.Target{WMClass: "WarpTerminal"}, want: true},
		{name: "rio", target: focus.Target{WMClass: "rio"}, want: true},
		{name: "tabby", target: focus.Target{WMClass: "tabby"}, want: true},
		{name: "hyper", target: focus.Target{WMClass: "Hyper"}, want: true},
		{name: "ordinary editor", target: focus.Target{WMClass: "gedit"}, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := focus.LooksLikeTerminal(tt.target); got != tt.want {
				t.Fatalf("LooksLikeTerminal(%+v) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

func TestEnsurePortalLoadsAndSavesRestoreToken(t *testing.T) {
	store := NewPortalStateStore(filepath.Join(t.TempDir(), "state.json"))
	if err := store.Save(PortalAccess{RemoteDesktopRestoreToken: "old-token"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	coord := &Coordinator{
		ClipboardPlan:       "portal",
		PastePlan:           "portal",
		UsePortalClipboard:  true,
		UsePortalPaste:      true,
		EnableAutoPaste:     true,
		PersistPortalAccess: true,
		PortalStateStore:    store,
		PortalFactory: func(_ context.Context, req PortalRequest) (PortalSession, error) {
			if !req.Persist {
				t.Fatal("expected persist request")
			}
			if req.RestoreToken != "old-token" {
				t.Fatalf("restore token = %q, want %q", req.RestoreToken, "old-token")
			}
			return &fakePortalSession{restoreToken: "new-token"}, nil
		},
	}

	if _, err := coord.Deliver(context.Background(), "hello"); err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}

	saved, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if saved.RemoteDesktopRestoreToken != "new-token" {
		t.Fatalf("saved token = %q, want %q", saved.RemoteDesktopRestoreToken, "new-token")
	}
}

type fakePortalSession struct {
	clipboard    string
	clipboardErr error
	pasteCalls   int
	pasteErr     error
	lastShortcut string
	restoreToken string
}

type fixedFocusProvider struct {
	target focus.Target
	err    error
}

func (p fixedFocusProvider) Focused(context.Context) (focus.Target, error) {
	return p.target, p.err
}

func (fixedFocusProvider) Summary() string {
	return "fixed"
}

func (fixedFocusProvider) Close() error {
	return nil
}

func (f *fakePortalSession) SetClipboard(_ context.Context, text string) error {
	if f.clipboardErr != nil {
		return f.clipboardErr
	}
	f.clipboard = text
	return nil
}

func (f *fakePortalSession) SendPaste(_ context.Context, shortcut string) error {
	f.pasteCalls++
	f.lastShortcut = shortcut
	return f.pasteErr
}

func (f *fakePortalSession) Close() error {
	return nil
}

func (f *fakePortalSession) RestoreToken() string {
	return f.restoreToken
}
