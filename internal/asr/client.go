package asr

import (
	"context"
	"fmt"
	"strings"

	"coe/internal/audio"
	"coe/internal/config"
)

type Result struct {
	Text    string
	Warning string
}

type Client interface {
	Transcribe(context.Context, audio.Result) (Result, error)
	Name() string
}

const (
	ProviderStub            = "stub"
	ProviderOpenAI          = "openai"
	ProviderWhisperCPP      = "whispercpp"
	ProviderSenseVoice      = "sensevoice"
	ProviderQwen3ASRVLLM    = "qwen3-asr-vllm"
	ProviderWhisperCPPAlias = "whisper.cpp"
)

func NormalizeProviderName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", ProviderStub:
		return ProviderStub
	case ProviderWhisperCPPAlias, ProviderWhisperCPP:
		return ProviderWhisperCPP
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func SupportedProvider(value string) bool {
	switch NormalizeProviderName(value) {
	case ProviderStub, ProviderOpenAI, ProviderWhisperCPP, ProviderSenseVoice, ProviderQwen3ASRVLLM:
		return true
	default:
		return false
	}
}

func NewClient(provider config.ASRConfig) (Client, error) {
	switch NormalizeProviderName(provider.Provider) {
	case ProviderStub:
		return StubClient{}, nil
	case ProviderOpenAI:
		return OpenAIClient{
			Endpoint:   provider.Endpoint,
			Model:      provider.Model,
			APIKey:     provider.APIKey,
			APIKeyEnv:  provider.APIKeyEnv,
			Language:   provider.Language,
			Prompt:     provider.Prompt,
			PromptFile: provider.PromptFile,
		}, nil
	case ProviderWhisperCPP:
		return WhisperCPPCLIClient{
			Binary:     provider.Binary,
			ModelPath:  provider.ModelPath,
			Language:   provider.Language,
			Prompt:     provider.Prompt,
			PromptFile: provider.PromptFile,
			Threads:    provider.Threads,
			UseGPU:     provider.UseGPU,
		}, nil
	case ProviderSenseVoice:
		return SenseVoiceHTTPClient{
			Endpoint: provider.Endpoint,
			Language: provider.Language,
		}, nil
	case ProviderQwen3ASRVLLM:
		return Qwen3ASRVLLMClient{
			Endpoint:  provider.Endpoint,
			Model:     provider.Model,
			APIKey:    provider.APIKey,
			APIKeyEnv: provider.APIKeyEnv,
			Prompt:    provider.Prompt,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported ASR provider %q", provider.Provider)
	}
}

type StubClient struct{}

func (StubClient) Transcribe(_ context.Context, capture audio.Result) (Result, error) {
	return Result{Text: fmt.Sprintf("[stub transcription from %d bytes]", capture.ByteCount)}, nil
}

func (StubClient) Name() string {
	return "stub-asr"
}
