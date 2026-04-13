package main

import (
	"strings"
	"testing"

	"coe/internal/config"
)

func TestValidateASRConfigSupportsQwen3ASRVLLM(t *testing.T) {
	t.Parallel()

	check := validateASRConfig(config.ASRConfig{
		Provider: "qwen3-asr-vllm",
	})

	if !check.OK {
		t.Fatalf("validateASRConfig().OK = false, detail=%q problem=%q", check.Detail, check.Problem)
	}
	for _, want := range []string{
		"provider=qwen3-asr-vllm",
		"http://127.0.0.1:8000/v1/chat/completions",
		"model=Qwen3-ASR",
	} {
		if !strings.Contains(check.Detail, want) {
			t.Fatalf("validateASRConfig().Detail missing %q in %q", want, check.Detail)
		}
	}
	if check.Problem != "" {
		t.Fatalf("validateASRConfig().Problem = %q, want empty", check.Problem)
	}
}

func TestValidateASRConfigSupportsDoubao(t *testing.T) {
	t.Setenv("DOUBAO_ASR_API_KEY", "test-key")

	check := validateASRConfig(config.ASRConfig{
		Provider: "doubao",
	})

	if !check.OK {
		t.Fatalf("validateASRConfig().OK = false, detail=%q problem=%q", check.Detail, check.Problem)
	}
	for _, want := range []string{
		"provider=doubao",
		"https://openspeech.bytedance.com/api/v3/auc/bigmodel/recognize/flash",
		"api_key=env:DOUBAO_ASR_API_KEY",
	} {
		if !strings.Contains(check.Detail, want) {
			t.Fatalf("validateASRConfig().Detail missing %q in %q", want, check.Detail)
		}
	}
	if check.Problem != "" {
		t.Fatalf("validateASRConfig().Problem = %q, want empty", check.Problem)
	}
}

func TestValidateASRConfigSupportsVoxtype(t *testing.T) {
	t.Parallel()

	check := validateASRConfig(config.ASRConfig{
		Provider: "voxtype",
		Binary:   "sh",
		Engine:   "omnilingual",
	})

	if !check.OK {
		t.Fatalf("validateASRConfig().OK = false, detail=%q problem=%q", check.Detail, check.Problem)
	}
	for _, want := range []string{
		"provider=voxtype",
		"engine=omnilingual",
	} {
		if !strings.Contains(check.Detail, want) {
			t.Fatalf("validateASRConfig().Detail missing %q in %q", want, check.Detail)
		}
	}
	if check.Problem != "" {
		t.Fatalf("validateASRConfig().Problem = %q, want empty", check.Problem)
	}
}
