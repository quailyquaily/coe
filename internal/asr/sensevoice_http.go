package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"coe/internal/audio"
)

const defaultSenseVoiceEndpoint = "http://127.0.0.1:50000/api/v1/asr"

type SenseVoiceHTTPClient struct {
	Endpoint   string
	Language   string
	HTTPClient *http.Client
}

func (c SenseVoiceHTTPClient) Name() string {
	return "sensevoice-http"
}

func (c SenseVoiceHTTPClient) Transcribe(ctx context.Context, capture audio.Result) (Result, error) {
	wav, err := audio.EncodeWAV(capture)
	if err != nil {
		return Result{}, err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	fileWriter, err := writer.CreateFormFile("files", "capture.wav")
	if err != nil {
		return Result{}, err
	}
	if _, err := fileWriter.Write(wav); err != nil {
		return Result{}, err
	}

	language := strings.TrimSpace(c.Language)
	if language == "" {
		language = "auto"
	}
	if err := writer.WriteField("lang", language); err != nil {
		return Result{}, err
	}
	if err := writer.Close(); err != nil {
		return Result{}, err
	}

	endpoint, err := resolveSenseVoiceEndpoint(c.Endpoint)
	if err != nil {
		return Result{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

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
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = resp.Status
		}
		return Result{}, fmt.Errorf("sensevoice transcription failed at %s: %s", endpoint, truncateForWarning(message, 240))
	}

	var payload senseVoiceResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return Result{}, err
	}

	if len(payload.Result) == 0 {
		return Result{Warning: "SenseVoice returned no result entries"}, nil
	}

	entry := payload.Result[0]
	text := strings.TrimSpace(entry.Text)
	if text == "" {
		text = strings.TrimSpace(entry.CleanText)
	}
	if text == "" {
		text = strings.TrimSpace(entry.RawText)
	}
	if text == "" {
		return Result{
			Warning: fmt.Sprintf("SenseVoice returned empty text; raw=%s", truncateForWarning(strings.TrimSpace(string(data)), 240)),
		}, nil
	}

	return Result{Text: text}, nil
}

type senseVoiceResponse struct {
	Result []senseVoiceEntry `json:"result"`
}

type senseVoiceEntry struct {
	Text      string `json:"text"`
	CleanText string `json:"clean_text"`
	RawText   string `json:"raw_text"`
}

func resolveSenseVoiceEndpoint(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return defaultSenseVoiceEndpoint, nil
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid SenseVoice endpoint %q: %w", value, err)
	}

	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/api/v1/asr"
		return parsed.String(), nil
	}
	if parsed.Path == "/api/v1" || parsed.Path == "/api/v1/" {
		parsed.Path = "/api/v1/asr"
		return parsed.String(), nil
	}
	if strings.HasSuffix(parsed.Path, "/") && path.Clean(parsed.Path) == "/api/v1/asr" {
		parsed.Path = "/api/v1/asr"
		return parsed.String(), nil
	}

	return parsed.String(), nil
}
