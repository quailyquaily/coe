package llm

import (
	"context"
	"fmt"
	"strings"

	"coe/internal/config"
	"coe/internal/prompts"
)

type Result struct {
	Text string
}

type Corrector interface {
	Correct(context.Context, string) (Result, error)
	Name() string
}

func NewCorrector(provider config.LLMConfig) (Corrector, error) {
	return NewCorrectorWithTemplate(provider, "")
}

func NewCorrectorWithTemplate(provider config.LLMConfig, promptTemplate string) (Corrector, error) {
	switch strings.ToLower(provider.Provider) {
	case "", "stub":
		return StubCorrector{}, nil
	case "openai":
		if strings.TrimSpace(promptTemplate) == "" {
			promptTemplate = prompts.TemplateLLMCorrection
		}
		return OpenAICorrector{
			Endpoint:       provider.Endpoint,
			EndpointType:   provider.EndpointType,
			Model:          provider.Model,
			APIKey:         provider.APIKey,
			APIKeyEnv:      provider.APIKeyEnv,
			Prompt:         provider.Prompt,
			PromptFile:     provider.PromptFile,
			PromptTemplate: promptTemplate,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider %q", provider.Provider)
	}
}

func NewCorrectorWithResolvedPrompt(provider config.LLMConfig, resolvedPrompt string) (Corrector, error) {
	switch strings.ToLower(provider.Provider) {
	case "", "stub":
		return StubCorrector{}, nil
	case "openai":
		return OpenAICorrector{
			Endpoint:       provider.Endpoint,
			EndpointType:   provider.EndpointType,
			Model:          provider.Model,
			APIKey:         provider.APIKey,
			APIKeyEnv:      provider.APIKeyEnv,
			ResolvedPrompt: strings.TrimSpace(resolvedPrompt),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider %q", provider.Provider)
	}
}

func ResolvePrompt(provider config.LLMConfig, promptTemplate string, templateData prompts.LLMTemplateData) (string, error) {
	switch strings.ToLower(provider.Provider) {
	case "", "stub":
		return "", nil
	case "openai":
		templateName := strings.TrimSpace(promptTemplate)
		if templateName == "" {
			templateName = prompts.TemplateLLMCorrection
		}
		templateData.Provider = "openai"
		templateData.Model = defaultCorrectorModel(provider.Model)
		templateData.EndpointType = normalizeEndpointType(provider.EndpointType)
		return prompts.ResolveNamed(templateName, provider.Prompt, provider.PromptFile, templateData)
	default:
		return "", fmt.Errorf("unsupported LLM provider %q", provider.Provider)
	}
}

type StubCorrector struct{}

func (StubCorrector) Correct(_ context.Context, input string) (Result, error) {
	return Result{Text: input}, nil
}

func (StubCorrector) Name() string {
	return "stub-llm"
}
