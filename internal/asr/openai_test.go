package asr

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"coe/internal/audio"
)

func TestOpenAIClientTranscribe(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization header = %q", got)
		}

		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader() error = %v", err)
		}

		fields := map[string]string{}
		var fileData []byte

		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart() error = %v", err)
			}

			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}

			if part.FormName() == "file" {
				fileData = data
				continue
			}
			fields[part.FormName()] = string(data)
		}

		if fields["model"] != "gpt-4o-mini-transcribe" {
			t.Fatalf("model = %q", fields["model"])
		}
		if fields["language"] != "zh" {
			t.Fatalf("language = %q", fields["language"])
		}
		if fields["response_format"] != "json" {
			t.Fatalf("response_format = %q", fields["response_format"])
		}
		if string(fileData[:4]) != "RIFF" || string(fileData[8:12]) != "WAVE" {
			t.Fatalf("expected WAV upload, got %q / %q", string(fileData[:4]), string(fileData[8:12]))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"  hello world  "}`))
	}))
	defer server.Close()

	client := OpenAIClient{
		Endpoint:   server.URL,
		Model:      "gpt-4o-mini-transcribe",
		APIKeyEnv:  "OPENAI_API_KEY",
		Language:   "zh",
		HTTPClient: server.Client(),
	}

	result, err := client.Transcribe(context.Background(), audio.Result{
		Data:       []byte{0x01, 0x02, 0x03, 0x04},
		ByteCount:  4,
		SampleRate: 16000,
		Channels:   1,
		Format:     "s16",
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Text != "hello world" {
		t.Fatalf("result.Text = %q", result.Text)
	}
}

func TestOpenAIClientTranscribeRetriesWithoutLanguageHintOnEmptyText(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++

		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader() error = %v", err)
		}

		fields := map[string]string{}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart() error = %v", err)
			}

			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if part.FormName() == "file" {
				continue
			}
			fields[part.FormName()] = string(data)
		}

		w.Header().Set("Content-Type", "application/json")
		switch requests {
		case 1:
			if fields["language"] != "zh" {
				t.Fatalf("first request language = %q", fields["language"])
			}
			_, _ = w.Write([]byte(`{"text":"","language":"zh"}`))
		case 2:
			if _, ok := fields["language"]; ok {
				t.Fatalf("second request should omit language, got %q", fields["language"])
			}
			_, _ = w.Write([]byte(`{"text":"hello retry"}`))
		default:
			t.Fatalf("unexpected request count %d", requests)
		}
	}))
	defer server.Close()

	client := OpenAIClient{
		Endpoint:   server.URL,
		Model:      "gpt-4o-mini-transcribe",
		APIKeyEnv:  "OPENAI_API_KEY",
		Language:   "zh",
		HTTPClient: server.Client(),
	}

	result, err := client.Transcribe(context.Background(), audio.Result{
		Data:       []byte{0x01, 0x02, 0x03, 0x04},
		ByteCount:  4,
		SampleRate: 16000,
		Channels:   1,
		Format:     "s16",
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Text != "hello retry" {
		t.Fatalf("result.Text = %q", result.Text)
	}
	if !strings.Contains(result.Warning, "retry without language hint succeeded") {
		t.Fatalf("result.Warning = %q", result.Warning)
	}
}

func TestOpenAIClientTranscribeReturnsWarningForEmptyText(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"","language":"zh","duration":8.19}`))
	}))
	defer server.Close()

	client := OpenAIClient{
		Endpoint:   server.URL,
		Model:      "gpt-4o-mini-transcribe",
		APIKeyEnv:  "OPENAI_API_KEY",
		Language:   "zh",
		HTTPClient: server.Client(),
	}

	result, err := client.Transcribe(context.Background(), audio.Result{
		Data:       []byte{0x01, 0x02, 0x03, 0x04},
		ByteCount:  4,
		SampleRate: 16000,
		Channels:   1,
		Format:     "s16",
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Text != "" {
		t.Fatalf("result.Text = %q, want empty", result.Text)
	}
	if !strings.Contains(result.Warning, "OpenAI transcription returned empty text") {
		t.Fatalf("result.Warning = %q", result.Warning)
	}
}

func TestOpenAIClientMissingAPIKey(t *testing.T) {
	if err := os.Unsetenv("OPENAI_API_KEY"); err != nil {
		t.Fatalf("Unsetenv() error = %v", err)
	}

	client := OpenAIClient{}
	_, err := client.Transcribe(context.Background(), audio.Result{
		Data:       []byte{0x01, 0x02},
		ByteCount:  2,
		SampleRate: 16000,
		Channels:   1,
		Format:     "s16",
	})
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("expected missing key error, got %v", err)
	}
}
