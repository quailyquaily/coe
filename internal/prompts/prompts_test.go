package prompts

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveASRDefault(t *testing.T) {
	got, err := ResolveASR("", "", ASRTemplateData{
		Provider: "openai",
		Model:    "gpt-4o-mini-transcribe",
		Language: "zh",
	})
	if err != nil {
		t.Fatalf("ResolveASR() error = %v", err)
	}
	if got != "" {
		t.Fatalf("ResolveASR() = %q, want empty", got)
	}
}

func TestResolveASROverrideTemplateFile(t *testing.T) {
	path := filepath.Join("testdata", "asr-override.tmpl")

	got, err := ResolveASR("", path, ASRTemplateData{
		Provider: "openai",
		Model:    "gpt-4o-mini-transcribe",
		Language: "zh",
	})
	if err != nil {
		t.Fatalf("ResolveASR() error = %v", err)
	}
	if got != "Hint for openai / zh / gpt-4o-mini-transcribe" {
		t.Fatalf("ResolveASR() = %q", got)
	}
}

func TestResolveLLMCorrectionDefault(t *testing.T) {
	got, err := ResolveLLMCorrection("", "", LLMTemplateData{
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		EndpointType: "chat",
	})
	if err != nil {
		t.Fatalf("ResolveLLMCorrection() error = %v", err)
	}
	if got == "" {
		t.Fatal("expected default correction prompt")
	}
	for _, fragment := range []string{
		"TASK: clean ASR dictation text.",
		"if the utterance mixes languages",
		"drop filler / discourse particles",
		"dedupe accidental repeated words / phrases",
		"number words -> Arabic numerals",
		"EXAMPLES:",
		"we need three people",
		"住在二十一楼",
		"我住在21楼",
		"用 grep 查一下 error log",
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("default correction prompt missing %q", fragment)
		}
	}
}

func TestResolveLLMCorrectionIncludesDictionaryWhenProvided(t *testing.T) {
	got, err := ResolveLLMCorrection("", "", LLMTemplateData{
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		EndpointType: "chat",
		Dictionary:   "- \"system control\" => \"systemctl\"",
	})
	if err != nil {
		t.Fatalf("ResolveLLMCorrection() error = %v", err)
	}
	for _, fragment := range []string{
		"PERSONAL DICTIONARY:",
		`- "system control" => "systemctl"`,
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("dictionary prompt missing %q", fragment)
		}
	}
}

func TestResolveLLMCorrectionOverrideTemplateFile(t *testing.T) {
	path := filepath.Join("testdata", "llm-override.tmpl")

	got, err := ResolveLLMCorrection("", path, LLMTemplateData{
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		EndpointType: "chat",
	})
	if err != nil {
		t.Fatalf("ResolveLLMCorrection() error = %v", err)
	}
	if got != "Fix text with gpt-4o-mini via chat" {
		t.Fatalf("ResolveLLMCorrection() = %q", got)
	}
}

func TestResolveLLMCorrectionTerminalDefault(t *testing.T) {
	got, err := ResolveLLMCorrectionTerminal("", "", LLMTemplateData{
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		EndpointType: "responses",
	})
	if err != nil {
		t.Fatalf("ResolveLLMCorrectionTerminal() error = %v", err)
	}
	for _, fragment := range []string{
		"terminal / shell usage",
		"commands, flags, paths, filenames",
		"drop filler / discourse particles",
		`"sudo systemctl restart coe.service"`,
		`"帮我跑一下 ls dash la" -> "帮我跑一下 ls -la"`,
		`"用 grep 查一下 error log" -> "用 grep 查一下 error log"`,
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("terminal correction prompt missing %q", fragment)
		}
	}
}

func TestResolveSceneRouterDefault(t *testing.T) {
	got, err := ResolveSceneRouter(LLMTemplateData{
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		EndpointType: "responses",
	})
	if err != nil {
		t.Fatalf("ResolveSceneRouter() error = %v", err)
	}
	for _, fragment := range []string{
		"route a scene-switch command from JSON input",
		`{"intent":"switch_scene","target_scene":"terminal","matched_alias":"终端"}`,
		`{"intent":"no_match"}`,
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("scene router prompt missing %q", fragment)
		}
	}
}
