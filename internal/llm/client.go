package llm

import (
	"context"
	"fmt"
	"strings"

	"coe/internal/config"
)

type Result struct {
	Text string
}

type Corrector interface {
	Correct(context.Context, string) (Result, error)
	Name() string
}

func NewCorrector(provider config.LLMConfig) (Corrector, error) {
	switch strings.ToLower(provider.Provider) {
	case "", "stub":
		return StubCorrector{}, nil
	case "openai":
		return OpenAICorrector{
			Endpoint:     provider.Endpoint,
			EndpointType: provider.EndpointType,
			Model:        provider.Model,
			APIKey:       provider.APIKey,
			APIKeyEnv:    provider.APIKeyEnv,
			Prompt:       provider.Prompt,
			PromptFile:   provider.PromptFile,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider %q", provider.Provider)
	}
}

type StubCorrector struct{}

func (StubCorrector) Correct(_ context.Context, input string) (Result, error) {
	return Result{Text: input}, nil
}

func (StubCorrector) Name() string {
	return "stub-llm"
}
