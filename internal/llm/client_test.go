package llm

import (
	"testing"
	"time"

	"coe/internal/config"
)

func TestNewCorrectorAppliesTimeout(t *testing.T) {
	t.Parallel()

	corrector, err := NewCorrector(config.LLMConfig{Provider: "openai", TimeoutSeconds: 9})
	if err != nil {
		t.Fatalf("NewCorrector() error = %v", err)
	}
	openAICorrector, ok := corrector.(OpenAICorrector)
	if !ok {
		t.Fatalf("corrector type = %T", corrector)
	}
	if openAICorrector.Timeout != 9*time.Second {
		t.Fatalf("Timeout = %v", openAICorrector.Timeout)
	}
	if openAICorrector.HTTPClient == nil {
		t.Fatal("HTTPClient is nil")
	}
	if openAICorrector.HTTPClient.Timeout != 9*time.Second {
		t.Fatalf("HTTPClient.Timeout = %v", openAICorrector.HTTPClient.Timeout)
	}
}

func TestResolveTimeoutFallback(t *testing.T) {
	t.Parallel()

	if got := resolveTimeout(0, 45); got != 45*time.Second {
		t.Fatalf("resolveTimeout(0,45) = %v", got)
	}
	if got := resolveTimeout(-3, 45); got != 45*time.Second {
		t.Fatalf("resolveTimeout(-3,45) = %v", got)
	}
}
