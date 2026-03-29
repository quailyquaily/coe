package i18n

import "testing"

func TestNewFromEnvironmentUsesLocalePriority(t *testing.T) {
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_MESSAGES", "ja_JP.UTF-8")
	t.Setenv("LC_ALL", "zh_CN.UTF-8")

	got := NewFromEnvironment()
	if got.Locale() != LocaleChinese {
		t.Fatalf("Locale() = %q, want %q", got.Locale(), LocaleChinese)
	}
}

func TestNewForLocaleFallsBackToEnglish(t *testing.T) {
	got := NewForLocale("fr_FR.UTF-8")
	if got.Locale() != LocaleEnglish {
		t.Fatalf("Locale() = %q, want %q", got.Locale(), LocaleEnglish)
	}
}

func TestNewFromEnvironmentSkipsEmptyLocaleVariables(t *testing.T) {
	t.Setenv("LANG", "ja_JP.UTF-8")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LC_ALL", "")

	got := NewFromEnvironment()
	if got.Locale() != LocaleJapanese {
		t.Fatalf("Locale() = %q, want %q", got.Locale(), LocaleJapanese)
	}
}

func TestLocalizeWarningTranslatesKnownWarnings(t *testing.T) {
	got := NewForLocale("zh_CN.UTF-8").LocalizeWarning("captured audio is near-silent; skipped transcription")
	if got != "录到的音频几乎无声，已跳过转写。" {
		t.Fatalf("LocalizeWarning() = %q", got)
	}
}
