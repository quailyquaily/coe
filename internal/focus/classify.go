package focus

import "strings"

func LooksLikeTerminal(target Target) bool {
	joined := strings.ToLower(strings.Join([]string{
		target.AppID,
		target.WMClass,
		target.Title,
	}, " "))

	for _, needle := range []string{
		"ptyxis",
		"kgx",
		"gnome-console",
		"org.gnome.console",
		"gnome-terminal",
		"org.gnome.terminal",
		"konsole",
		"xfce4-terminal",
		"tilix",
		"warp",
		"wezterm",
		"alacritty",
		"kitty",
		"foot",
		"ghostty",
		"rio",
		"tabby",
		"hyper",
		"terminal",
		"codex",
	} {
		if strings.Contains(joined, needle) {
			return true
		}
	}

	return false
}
