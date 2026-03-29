package asr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"coe/internal/audio"
)

const defaultQwen3ASRVLLMEndpoint = "http://127.0.0.1:8000/v1/chat/completions"

type Qwen3ASRVLLMClient struct {
	Endpoint   string
	Model      string
	APIKey     string
	APIKeyEnv  string
	Prompt     string
	HTTPClient *http.Client
}

func (c Qwen3ASRVLLMClient) Name() string {
	model := strings.TrimSpace(c.Model)
	if model == "" {
		model = "Qwen3-ASR"
	}
	return "qwen3-asr-vllm-" + model
}

func (c Qwen3ASRVLLMClient) Transcribe(ctx context.Context, capture audio.Result) (Result, error) {
	wav, err := audio.EncodeWAV(capture)
	if err != nil {
		return Result{}, err
	}

	payload := qwen3ASRChatRequest{
		Model: c.model(),
		Messages: []qwen3ASRChatMessage{
			{
				Role: "user",
				Content: c.contentWithAudio(
					base64.StdEncoding.EncodeToString(wav),
				),
			},
		},
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return Result{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), &body)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey := resolveOptionalAPIKey(c.APIKey, c.APIKeyEnv); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, err
	}

	if resp.StatusCode >= 300 {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &apiErr); err == nil && strings.TrimSpace(apiErr.Error.Message) != "" {
			return Result{}, fmt.Errorf("qwen3-asr-vllm transcription failed: %s", apiErr.Error.Message)
		}
		return Result{}, fmt.Errorf("qwen3-asr-vllm transcription failed: %s", resp.Status)
	}

	var chatResp qwen3ASRChatResponse
	if err := json.Unmarshal(data, &chatResp); err != nil {
		return Result{}, err
	}
	if len(chatResp.Choices) == 0 {
		return Result{Warning: "qwen3-asr-vllm returned no choices"}, nil
	}

	text := strings.TrimSpace(extractChatMessageText(chatResp.Choices[0].Message.Content))
	if text == "" {
		return Result{
			Warning: fmt.Sprintf("qwen3-asr-vllm returned empty text; raw=%s", truncateForWarning(strings.TrimSpace(string(data)), 240)),
		}, nil
	}
	return Result{Text: text}, nil
}

func (c Qwen3ASRVLLMClient) endpoint() string {
	value := strings.TrimSpace(c.Endpoint)
	if value == "" {
		return defaultQwen3ASRVLLMEndpoint
	}
	return value
}

func (c Qwen3ASRVLLMClient) model() string {
	value := strings.TrimSpace(c.Model)
	if value == "" {
		return "Qwen3-ASR"
	}
	return value
}

func (c Qwen3ASRVLLMClient) contentWithAudio(audioData string) []qwen3ASRChatContent {
	content := make([]qwen3ASRChatContent, 0, 2)
	if prompt := strings.TrimSpace(c.Prompt); prompt != "" {
		content = append(content, qwen3ASRChatContent{
			Type: "text",
			Text: prompt,
		})
	}
	content = append(content, qwen3ASRChatContent{
		Type: "input_audio",
		InputAudio: &qwen3ASRInputAudio{
			Data:   audioData,
			Format: "wav",
		},
	})
	return content
}

func resolveOptionalAPIKey(explicit, envName string) string {
	if value := strings.TrimSpace(explicit); value != "" {
		return value
	}
	keyEnv := strings.TrimSpace(envName)
	if keyEnv == "" {
		keyEnv = "OPENAI_API_KEY"
	}
	return strings.TrimSpace(os.Getenv(keyEnv))
}

func extractChatMessageText(content any) string {
	switch value := content.(type) {
	case string:
		return sanitizeQwen3ASRText(value)
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			switch entry := item.(type) {
			case string:
				if text := strings.TrimSpace(entry); text != "" {
					parts = append(parts, sanitizeQwen3ASRText(text))
				}
			case map[string]any:
				itemType, _ := entry["type"].(string)
				if text, ok := entry["text"].(string); ok && strings.TrimSpace(text) != "" {
					if itemType == "" || itemType == "text" {
						parts = append(parts, sanitizeQwen3ASRText(text))
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func sanitizeQwen3ASRText(text string) string {
	text = strings.TrimSpace(text)
	if idx := strings.LastIndex(text, "<asr_text>"); idx >= 0 {
		text = text[idx+len("<asr_text>"):]
	}
	return strings.TrimSpace(text)
}

type qwen3ASRChatRequest struct {
	Model    string                `json:"model"`
	Messages []qwen3ASRChatMessage `json:"messages"`
}

type qwen3ASRChatMessage struct {
	Role    string                `json:"role"`
	Content []qwen3ASRChatContent `json:"content"`
}

type qwen3ASRChatContent struct {
	Type       string              `json:"type"`
	Text       string              `json:"text,omitempty"`
	InputAudio *qwen3ASRInputAudio `json:"input_audio,omitempty"`
}

type qwen3ASRInputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

type qwen3ASRChatResponse struct {
	Choices []qwen3ASRChoice `json:"choices"`
}

type qwen3ASRChoice struct {
	Message qwen3ASRResponseMessage `json:"message"`
}

type qwen3ASRResponseMessage struct {
	Content any `json:"content"`
}
