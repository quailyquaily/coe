package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAICorrectorCorrect(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization header = %q", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		if payload["model"] != "gpt-4o-mini" {
			t.Fatalf("model = %v", payload["model"])
		}
		if payload["input"] != "hello,,world" {
			t.Fatalf("input = %v", payload["input"])
		}
		if payload["instructions"] == "" {
			t.Fatal("expected instructions")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"Hello, world."}`))
	}))
	defer server.Close()

	corrector := OpenAICorrector{
		Endpoint:     server.URL,
		EndpointType: "responses",
		Model:        "gpt-4o-mini",
		APIKey:       "test-key",
		APIKeyEnv:    "OPENAI_API_KEY",
		HTTPClient:   server.Client(),
	}

	result, err := corrector.Correct(context.Background(), "hello,,world")
	if err != nil {
		t.Fatalf("Correct() error = %v", err)
	}
	if result.Text != "Hello, world." {
		t.Fatalf("result.Text = %q", result.Text)
	}
}

func TestOpenAICorrectorRendersPromptTemplate(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload["instructions"] != "Fix text with gpt-4o-mini via responses" {
			t.Fatalf("instructions = %v", payload["instructions"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"ok"}`))
	}))
	defer server.Close()

	corrector := OpenAICorrector{
		Endpoint:     server.URL,
		EndpointType: "responses",
		Model:        "gpt-4o-mini",
		APIKey:       "test-key",
		PromptFile:   filepath.Join("testdata", "correction-prompt.tmpl"),
		HTTPClient:   server.Client(),
	}

	result, err := corrector.Correct(context.Background(), "hello,,world")
	if err != nil {
		t.Fatalf("Correct() error = %v", err)
	}
	if result.Text != "ok" {
		t.Fatalf("result.Text = %q", result.Text)
	}
}

func TestOpenAICorrectorCorrectFromOutputArray(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output": [
				{
					"type": "message",
					"content": [
						{"type":"output_text","text":"Hello, world."}
					]
				}
			]
		}`))
	}))
	defer server.Close()

	corrector := OpenAICorrector{
		Endpoint:     server.URL,
		EndpointType: "responses",
		Model:        "gpt-4o-mini",
		APIKey:       "test-key",
		APIKeyEnv:    "OPENAI_API_KEY",
		HTTPClient:   server.Client(),
	}

	result, err := corrector.Correct(context.Background(), "hello,,world")
	if err != nil {
		t.Fatalf("Correct() error = %v", err)
	}
	if result.Text != "Hello, world." {
		t.Fatalf("result.Text = %q", result.Text)
	}
}

func TestOpenAICorrectorMissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	corrector := OpenAICorrector{}
	_, err := corrector.Correct(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("expected missing key error, got %v", err)
	}
}

func TestOpenAICorrectorChatEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer direct-key" {
			t.Fatalf("Authorization header = %q", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		if payload["model"] != "gpt-4o-mini" {
			t.Fatalf("model = %v", payload["model"])
		}

		messages, ok := payload["messages"].([]any)
		if !ok || len(messages) != 2 {
			t.Fatalf("messages = %#v", payload["messages"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-test",
			"object":"chat.completion",
			"choices":[
				{
					"index":0,
					"message":{"role":"assistant","content":"Hello, world."},
					"finish_reason":"stop"
				}
			],
			"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
		}`))
	}))
	defer server.Close()

	corrector := OpenAICorrector{
		Endpoint:     server.URL + "/v1",
		EndpointType: "chat",
		Model:        "gpt-4o-mini",
		APIKey:       "direct-key",
	}

	result, err := corrector.Correct(context.Background(), "hello,,world")
	if err != nil {
		t.Fatalf("Correct() error = %v", err)
	}
	if result.Text != "Hello, world." {
		t.Fatalf("result.Text = %q", result.Text)
	}
}
