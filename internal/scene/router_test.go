package scene

import (
	"strings"
	"testing"

	"coe/internal/i18n"
)

func TestBuildRouteInputIncludesScenes(t *testing.T) {
	t.Parallel()

	state, err := NewState(DefaultCatalog(), IDGeneral)
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}

	got, err := BuildRouteInput(state.Current(), state.List(), i18n.NewForLocale("zh_CN.UTF-8"), "切换场景到终端")
	if err != nil {
		t.Fatalf("BuildRouteInput() error = %v", err)
	}
	for _, fragment := range []string{
		`"current_scene":"general"`,
		`"终端"`,
		`"切换场景到终端"`,
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("BuildRouteInput() missing %q in %q", fragment, got)
		}
	}
}

func TestParseRouteOutput(t *testing.T) {
	t.Parallel()

	state, err := NewState(DefaultCatalog(), IDGeneral)
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}

	response, target, err := ParseRouteOutput("```json\n{\"intent\":\"switch_scene\",\"target_scene\":\"terminal\",\"matched_alias\":\"终端\"}\n```", state)
	if err != nil {
		t.Fatalf("ParseRouteOutput() error = %v", err)
	}
	if response.MatchedAlias != "终端" {
		t.Fatalf("MatchedAlias = %q, want %q", response.MatchedAlias, "终端")
	}
	if target.ID != IDTerminal {
		t.Fatalf("target.ID = %q, want %q", target.ID, IDTerminal)
	}
}
