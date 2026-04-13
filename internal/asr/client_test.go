package asr

import "testing"

func TestNormalizeProviderName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty becomes stub",
			input: "",
			want:  ProviderStub,
		},
		{
			name:  "whisper alias is normalized",
			input: "Whisper.Cpp",
			want:  ProviderWhisperCPP,
		},
		{
			name:  "doubao flash alias is normalized",
			input: "Doubao-Flash",
			want:  ProviderDoubao,
		},
		{
			name:  "qwen provider is preserved",
			input: "qwen3-asr-vllm",
			want:  ProviderQwen3ASRVLLM,
		},
		{
			name:  "voxtype provider is preserved",
			input: "voxtype",
			want:  ProviderVoxtype,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeProviderName(tt.input); got != tt.want {
				t.Fatalf("NormalizeProviderName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSupportedProvider(t *testing.T) {
	t.Parallel()

	if !SupportedProvider(ProviderQwen3ASRVLLM) {
		t.Fatalf("SupportedProvider(%q) = false, want true", ProviderQwen3ASRVLLM)
	}
	if !SupportedProvider(ProviderDoubao) {
		t.Fatalf("SupportedProvider(%q) = false, want true", ProviderDoubao)
	}
	if !SupportedProvider(ProviderVoxtype) {
		t.Fatalf("SupportedProvider(%q) = false, want true", ProviderVoxtype)
	}
	if SupportedProvider("unknown") {
		t.Fatal(`SupportedProvider("unknown") = true, want false`)
	}
}
