package notify

import "testing"

func TestBuildHintsIncludesTransientWhenRequested(t *testing.T) {
	t.Parallel()

	hints := buildHints(Message{
		Urgency:   UrgencyNormal,
		Transient: true,
	})

	if got, ok := hints["transient"]; !ok {
		t.Fatal("expected transient hint to be present")
	} else if value, ok := got.Value().(bool); !ok || !value {
		t.Fatalf("expected transient hint to be true, got %#v", got.Value())
	}
}

func TestBuildHintsOmitsTransientByDefault(t *testing.T) {
	t.Parallel()

	hints := buildHints(Message{
		Urgency: UrgencyLow,
	})

	if _, ok := hints["transient"]; ok {
		t.Fatal("expected transient hint to be omitted by default")
	}
}

func TestShouldCloseAfterTimeout(t *testing.T) {
	t.Parallel()

	if !shouldCloseAfterTimeout(Message{
		Urgency:   UrgencyNormal,
		Timeout:   3,
		Transient: true,
	}) {
		t.Fatal("expected transient non-critical notification to be auto-closed")
	}
}

func TestShouldNotCloseCriticalNotificationAfterTimeout(t *testing.T) {
	t.Parallel()

	if shouldCloseAfterTimeout(Message{
		Urgency:   UrgencyCritical,
		Timeout:   3,
		Transient: true,
	}) {
		t.Fatal("expected critical notification to stay visible")
	}
}

func TestShouldNotClosePersistentNotification(t *testing.T) {
	t.Parallel()

	if shouldCloseAfterTimeout(Message{
		Urgency: UrgencyNormal,
		Timeout: 3,
	}) {
		t.Fatal("expected non-transient notification to stay visible")
	}
}
