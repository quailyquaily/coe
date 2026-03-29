package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"coe/internal/audio"
	"coe/internal/prompts"
)

const (
	defaultOpenAITranscriptionEndpoint = "https://api.openai.com/v1/audio/transcriptions"
	maxOpenAIAudioUploadBytes          = 25 * 1024 * 1024
)

type OpenAIClient struct {
	Endpoint   string
	Model      string
	APIKey     string
	APIKeyEnv  string
	Language   string
	Prompt     string
	PromptFile string
	HTTPClient *http.Client
}

func (c OpenAIClient) Name() string {
	model := c.Model
	if model == "" {
		model = "gpt-4o-mini-transcribe"
	}
	return "openai-" + model
}

func (c OpenAIClient) Transcribe(ctx context.Context, capture audio.Result) (Result, error) {
	wav, err := audio.EncodeWAV(capture)
	if err != nil {
		return Result{}, err
	}
	if len(wav) > maxOpenAIAudioUploadBytes {
		return Result{}, fmt.Errorf("audio payload is %d bytes, over OpenAI 25 MB upload limit", len(wav))
	}

	prompt, err := prompts.ResolveASR(c.Prompt, c.PromptFile, prompts.ASRTemplateData{
		Provider: "openai",
		Model:    defaultOpenAIModel(c.Model),
		Language: strings.TrimSpace(c.Language),
	})
	if err != nil {
		return Result{}, err
	}

	first, err := c.transcribeOnce(ctx, wav, c.Language, prompt)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(first.Text) != "" {
		return Result{
			Text:    strings.TrimSpace(first.Text),
			Warning: strings.TrimSpace(first.warning()),
		}, nil
	}
	if c.Language != "" {
		retry, err := c.transcribeOnce(ctx, wav, "", prompt)
		if err != nil {
			return Result{}, err
		}
		if strings.TrimSpace(retry.Text) != "" {
			return Result{
				Text:    strings.TrimSpace(retry.Text),
				Warning: "initial transcription returned empty text with language hint; retry without language hint succeeded",
			}, nil
		}
	}

	return Result{
		Text:    "",
		Warning: strings.TrimSpace(first.warning()),
	}, nil
}

func (c OpenAIClient) transcribeOnce(ctx context.Context, wav []byte, language, prompt string) (openAITranscriptionPayload, error) {
	apiKey, _, err := resolveAPIKey(c.APIKey, c.APIKeyEnv)
	if err != nil {
		return openAITranscriptionPayload{}, err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	fileWriter, err := writer.CreateFormFile("file", "speech.wav")
	if err != nil {
		return openAITranscriptionPayload{}, err
	}
	if _, err := fileWriter.Write(wav); err != nil {
		return openAITranscriptionPayload{}, err
	}

	model := defaultOpenAIModel(c.Model)
	if err := writer.WriteField("model", model); err != nil {
		return openAITranscriptionPayload{}, err
	}
	if err := writer.WriteField("response_format", "json"); err != nil {
		return openAITranscriptionPayload{}, err
	}
	if language != "" {
		if err := writer.WriteField("language", language); err != nil {
			return openAITranscriptionPayload{}, err
		}
	}
	if prompt != "" {
		if err := writer.WriteField("prompt", prompt); err != nil {
			return openAITranscriptionPayload{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return openAITranscriptionPayload{}, err
	}

	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = defaultOpenAITranscriptionEndpoint
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return openAITranscriptionPayload{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return openAITranscriptionPayload{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
			return openAITranscriptionPayload{}, fmt.Errorf("openai transcription failed: %s", apiErr.Error.Message)
		}
		return openAITranscriptionPayload{}, fmt.Errorf("openai transcription failed: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return openAITranscriptionPayload{}, err
	}

	var payload openAITranscriptionPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return openAITranscriptionPayload{}, err
	}
	payload.Raw = strings.TrimSpace(string(data))
	payload.LanguageHint = language
	return payload, nil
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

type openAITranscriptionPayload struct {
	Text         string `json:"text"`
	Language     string `json:"language"`
	Duration     any    `json:"duration"`
	Raw          string `json:"-"`
	LanguageHint string `json:"-"`
}

func (p openAITranscriptionPayload) warning() string {
	if strings.TrimSpace(p.Text) != "" {
		return ""
	}

	parts := []string{"OpenAI transcription returned empty text"}
	if p.LanguageHint != "" {
		parts = append(parts, fmt.Sprintf("language_hint=%s", p.LanguageHint))
	}
	if strings.TrimSpace(p.Language) != "" {
		parts = append(parts, fmt.Sprintf("detected_language=%s", p.Language))
	}
	if p.Duration != nil {
		parts = append(parts, fmt.Sprintf("duration=%v", p.Duration))
	}
	if p.Raw != "" {
		parts = append(parts, fmt.Sprintf("raw=%s", truncateForWarning(p.Raw, 240)))
	}
	return strings.Join(parts, "; ")
}

func truncateForWarning(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func defaultOpenAIModel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "gpt-4o-mini-transcribe"
	}
	return strings.TrimSpace(value)
}
