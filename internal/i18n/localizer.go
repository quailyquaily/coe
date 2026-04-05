package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Locale string

const (
	LocaleEnglish  Locale = "en"
	LocaleChinese  Locale = "zh"
	LocaleJapanese Locale = "ja"
)

type Key string

const (
	RecordingStartedTitle       Key = "recording_started_title"
	RecordingStartedBody        Key = "recording_started_body"
	ServiceReadyTitle           Key = "service_ready_title"
	ServiceReadyBody            Key = "service_ready_body"
	ServiceReadyTriggerLine     Key = "service_ready_trigger_line"
	SceneSwitchedTitle          Key = "scene_switched_title"
	SceneGeneralName            Key = "scene_general_name"
	SceneTerminalName           Key = "scene_terminal_name"
	NoSpeechDetectedTitle       Key = "no_speech_detected_title"
	NoSpeechDetectedFallback    Key = "no_speech_detected_fallback"
	DeliveryClipboard           Key = "delivery_clipboard"
	DeliveryFcitx               Key = "delivery_fcitx"
	DeliveryPasted              Key = "delivery_pasted"
	DeliveryNoAction            Key = "delivery_no_action"
	AutoPasteNeedsAttention     Key = "auto_paste_needs_attention"
	CorrectionFallback          Key = "correction_fallback"
	DictationCompleteTitle      Key = "dictation_complete_title"
	RecordingFailedToStartTitle Key = "recording_failed_to_start_title"
	RecordingFailedTitle        Key = "recording_failed_title"
	DictationFailedTitle        Key = "dictation_failed_title"
	WarningNearSilent           Key = "warning_near_silent"
	WarningCorruptAudio         Key = "warning_corrupt_audio"
	WarningEmptyTranscript      Key = "warning_empty_transcript"
	HotkeyPickerTitle           Key = "hotkey_picker_title"
	HotkeyPickerHeading         Key = "hotkey_picker_heading"
	HotkeyPickerHint            Key = "hotkey_picker_hint"
	HotkeyPickerWaiting         Key = "hotkey_picker_waiting"
	HotkeyPickerCapturedFormat  Key = "hotkey_picker_captured_format"
	HotkeyPickerPressFirst      Key = "hotkey_picker_press_first"
	HotkeyPickerConfirm         Key = "hotkey_picker_confirm"
	HotkeyPickerCancel          Key = "hotkey_picker_cancel"
)

type Localizer struct {
	locale Locale
}

func NewFromEnvironment() Localizer {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if locale := normalizeLocale(os.Getenv(name)); locale != "" {
			return Localizer{locale: locale}
		}
	}
	return Localizer{locale: LocaleEnglish}
}

func NewForLocale(value string) Localizer {
	locale := normalizeLocale(value)
	if locale == "" {
		locale = LocaleEnglish
	}
	return Localizer{locale: locale}
}

func (l Localizer) Locale() Locale {
	if l.locale == "" {
		return LocaleEnglish
	}
	return l.locale
}

func (l Localizer) Text(key Key) string {
	localeMessages, ok := messageCatalog[l.Locale()]
	if ok {
		if value, ok := localeMessages[key]; ok {
			return value
		}
	}
	return messageCatalog[LocaleEnglish][key]
}

func (l Localizer) Format(key Key, args ...any) string {
	return fmt.Sprintf(l.Text(key), args...)
}

func (l Localizer) LocalizeWarning(message string) string {
	trimmed := strings.TrimSpace(message)
	switch trimmed {
	case "captured audio is near-silent; skipped transcription":
		return l.Text(WarningNearSilent)
	case "captured audio appears saturated or corrupted; skipped transcription":
		return l.Text(WarningCorruptAudio)
	case "ASR returned empty transcript; skipped correction and output":
		return l.Text(WarningEmptyTranscript)
	default:
		return trimmed
	}
}

func normalizeLocale(value string) Locale {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch {
	case trimmed == "":
		return ""
	case trimmed == "c", trimmed == "c.utf-8", trimmed == "posix":
		return LocaleEnglish
	case strings.HasPrefix(trimmed, "zh"):
		return LocaleChinese
	case strings.HasPrefix(trimmed, "ja"):
		return LocaleJapanese
	default:
		return LocaleEnglish
	}
}

//go:embed locales/*.json
var localeFiles embed.FS

var messageCatalog = mustLoadCatalog()

func mustLoadCatalog() map[Locale]map[Key]string {
	catalog := map[Locale]map[Key]string{}
	for _, locale := range []Locale{LocaleEnglish, LocaleChinese, LocaleJapanese} {
		path := filepath.Join("locales", string(locale)+".json")
		data, err := localeFiles.ReadFile(path)
		if err != nil {
			panic(fmt.Sprintf("load locale catalog %s: %v", path, err))
		}

		raw := map[string]string{}
		if err := json.Unmarshal(data, &raw); err != nil {
			panic(fmt.Sprintf("parse locale catalog %s: %v", path, err))
		}

		translated := make(map[Key]string, len(raw))
		for key, value := range raw {
			translated[Key(key)] = value
		}
		catalog[locale] = translated
	}

	return catalog
}
