package asr

import (
	"net/http"
	"testing"
	"time"

	"coe/internal/config"
)

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

func TestNewClientHTTPTimeoutFromConfig(t *testing.T) {
	t.Parallel()

	client, err := NewClient(config.ASRConfig{Provider: ProviderOpenAI, TimeoutSeconds: 7})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	openAIClient, ok := client.(OpenAIClient)
	if !ok {
		t.Fatalf("client type = %T", client)
	}
	if openAIClient.HTTPClient == nil {
		t.Fatal("HTTPClient is nil")
	}
	if openAIClient.HTTPClient.Timeout != 7*time.Second {
		t.Fatalf("HTTPClient.Timeout = %v", openAIClient.HTTPClient.Timeout)
	}
}

func TestNewHTTPClientUsesFallback(t *testing.T) {
	t.Parallel()

	client := newHTTPClient(0, 60)
	if client.Timeout != 60*time.Second {
		t.Fatalf("client.Timeout = %v", client.Timeout)
	}

	client = newHTTPClient(-1, 60)
	if client.Timeout != 60*time.Second {
		t.Fatalf("client.Timeout = %v", client.Timeout)
	}
}

func TestNewHTTPClientUsesConfiguredTimeout(t *testing.T) {
	t.Parallel()

	client := newHTTPClient(12, 60)
	if client.Timeout != 12*time.Second {
		t.Fatalf("client.Timeout = %v", client.Timeout)
	}
	if _, ok := any(client).(*http.Client); !ok {
		t.Fatal("expected *http.Client")
	}
}
