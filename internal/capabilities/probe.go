package capabilities

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"coe/internal/platform/portal"
)

type FeatureMode string

const (
	ModePortal          FeatureMode = "portal"
	ModeCommand         FeatureMode = "command"
	ModeExternalBinding FeatureMode = "external-binding"
	ModeUnavailable     FeatureMode = "unavailable"
)

type RuntimeProfile string

const (
	ProfileGNOMEFull       RuntimeProfile = "gnome-portal-full"
	ProfileGNOMEClipboard  RuntimeProfile = "gnome-clipboard-only"
	ProfileExternalTrigger RuntimeProfile = "gnome-external-trigger"
	ProfileUnsupported     RuntimeProfile = "unsupported"
)

type Binary struct {
	Name  string
	Path  string
	Found bool
}

type FeaturePlan struct {
	Mode   FeatureMode
	Detail string
}

type FcitxStatus struct {
	Binary          Binary
	Running         bool
	AddonConfigPath string
	ModulePath      string
	LogPath         string
	LogPresent      bool
	InitOK          bool
}

type Capabilities struct {
	SessionType string
	Desktop     string
	DBusSession bool
	Binaries    map[string]Binary
	Fcitx       FcitxStatus
	Portals     portal.Interfaces
	Hotkey      FeaturePlan
	Audio       FeaturePlan
	Clipboard   FeaturePlan
	Paste       FeaturePlan
	Profile     RuntimeProfile
	Notes       []string
}

func Probe(ctx context.Context) (Capabilities, error) {
	caps := Capabilities{
		SessionType: strings.ToLower(os.Getenv("XDG_SESSION_TYPE")),
		Desktop:     strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP")),
		DBusSession: os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "",
		Binaries:    detectBinaries("pw-record", "wl-copy", "wtype", "ydotool", "fcitx5"),
	}
	caps.Fcitx = detectFcitx(caps.Binaries["fcitx5"])

	if caps.DBusSession {
		client, err := portal.ConnectSession()
		if err != nil {
			caps.Notes = append(caps.Notes, fmt.Sprintf("connect to session bus failed: %v", err))
		} else {
			defer client.Close()

			interfaces, err := client.Probe(ctx)
			if err != nil {
				caps.Notes = append(caps.Notes, fmt.Sprintf("portal probe returned partial data: %v", err))
			}
			caps.Portals = interfaces
		}
	} else {
		caps.Notes = append(caps.Notes, "session D-Bus is not available")
	}

	caps.Hotkey = planHotkey(caps)
	caps.Audio = planAudio(caps)
	caps.Clipboard = planClipboard(caps)
	caps.Paste = planPaste(caps)
	caps.Profile = selectProfile(caps)
	caps.Notes = append(caps.Notes, generateNotes(caps)...)

	return caps, nil
}

func (c Capabilities) Report() string {
	var b strings.Builder

	fmt.Fprintf(&b, "runtime profile: %s\n", c.Profile)
	fmt.Fprintf(&b, "session type: %s\n", blankAsUnknown(c.SessionType))
	fmt.Fprintf(&b, "desktop: %s\n", blankAsUnknown(c.Desktop))
	fmt.Fprintf(&b, "dbus session: %t\n", c.DBusSession)
	fmt.Fprintf(&b, "fcitx5: %s\n", formatBinaryStatus(c.Fcitx.Binary))
	fmt.Fprintf(&b, "fcitx running: %t\n", c.Fcitx.Running)
	fmt.Fprintf(&b, "fcitx addon config: %s\n", blankAsMissing(c.Fcitx.AddonConfigPath))
	fmt.Fprintf(&b, "fcitx module library: %s\n", blankAsMissing(c.Fcitx.ModulePath))
	fmt.Fprintf(&b, "fcitx init marker: %s\n", formatFcitxInitMarker(c.Fcitx))
	fmt.Fprintf(&b, "portal global shortcuts: %s\n", formatPortalStatus(c.Portals.GlobalShortcuts))
	fmt.Fprintf(&b, "portal remote desktop: %s\n", formatPortalStatus(c.Portals.RemoteDesktop))
	fmt.Fprintf(&b, "portal clipboard: %s\n", formatPortalStatus(c.Portals.Clipboard))
	fmt.Fprintf(&b, "hotkey plan: %s", c.Hotkey.Mode)
	if c.Hotkey.Detail != "" {
		fmt.Fprintf(&b, " (%s)", c.Hotkey.Detail)
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "audio plan: %s", c.Audio.Mode)
	if c.Audio.Detail != "" {
		fmt.Fprintf(&b, " (%s)", c.Audio.Detail)
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "clipboard plan: %s", c.Clipboard.Mode)
	if c.Clipboard.Detail != "" {
		fmt.Fprintf(&b, " (%s)", c.Clipboard.Detail)
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "paste plan: %s", c.Paste.Mode)
	if c.Paste.Detail != "" {
		fmt.Fprintf(&b, " (%s)", c.Paste.Detail)
	}
	fmt.Fprintln(&b)

	names := make([]string, 0, len(c.Binaries))
	for name := range c.Binaries {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Fprintln(&b, "binaries:")
	for _, name := range names {
		binary := c.Binaries[name]
		if binary.Found {
			fmt.Fprintf(&b, "  - %s: %s\n", name, binary.Path)
			continue
		}
		fmt.Fprintf(&b, "  - %s: missing\n", name)
	}

	if len(c.Notes) > 0 {
		fmt.Fprintln(&b, "notes:")
		for _, note := range c.Notes {
			fmt.Fprintf(&b, "  - %s\n", note)
		}
	}

	return b.String()
}

func detectBinaries(names ...string) map[string]Binary {
	result := make(map[string]Binary, len(names))
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err != nil {
			result[name] = Binary{Name: name}
			continue
		}
		result[name] = Binary{Name: name, Path: path, Found: true}
	}
	return result
}

func detectFcitx(binary Binary) FcitxStatus {
	status := FcitxStatus{Binary: binary}
	if binary.Found {
		status.Running = processRunning("fcitx5")
	}

	status.AddonConfigPath = firstExistingPath(
		systemFcitxAddonPaths()...,
	)
	status.ModulePath = firstExistingPath(
		fcitxModuleSearchPatterns()...,
	)

	status.LogPath = fmt.Sprintf("/tmp/coe-fcitx-%d.log", os.Getuid())
	if data, err := os.ReadFile(status.LogPath); err == nil {
		status.LogPresent = true
		status.InitOK = strings.Contains(string(data), "init ok")
	}

	return status
}

func systemFcitxAddonPaths() []string {
	paths := prependIfSet([]string{
		"/usr/share/fcitx5/addon/coe.conf",
		"/usr/local/share/fcitx5/addon/coe.conf",
	}, fcitxAddonPathFromPkgConfig())
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".local/share/fcitx5/addon/coe.conf"))
	}
	return uniqueStrings(paths)
}

func fcitxModuleSearchPatterns() []string {
	patterns := prependIfSet([]string{
		"/usr/lib64/fcitx5/libcoefcitx.so",
		"/usr/lib/*/fcitx5/libcoefcitx.so",
		"/usr/lib/fcitx5/libcoefcitx.so",
		"/usr/local/lib64/fcitx5/libcoefcitx.so",
		"/usr/local/lib/*/fcitx5/libcoefcitx.so",
		"/usr/local/lib/fcitx5/libcoefcitx.so",
	}, fcitxModulePathFromPkgConfig())
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		patterns = append(patterns,
			filepath.Join(home, ".local/lib64/fcitx5/libcoefcitx.so"),
			filepath.Join(home, ".local/lib/*/fcitx5/libcoefcitx.so"),
			filepath.Join(home, ".local/lib/fcitx5/libcoefcitx.so"),
		)
	}
	return uniqueStrings(patterns)
}

func fcitxAddonPathFromPkgConfig() string {
	prefix, _, datadir, ok := fcitxLayoutFromPkgConfig()
	if !ok {
		return ""
	}
	if datadir == "" {
		datadir = filepath.Join(prefix, "share")
	}
	return filepath.Join(datadir, "fcitx5", "addon", "coe.conf")
}

func fcitxModulePathFromPkgConfig() string {
	_, libdir, _, ok := fcitxLayoutFromPkgConfig()
	if !ok {
		return ""
	}
	return filepath.Join(libdir, "fcitx5", "libcoefcitx.so")
}

func fcitxLayoutFromPkgConfig() (prefix string, libdir string, datadir string, ok bool) {
	if _, err := exec.LookPath("pkg-config"); err != nil {
		return "", "", "", false
	}
	if err := exec.Command("pkg-config", "--exists", "Fcitx5Core").Run(); err != nil {
		return "", "", "", false
	}

	prefix = pkgConfigVar("Fcitx5Core", "prefix")
	libdir = pkgConfigVar("Fcitx5Core", "libdir")
	datadir = pkgConfigVar("Fcitx5Core", "datadir")
	if prefix == "" || libdir == "" {
		return "", "", "", false
	}
	if datadir == "" {
		datadir = filepath.Join(prefix, "share")
	}
	return prefix, libdir, datadir, true
}

func pkgConfigVar(pkgName string, variable string) string {
	output, err := exec.Command("pkg-config", "--variable="+variable, pkgName).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func prependIfSet(values []string, value string) []string {
	if value == "" {
		return values
	}
	return append([]string{value}, values...)
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func firstExistingPath(patterns ...string) string {
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			sort.Strings(matches)
			for _, match := range matches {
				if _, statErr := os.Stat(match); statErr == nil {
					return match
				}
			}
		}
		if _, err := os.Stat(pattern); err == nil {
			return pattern
		}
	}
	return ""
}

func processRunning(name string) bool {
	if name == "" {
		return false
	}
	err := exec.Command("pgrep", "-x", name).Run()
	return err == nil
}

func planHotkey(c Capabilities) FeaturePlan {
	switch {
	case c.Portals.GlobalShortcuts.Available:
		return FeaturePlan{Mode: ModePortal, Detail: formatInterfaceDetail(portal.GlobalShortcutsInterface, c.Portals.GlobalShortcuts.Version)}
	case c.SessionType == "wayland" && strings.Contains(c.Desktop, "gnome"):
		return FeaturePlan{Mode: ModeExternalBinding, Detail: "GNOME custom shortcut -> `coe trigger toggle`"}
	default:
		return FeaturePlan{Mode: ModeUnavailable}
	}
}

func planAudio(c Capabilities) FeaturePlan {
	if binary := c.Binaries["pw-record"]; binary.Found {
		return FeaturePlan{Mode: ModeCommand, Detail: binary.Path}
	}
	return FeaturePlan{Mode: ModeUnavailable}
}

func planClipboard(c Capabilities) FeaturePlan {
	switch {
	case c.Portals.Clipboard.Available:
		return FeaturePlan{Mode: ModePortal, Detail: formatInterfaceDetail(portal.ClipboardInterface, c.Portals.Clipboard.Version)}
	case c.Binaries["wl-copy"].Found:
		return FeaturePlan{Mode: ModeCommand, Detail: c.Binaries["wl-copy"].Path}
	default:
		return FeaturePlan{Mode: ModeUnavailable}
	}
}

func planPaste(c Capabilities) FeaturePlan {
	switch {
	case c.Portals.RemoteDesktop.Available:
		return FeaturePlan{Mode: ModePortal, Detail: formatInterfaceDetail(portal.RemoteDesktopInterface, c.Portals.RemoteDesktop.Version)}
	case c.Binaries["ydotool"].Found:
		return FeaturePlan{Mode: ModeCommand, Detail: c.Binaries["ydotool"].Path}
	case c.Binaries["wtype"].Found:
		return FeaturePlan{Mode: ModeCommand, Detail: c.Binaries["wtype"].Path}
	default:
		return FeaturePlan{Mode: ModeUnavailable}
	}
}

func selectProfile(c Capabilities) RuntimeProfile {
	if c.SessionType != "wayland" || !strings.Contains(c.Desktop, "gnome") {
		return ProfileUnsupported
	}

	if c.Hotkey.Mode == ModePortal && c.Audio.Mode == ModeCommand &&
		c.Clipboard.Mode != ModeUnavailable && c.Paste.Mode == ModePortal {
		return ProfileGNOMEFull
	}

	if c.Hotkey.Mode == ModePortal && c.Audio.Mode == ModeCommand &&
		c.Clipboard.Mode != ModeUnavailable {
		return ProfileGNOMEClipboard
	}

	if c.Hotkey.Mode == ModeExternalBinding && c.Audio.Mode == ModeCommand &&
		c.Clipboard.Mode != ModeUnavailable {
		return ProfileExternalTrigger
	}

	return ProfileUnsupported
}

func generateNotes(c Capabilities) []string {
	notes := make([]string, 0, 4)
	if c.SessionType != "wayland" {
		notes = append(notes, "target architecture is Wayland-only for the first milestone")
	}
	if !strings.Contains(c.Desktop, "gnome") {
		notes = append(notes, "current runtime is outside the GNOME-first support target")
	}
	if c.Paste.Mode == ModePortal {
		notes = append(notes, "auto-paste will still depend on portal authorization at runtime")
	}
	if c.Portals.GlobalShortcuts.Available && !c.Portals.RemoteDesktop.Available {
		notes = append(notes, "global trigger is available, but auto-paste is likely clipboard-only")
	}
	if !c.Portals.GlobalShortcuts.Available && c.Portals.RemoteDesktop.Available && c.Portals.Clipboard.Available &&
		strings.Contains(c.Desktop, "gnome") {
		notes = append(notes, "this GNOME session exposes RemoteDesktop and Clipboard but not GlobalShortcuts")
	}
	if c.Hotkey.Mode == ModeExternalBinding {
		notes = append(notes, "external binding is a degraded fallback and will not preserve hold-to-talk semantics by itself")
	}
	if c.Fcitx.Binary.Found && c.Fcitx.ModulePath == "" {
		notes = append(notes, "fcitx5 is installed but the Coe module library was not found")
	}
	if c.Fcitx.Binary.Found && c.Fcitx.AddonConfigPath == "" {
		notes = append(notes, "fcitx5 is installed but the Coe addon config was not found")
	}
	if c.Fcitx.LogPresent && !c.Fcitx.InitOK {
		notes = append(notes, "fcitx5 module log exists but has not reported init ok")
	}
	return notes
}

func blankAsUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func blankAsMissing(value string) string {
	if value == "" {
		return "missing"
	}
	return value
}

func formatBinaryStatus(binary Binary) string {
	if binary.Found {
		return fmt.Sprintf("true (%s)", binary.Path)
	}
	return "false"
}

func formatFcitxInitMarker(status FcitxStatus) string {
	switch {
	case status.LogPresent && status.InitOK:
		return fmt.Sprintf("true (%s)", status.LogPath)
	case status.LogPresent:
		return fmt.Sprintf("present without init ok (%s)", status.LogPath)
	default:
		return "false"
	}
}

func formatPortalStatus(status portal.InterfaceStatus) string {
	if !status.Available {
		return "false"
	}
	if status.Version == 0 {
		return "true"
	}
	return fmt.Sprintf("true (v%d)", status.Version)
}

func formatInterfaceDetail(name string, version uint32) string {
	if version == 0 {
		return name
	}
	return fmt.Sprintf("%s v%d", name, version)
}
