package scene

import (
	"encoding/json"
	"errors"
	"strings"

	"coe/internal/i18n"
)

type RouteScene struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Aliases     []string `json:"aliases"`
	Description string   `json:"description"`
}

type RouteRequest struct {
	CurrentScene    string       `json:"current_scene"`
	AvailableScenes []RouteScene `json:"available_scenes"`
	Utterance       string       `json:"utterance"`
}

type RouteResponse struct {
	Intent       string `json:"intent"`
	TargetScene  string `json:"target_scene"`
	MatchedAlias string `json:"matched_alias,omitempty"`
}

func BuildRouteInput(current Scene, scenes []Scene, localizer i18n.Localizer, utterance string) (string, error) {
	request := RouteRequest{
		CurrentScene: current.ID,
		Utterance:    strings.TrimSpace(utterance),
	}
	request.AvailableScenes = make([]RouteScene, 0, len(scenes))
	for _, candidate := range scenes {
		request.AvailableScenes = append(request.AvailableScenes, RouteScene{
			ID:          candidate.ID,
			DisplayName: candidate.DisplayName(localizer),
			Aliases:     append([]string(nil), candidate.Aliases...),
			Description: candidate.Description,
		})
	}

	body, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func ParseRouteOutput(text string, state *State) (RouteResponse, Scene, error) {
	if state == nil {
		return RouteResponse{}, Scene{}, errors.New("scene state is not configured")
	}

	raw := strings.TrimSpace(text)
	if raw == "" {
		return RouteResponse{}, Scene{}, errors.New("scene router returned empty response")
	}
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}
	start := strings.IndexByte(raw, '{')
	end := strings.LastIndexByte(raw, '}')
	if start >= 0 && end >= start {
		raw = raw[start : end+1]
	}

	var response RouteResponse
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		return RouteResponse{}, Scene{}, err
	}
	if strings.TrimSpace(response.Intent) != "switch_scene" {
		return RouteResponse{}, Scene{}, errors.New("scene router did not return switch_scene")
	}

	scene, ok := state.SceneByID(response.TargetScene)
	if !ok {
		return RouteResponse{}, Scene{}, errors.New("scene router returned an unknown target scene")
	}
	return response, scene, nil
}
