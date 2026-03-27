package asr

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"coe/internal/audio"
)

func TestSenseVoiceHTTPClientTranscribe(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
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

			if part.FormName() == "files" {
				fileData = data
				continue
			}
			fields[part.FormName()] = string(data)
		}

		if fields["lang"] != "zh" {
			t.Fatalf("lang = %q", fields["lang"])
		}
		if string(fileData[:4]) != "RIFF" || string(fileData[8:12]) != "WAVE" {
			t.Fatalf("expected WAV upload, got %q / %q", string(fileData[:4]), string(fileData[8:12]))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[{"text":" 你好，世界。 ","clean_text":"你好世界","raw_text":"<|zh|>你好，世界。"}]}`))
	}))
	defer server.Close()

	client := SenseVoiceHTTPClient{
		Endpoint:   server.URL,
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
	if result.Text != "你好，世界。" {
		t.Fatalf("result.Text = %q", result.Text)
	}
}

func TestSenseVoiceHTTPClientDefaultsLanguageToAuto(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			if part.FormName() == "files" {
				continue
			}
			fields[part.FormName()] = string(data)
		}

		if fields["lang"] != "auto" {
			t.Fatalf("lang = %q", fields["lang"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[{"clean_text":"hello"}]}`))
	}))
	defer server.Close()

	client := SenseVoiceHTTPClient{
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
	if result.Text != "hello" {
		t.Fatalf("result.Text = %q", result.Text)
	}
}

func TestSenseVoiceHTTPClientWarnsOnEmptyResult(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[]}`))
	}))
	defer server.Close()

	client := SenseVoiceHTTPClient{
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
		t.Fatal("expected warning for empty result")
	}
}

func TestResolveSenseVoiceEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "default", input: "", expected: defaultSenseVoiceEndpoint},
		{name: "base url", input: "http://127.0.0.1:50000", expected: defaultSenseVoiceEndpoint},
		{name: "base url slash", input: "http://127.0.0.1:50000/", expected: defaultSenseVoiceEndpoint},
		{name: "api v1", input: "http://127.0.0.1:50000/api/v1", expected: defaultSenseVoiceEndpoint},
		{name: "full path", input: defaultSenseVoiceEndpoint, expected: defaultSenseVoiceEndpoint},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveSenseVoiceEndpoint(tt.input)
			if err != nil {
				t.Fatalf("resolveSenseVoiceEndpoint() error = %v", err)
			}
			if got != tt.expected {
				t.Fatalf("resolveSenseVoiceEndpoint() = %q, want %q", got, tt.expected)
			}
		})
	}
}
