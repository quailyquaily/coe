package output

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			if !req.Clipboard || req.Paste {
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
			if req.Clipboard || !req.Paste {
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

type fakePortalSession struct {
	clipboard    string
	clipboardErr error
	pasteCalls   int
	pasteErr     error
}

func (f *fakePortalSession) SetClipboard(_ context.Context, text string) error {
	if f.clipboardErr != nil {
		return f.clipboardErr
	}
	f.clipboard = text
	return nil
}

func (f *fakePortalSession) SendPaste(context.Context) error {
	f.pasteCalls++
	return f.pasteErr
}

func (f *fakePortalSession) Close() error {
	return nil
}
