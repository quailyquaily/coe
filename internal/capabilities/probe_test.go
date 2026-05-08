package capabilities

import (
	"context"
	"strings"
	"testing"

	"coe/internal/platform/portal"
)

func TestSelectProfileFull(t *testing.T) {
	t.Parallel()

	caps := Capabilities{
		SessionType: "wayland",
		Desktop:     "gnome",
		Portals: portal.Interfaces{
			GlobalShortcuts: portal.InterfaceStatus{Available: true, Version: 2},
			RemoteDesktop:   portal.InterfaceStatus{Available: true, Version: 2},
			Clipboard:       portal.InterfaceStatus{Available: true, Version: 1},
		},
		Binaries: map[string]Binary{
			"pw-record": {Found: true, Path: "/usr/bin/pw-record"},
		},
	}

	caps.Hotkey = planHotkey(caps)
	caps.Audio = planAudio(caps)
	caps.Clipboard = planClipboard(caps)
	caps.Paste = planPaste(caps)

	if got := selectProfile(caps); got != ProfileGNOMEFull {
		t.Fatalf("selectProfile() = %q, want %q", got, ProfileGNOMEFull)
	}
}

func TestReportIncludesFcitxStatus(t *testing.T) {
	t.Parallel()

	caps := Capabilities{
		Profile:     ProfileExternalTrigger,
		SessionType: "wayland",
		Desktop:     "gnome",
		DBusSession: true,
		Fcitx: FcitxStatus{
			Binary:          Binary{Name: "fcitx5", Path: "/usr/bin/fcitx5", Found: true},
			Running:         true,
			AddonConfigPath: "/usr/share/fcitx5/addon/coe.conf",
			ModulePath:      "/usr/lib/x86_64-linux-gnu/fcitx5/libcoefcitx.so",
			LogPath:         "/tmp/coe-fcitx-1000.log",
			LogPresent:      true,
			InitOK:          true,
		},
		Hotkey:    FeaturePlan{Mode: ModeExternalBinding, Detail: "GNOME custom shortcut -> `coe trigger toggle`"},
		Audio:     FeaturePlan{Mode: ModeCommand, Detail: "/usr/bin/pw-record"},
		Clipboard: FeaturePlan{Mode: ModePortal, Detail: "org.freedesktop.portal.Clipboard v1"},
		Paste:     FeaturePlan{Mode: ModePortal, Detail: "org.freedesktop.portal.RemoteDesktop v2"},
	}

	report := caps.Report()
	for _, want := range []string{
		"fcitx5: true (/usr/bin/fcitx5)",
		"fcitx running: true",
		"fcitx addon config: /usr/share/fcitx5/addon/coe.conf",
		"fcitx module library: /usr/lib/x86_64-linux-gnu/fcitx5/libcoefcitx.so",
		"fcitx init marker: true (/tmp/coe-fcitx-1000.log)",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("Report() missing %q in:\n%s", want, report)
		}
	}
}

func TestProbeWithOptionsSkipsPortals(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/tmp/coe-missing-bus")

	caps, err := ProbeWithOptions(context.Background(), ProbeOptions{SkipPortals: true})
	if err != nil {
		t.Fatalf("ProbeWithOptions() error = %v", err)
	}

	requireNoteContains(t, caps.Notes, "portal probe skipped: runtime mode does not need desktop portals")
}

func TestProbeSkipsPortalWhenDesktopEnvIsMissing(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/tmp/coe-missing-bus")
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")

	caps, err := Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}

	requireNoteContains(t, caps.Notes, "portal probe skipped: XDG_CURRENT_DESKTOP is missing")
}

func TestProbeSkipsPortalWhenDisplayEnvIsMissing(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/tmp/coe-missing-bus")
	t.Setenv("XDG_CURRENT_DESKTOP", "Hyprland")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")

	caps, err := Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}

	requireNoteContains(t, caps.Notes, "portal probe skipped: display environment is missing")
}

func requireNoteContains(t *testing.T, notes []string, want string) {
	t.Helper()
	for _, note := range notes {
		if strings.Contains(note, want) {
			return
		}
	}
	t.Fatalf("notes missing %q in %#v", want, notes)
}
