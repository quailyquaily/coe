package scene

import "coe/internal/i18n"

type DisplayScene struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Current     bool   `json:"current"`
}

func ListDisplayScenes(state *State, localizer i18n.Localizer) []DisplayScene {
	if state == nil {
		return nil
	}

	currentID := state.Current().ID
	scenes := state.List()
	result := make([]DisplayScene, 0, len(scenes))
	for _, item := range scenes {
		result = append(result, DisplayScene{
			ID:          item.ID,
			DisplayName: item.DisplayName(localizer),
			Current:     item.ID == currentID,
		})
	}
	return result
}
