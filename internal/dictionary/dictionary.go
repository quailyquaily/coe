package dictionary

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	defaultPromptMaxMappings = 100
	defaultPromptMaxChars    = 2000
)

type ConfigFile struct {
	Entries []Entry `yaml:"entries"`
}

type Entry struct {
	Canonical string   `yaml:"canonical"`
	Aliases   []string `yaml:"aliases"`
	Scenes    []string `yaml:"scenes"`
}

type CompiledEntry struct {
	Canonical string
	Aliases   []string
	Scenes    map[string]struct{}
}

type Dictionary struct {
	entries []CompiledEntry
}

func Load(path string) (*Dictionary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func Parse(data []byte) (*Dictionary, error) {
	var file ConfigFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse dictionary: %w", err)
	}

	compiled := make([]CompiledEntry, 0, len(file.Entries))
	for index, raw := range file.Entries {
		entry, ok, err := compileEntry(index, raw)
		if err != nil {
			return nil, err
		}
		if ok {
			compiled = append(compiled, entry)
		}
	}

	return &Dictionary{entries: compiled}, nil
}

func (d *Dictionary) Empty() bool {
	return d == nil || len(d.entries) == 0
}

func (d *Dictionary) EntriesForScene(sceneID string) []CompiledEntry {
	if d == nil || len(d.entries) == 0 {
		return nil
	}

	sceneID = strings.TrimSpace(sceneID)
	result := make([]CompiledEntry, 0, len(d.entries))
	for _, entry := range d.entries {
		if len(entry.Scenes) == 0 {
			result = append(result, entry)
			continue
		}
		if _, ok := entry.Scenes[sceneID]; ok {
			result = append(result, entry)
		}
	}
	return result
}

func (d *Dictionary) RenderPrompt(sceneID string) string {
	if d == nil {
		return ""
	}

	lines := make([]string, 0, defaultPromptMaxMappings)
	totalChars := 0
	for _, entry := range d.EntriesForScene(sceneID) {
		for _, alias := range entry.Aliases {
			if utf8.RuneCountInString(alias) <= 1 {
				continue
			}
			line := fmt.Sprintf(`- %q => %q`, alias, entry.Canonical)
			nextChars := totalChars + len(line)
			if len(lines) >= defaultPromptMaxMappings || nextChars > defaultPromptMaxChars {
				return strings.Join(lines, "\n")
			}
			lines = append(lines, line)
			totalChars = nextChars + 1
		}
	}

	return strings.Join(lines, "\n")
}

func (d *Dictionary) Normalize(sceneID, text string) string {
	if d == nil || strings.TrimSpace(text) == "" {
		return text
	}

	result := text
	replacements := flattenAliases(d.EntriesForScene(sceneID))
	multiCharPairs := make([]string, 0, len(replacements)*2)
	for _, replacement := range replacements {
		if replacement.singleRune {
			continue
		}
		multiCharPairs = append(multiCharPairs, replacement.alias, replacement.canonical)
	}
	if len(multiCharPairs) > 0 {
		result = strings.NewReplacer(multiCharPairs...).Replace(result)
	}
	for _, replacement := range replacements {
		if !replacement.singleRune {
			continue
		}
		result = replaceSingleRuneToken(result, replacement.alias, replacement.canonical)
	}
	return result
}

type aliasReplacement struct {
	alias      string
	canonical  string
	singleRune bool
	order      int
}

func flattenAliases(entries []CompiledEntry) []aliasReplacement {
	replacements := make([]aliasReplacement, 0, len(entries)*2)
	order := 0
	for _, entry := range entries {
		for _, alias := range entry.Aliases {
			replacements = append(replacements, aliasReplacement{
				alias:      alias,
				canonical:  entry.Canonical,
				singleRune: utf8.RuneCountInString(alias) == 1,
				order:      order,
			})
			order++
		}
	}

	slices.SortStableFunc(replacements, func(a, b aliasReplacement) int {
		alen := utf8.RuneCountInString(a.alias)
		blen := utf8.RuneCountInString(b.alias)
		if alen != blen {
			if alen > blen {
				return -1
			}
			return 1
		}
		if a.order < b.order {
			return -1
		}
		if a.order > b.order {
			return 1
		}
		return 0
	})

	return replacements
}

func compileEntry(index int, raw Entry) (CompiledEntry, bool, error) {
	canonical := strings.TrimSpace(raw.Canonical)
	if canonical == "" {
		return CompiledEntry{}, false, fmt.Errorf("dictionary entry %d has empty canonical", index)
	}

	aliases := make([]string, 0, len(raw.Aliases))
	seenAliases := make(map[string]struct{}, len(raw.Aliases))
	for _, alias := range raw.Aliases {
		alias = normalizeAlias(alias)
		if alias == "" {
			continue
		}
		if _, ok := seenAliases[alias]; ok {
			continue
		}
		seenAliases[alias] = struct{}{}
		aliases = append(aliases, alias)
	}
	if len(aliases) == 0 {
		return CompiledEntry{}, false, fmt.Errorf("dictionary entry %d has no aliases", index)
	}

	scenes := make(map[string]struct{}, len(raw.Scenes))
	for _, sceneID := range raw.Scenes {
		sceneID = strings.TrimSpace(sceneID)
		if sceneID == "" {
			continue
		}
		scenes[sceneID] = struct{}{}
	}

	return CompiledEntry{
		Canonical: canonical,
		Aliases:   aliases,
		Scenes:    scenes,
	}, true, nil
}

func normalizeAlias(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func replaceSingleRuneToken(text, alias, canonical string) string {
	runes := []rune(text)
	aliasRunes := []rune(alias)
	if len(aliasRunes) != 1 {
		return text
	}

	var builder strings.Builder
	for index := 0; index < len(runes); index++ {
		if runes[index] == aliasRunes[0] && tokenBoundary(runes, index-1) && tokenBoundary(runes, index+1) {
			builder.WriteString(canonical)
			continue
		}
		builder.WriteRune(runes[index])
	}
	return builder.String()
}

func tokenBoundary(runes []rune, index int) bool {
	if index < 0 || index >= len(runes) {
		return true
	}
	r := runes[index]
	if unicode.IsSpace(r) {
		return true
	}
	if unicode.IsPunct(r) || unicode.IsSymbol(r) {
		return true
	}
	return false
}
