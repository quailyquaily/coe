package scene

import "strings"

var switchPrefixes = []string{
	"切换场景",
	"切换到",
	"切到场景",
	"切到",
	"进入场景",
	"进入终端",
	"switch scene",
	"set scene",
	"change scene",
	"シーン切替",
	"シーン切り替え",
	"シーンを",
}

func LooksLikeSwitchCommand(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}

	lowered := strings.ToLower(trimmed)
	for _, prefix := range switchPrefixes {
		if strings.HasPrefix(trimmed, prefix) || strings.HasPrefix(lowered, prefix) {
			return true
		}
	}

	return false
}
