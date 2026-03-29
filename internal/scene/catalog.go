package scene

import (
	"strings"

	"coe/internal/i18n"
)

type Catalog struct {
	ordered []Scene
	byID    map[string]Scene
}

func DefaultCatalog() Catalog {
	ordered := []Scene{
		{
			ID:         IDGeneral,
			DisplayKey: i18n.SceneGeneralName,
			Aliases: []string{
				"general",
				"default",
				"normal",
				"通用",
				"普通",
				"默认",
				"一般",
				"通常",
			},
			Description: "General dictation and writing.",
		},
		{
			ID:         IDTerminal,
			DisplayKey: i18n.SceneTerminalName,
			Aliases: []string{
				"terminal",
				"shell",
				"command line",
				"command-line",
				"终端",
				"命令行",
				"shell 模式",
				"ターミナル",
				"シェル",
				"コマンドライン",
			},
			Description: "Linux commands, shell fragments, paths, flags, package names, and filenames.",
		},
	}

	byID := make(map[string]Scene, len(ordered))
	for _, item := range ordered {
		byID[item.ID] = item
	}

	return Catalog{
		ordered: ordered,
		byID:    byID,
	}
}

func (c Catalog) List() []Scene {
	result := make([]Scene, len(c.ordered))
	copy(result, c.ordered)
	return result
}

func (c Catalog) ByID(id string) (Scene, bool) {
	scene, ok := c.byID[strings.TrimSpace(id)]
	return scene, ok
}
