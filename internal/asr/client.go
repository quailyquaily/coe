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

func NewClient(provider config.Provider) (Client, error) {
	switch strings.ToLower(provider.Kind) {
	case "", "stub":
		return StubClient{}, nil
	case "openai":
		return OpenAIClient{
			Endpoint:  provider.Endpoint,
			Model:     provider.Model,
			APIKeyEnv: provider.APIKeyEnv,
			Language:  provider.Language,
			Prompt:    provider.Prompt,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported ASR provider kind %q", provider.Kind)
	}
}

type StubClient struct{}

func (StubClient) Transcribe(_ context.Context, capture audio.Result) (Result, error) {
	return Result{Text: fmt.Sprintf("[stub transcription from %d bytes]", capture.ByteCount)}, nil
}

func (StubClient) Name() string {
	return "stub-asr"
}
