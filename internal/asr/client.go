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

func NewClient(provider config.ASRConfig) (Client, error) {
	switch strings.ToLower(provider.Provider) {
	case "", "stub":
		return StubClient{}, nil
	case "openai":
		return OpenAIClient{
			Endpoint:   provider.Endpoint,
			Model:      provider.Model,
			APIKey:     provider.APIKey,
			APIKeyEnv:  provider.APIKeyEnv,
			Language:   provider.Language,
			Prompt:     provider.Prompt,
			PromptFile: provider.PromptFile,
		}, nil
	case "whispercpp", "whisper.cpp":
		return WhisperCPPCLIClient{
			Binary:     provider.Binary,
			ModelPath:  provider.ModelPath,
			Language:   provider.Language,
			Prompt:     provider.Prompt,
			PromptFile: provider.PromptFile,
			Threads:    provider.Threads,
			UseGPU:     provider.UseGPU,
		}, nil
	case "sensevoice":
		return SenseVoiceHTTPClient{
			Endpoint: provider.Endpoint,
			Language: provider.Language,
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
