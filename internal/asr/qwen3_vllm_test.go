package asr

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"coe/internal/audio"
)

func TestQwen3ASRVLLMClientTranscribe(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization header = %q", got)
		}

		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		var payload qwen3ASRChatRequest
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if payload.Model != "Qwen3-ASR" {
			t.Fatalf("model = %q", payload.Model)
		}
		if len(payload.Messages) != 1 {
			t.Fatalf("messages len = %d", len(payload.Messages))
		}
		if payload.Messages[0].Role != "user" {
			t.Fatalf("messages[0].role = %q", payload.Messages[0].Role)
		}
		if len(payload.Messages[0].Content) != 2 {
			t.Fatalf("content len = %d", len(payload.Messages[0].Content))
		}
		if payload.Messages[0].Content[0].Type != "text" || payload.Messages[0].Content[0].Text != "transcribe this audio" {
			t.Fatalf("prompt content = %#v", payload.Messages[0].Content[0])
		}
		audioItem := payload.Messages[0].Content[1]
		if audioItem.Type != "input_audio" || audioItem.InputAudio == nil {
			t.Fatalf("audio content = %#v", audioItem)
		}
		if audioItem.InputAudio.Format != "wav" {
			t.Fatalf("audio format = %q", audioItem.InputAudio.Format)
		}
		rawAudio, err := base64.StdEncoding.DecodeString(audioItem.InputAudio.Data)
		if err != nil {
			t.Fatalf("DecodeString() error = %v", err)
		}
		if string(rawAudio[:4]) != "RIFF" || string(rawAudio[8:12]) != "WAVE" {
			t.Fatalf("expected WAV payload, got %q / %q", string(rawAudio[:4]), string(rawAudio[8:12]))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":" 你好，世界 "}}]}`))
	}))
	defer server.Close()

	client := Qwen3ASRVLLMClient{
		Endpoint:   server.URL,
		Model:      "Qwen3-ASR",
		APIKey:     "test-key",
		Prompt:     "transcribe this audio",
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
	if result.Text != "你好，世界" {
		t.Fatalf("result.Text = %q", result.Text)
	}
}

func TestQwen3ASRVLLMClientTranscribeArrayContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":[{"type":"text","text":"first line"},{"type":"text","text":"second line"}]}}]}`))
	}))
	defer server.Close()

	client := Qwen3ASRVLLMClient{
		Endpoint:   server.URL,
		HTTPClient: server.Client(),
	}

	result, err := client.Transcribe(context.Background(), audio.Result{
		Data:       []byte{0x01, 0x02},
		ByteCount:  2,
		SampleRate: 16000,
		Channels:   1,
		Format:     "s16",
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Text != "first line\nsecond line" {
		t.Fatalf("result.Text = %q", result.Text)
	}
}

func TestExtractChatMessageTextStripsASRPrefixMarkup(t *testing.T) {
	t.Parallel()

	got := extractChatMessageText("language Chinese<asr_text>竟然成功了！我去。")
	if got != "竟然成功了！我去。" {
		t.Fatalf("extractChatMessageText() = %q", got)
	}
}

func TestQwen3ASRVLLMClientWarnsOnEmptyChoices(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	client := Qwen3ASRVLLMClient{
		Endpoint:   server.URL,
		HTTPClient: server.Client(),
	}

	result, err := client.Transcribe(context.Background(), audio.Result{
		Data:       []byte{0x01, 0x02},
		ByteCount:  2,
		SampleRate: 16000,
		Channels:   1,
		Format:     "s16",
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Warning == "" {
		t.Fatal("expected warning for empty choices")
	}
}
