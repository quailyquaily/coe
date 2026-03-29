package scene

import "coe/internal/i18n"

const (
	IDGeneral  = "general"
	IDTerminal = "terminal"
)

type Scene struct {
	ID          string
	DisplayKey  i18n.Key
	Aliases     []string
	Description string
}

func (s Scene) DisplayName(localizer i18n.Localizer) string {
	return localizer.Text(s.DisplayKey)
}
