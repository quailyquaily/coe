package scene

import "testing"

func TestStateDefaultsAndSwitch(t *testing.T) {
	t.Parallel()

	state, err := NewState(DefaultCatalog(), IDGeneral)
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	if got := state.Current().ID; got != IDGeneral {
		t.Fatalf("Current().ID = %q, want %q", got, IDGeneral)
	}

	changed, current, err := state.SwitchTo(IDTerminal)
	if err != nil {
		t.Fatalf("SwitchTo() error = %v", err)
	}
	if !changed {
		t.Fatal("SwitchTo() changed = false, want true")
	}
	if current.ID != IDTerminal {
		t.Fatalf("SwitchTo() scene = %q, want %q", current.ID, IDTerminal)
	}
}

func TestStateRejectsUnknownScene(t *testing.T) {
	t.Parallel()

	state, err := NewState(DefaultCatalog(), IDGeneral)
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	if _, _, err := state.SwitchTo("unknown"); err == nil {
		t.Fatal("SwitchTo() error = nil, want error")
	}
}
