package dictionary

import "testing"

func TestParseAndFilterByScene(t *testing.T) {
	t.Parallel()

	dict, err := Parse([]byte(`
entries:
  - canonical: "Coe"
    aliases: ["扣一", "口诶", "coe"]
  - canonical: "systemctl"
    aliases: ["system control", "system c t l"]
    scenes: ["terminal"]
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	general := dict.EntriesForScene("general")
	if len(general) != 1 {
		t.Fatalf("len(general) = %d, want 1", len(general))
	}
	terminal := dict.EntriesForScene("terminal")
	if len(terminal) != 2 {
		t.Fatalf("len(terminal) = %d, want 2", len(terminal))
	}
}

func TestRenderPromptSkipsSingleRuneAliases(t *testing.T) {
	t.Parallel()

	dict, err := Parse([]byte(`
entries:
  - canonical: "-d"
    aliases: ["d"]
    scenes: ["terminal"]
  - canonical: "systemctl"
    aliases: ["system control"]
    scenes: ["terminal"]
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := dict.RenderPrompt("terminal")
	if got != "- \"system control\" => \"systemctl\"" {
		t.Fatalf("RenderPrompt() = %q", got)
	}
}

func TestNormalizeAppliesLongestAliasFirst(t *testing.T) {
	t.Parallel()

	dict, err := Parse([]byte(`
entries:
  - canonical: "systemctl"
    aliases: ["system", "system control"]
    scenes: ["terminal"]
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := dict.Normalize("terminal", "please run system control now")
	if got != "please run systemctl now" {
		t.Fatalf("Normalize() = %q", got)
	}
}

func TestNormalizeSingleRuneAliasUsesTokenBoundaries(t *testing.T) {
	t.Parallel()

	dict, err := Parse([]byte(`
entries:
  - canonical: "-d"
    aliases: ["d"]
    scenes: ["terminal"]
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if got := dict.Normalize("terminal", "run d now"); got != "run -d now" {
		t.Fatalf("Normalize() = %q", got)
	}
	if got := dict.Normalize("terminal", "docker"); got != "docker" {
		t.Fatalf("Normalize() should keep embedded characters, got %q", got)
	}
}
