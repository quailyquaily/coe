package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"coe/internal/prompts"

	"github.com/quailyquaily/uniai"
)

const defaultOpenAIResponsesEndpoint = "https://api.openai.com/v1/responses"
const defaultOpenAIAPIBase = "https://api.openai.com/v1"

type OpenAICorrector struct {
	Endpoint       string
	EndpointType   string
	Model          string
	APIKey         string
	APIKeyEnv      string
	Prompt         string
	PromptFile     string
	PromptTemplate string
	ResolvedPrompt string
	Timeout        time.Duration
	HTTPClient     *http.Client
}

func (c OpenAICorrector) Name() string {
	endpointType := normalizeEndpointType(c.EndpointType)
	model := c.Model
	if model == "" {
		model = "gpt-4o-mini"
	}
	return "openai-" + endpointType + "-" + model
}

func (c OpenAICorrector) Correct(ctx context.Context, input string) (Result, error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	apiKey, _, err := resolveAPIKey(c.APIKey, c.APIKeyEnv)
	if err != nil {
		return Result{}, err
	}

	endpointType := normalizeEndpointType(c.EndpointType)
	instructions := strings.TrimSpace(c.ResolvedPrompt)
	if instructions == "" {
		templateName := strings.TrimSpace(c.PromptTemplate)
		if templateName == "" {
			templateName = prompts.TemplateLLMCorrection
		}
		instructions, err = prompts.ResolveNamed(templateName, c.Prompt, c.PromptFile, prompts.LLMTemplateData{
			Provider:     "openai",
			Model:        defaultCorrectorModel(c.Model),
			EndpointType: endpointType,
		})
		if err != nil {
			return Result{}, err
		}
	}

	switch endpointType {
	case "responses":
		return c.correctViaResponses(ctx, input, apiKey, instructions)
	case "chat":
		return c.correctViaChat(ctx, input, apiKey, instructions)
	default:
		return Result{}, fmt.Errorf("unsupported OpenAI endpoint type %q", c.EndpointType)
	}
}

func (c OpenAICorrector) correctViaResponses(ctx context.Context, input, apiKey, instructions string) (Result, error) {
	model := defaultCorrectorModel(c.Model)
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = defaultOpenAIResponsesEndpoint
	}

	payload := map[string]any{
		"model":             model,
		"instructions":      instructions,
		"input":             input,
		"max_output_tokens": 300,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
			return Result{}, fmt.Errorf("openai correction failed: %s", apiErr.Error.Message)
		}
		return Result{}, fmt.Errorf("openai correction failed: %s", resp.Status)
	}

	var payloadResp responsePayload
	if err := json.NewDecoder(resp.Body).Decode(&payloadResp); err != nil {
		return Result{}, err
	}

	return Result{Text: strings.TrimSpace(payloadResp.text())}, nil
}

func (c OpenAICorrector) correctViaChat(ctx context.Context, input, apiKey, instructions string) (Result, error) {
	model := defaultCorrectorModel(c.Model)

	client := uniai.New(uniai.Config{
		Provider:      "openai",
		OpenAIAPIKey:  apiKey,
		OpenAIAPIBase: normalizeOpenAIAPIBase(c.Endpoint),
		OpenAIModel:   model,
	})

	resp, err := client.Chat(
		ctx,
		uniai.WithProvider("openai"),
		uniai.WithModel(model),
		uniai.WithMessages(
			uniai.System(instructions),
			uniai.User(input),
		),
		uniai.WithMaxTokens(300),
	)
	if err != nil {
		return Result{}, fmt.Errorf("openai correction failed: %w", err)
	}

	return Result{Text: strings.TrimSpace(resp.Text)}, nil
}

type responsePayload struct {
	OutputText string           `json:"output_text"`
	Output     []responseOutput `json:"output"`
}

type responseOutput struct {
	Type    string                  `json:"type"`
	Content []responseOutputContent `json:"content"`
}

type responseOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (p responsePayload) text() string {
	if strings.TrimSpace(p.OutputText) != "" {
		return p.OutputText
	}

	var parts []string
	for _, item := range p.Output {
		if item.Type != "message" {
			continue
		}
		for _, content := range item.Content {
			if content.Type != "output_text" {
				continue
			}
			text := strings.TrimSpace(content.Text)
			if text != "" {
				parts = append(parts, text)
			}
		}
	}

	return strings.Join(parts, "\n")
}

func defaultCorrectorModel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "gpt-4o-mini"
	}
	return strings.TrimSpace(value)
}

func normalizeEndpointType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "response", "responses":
		return "responses"
	case "chat":
		return "chat"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeOpenAIAPIBase(endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if trimmed == "" {
		return defaultOpenAIAPIBase
	}

	for _, suffix := range []string{
		"/responses",
		"/chat/completions",
		"/audio/transcriptions",
	} {
		if strings.HasSuffix(trimmed, suffix) {
			return strings.TrimSuffix(trimmed, suffix)
		}
	}

	return trimmed
}

func resolveAPIKey(explicit, envName string) (string, string, error) {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit), "config", nil
	}

	keyEnv := strings.TrimSpace(envName)
	if keyEnv == "" {
		keyEnv = "OPENAI_API_KEY"
	}

	apiKey := strings.TrimSpace(os.Getenv(keyEnv))
	if apiKey == "" {
		return "", keyEnv, fmt.Errorf("missing OpenAI API key in %s", keyEnv)
	}

	return apiKey, keyEnv, nil
}
