package scene

import (
	"fmt"
	"strings"
	"sync"
)

type State struct {
	mu        sync.RWMutex
	catalog   Catalog
	currentID string
}

func NewState(catalog Catalog, initialID string) (*State, error) {
	initialID = strings.TrimSpace(initialID)
	if initialID == "" {
		initialID = IDGeneral
	}
	if _, ok := catalog.ByID(initialID); !ok {
		return nil, fmt.Errorf("unknown scene %q", initialID)
	}

	return &State{
		catalog:   catalog,
		currentID: initialID,
	}, nil
}

func (s *State) Current() Scene {
	s.mu.RLock()
	defer s.mu.RUnlock()

	scene, _ := s.catalog.ByID(s.currentID)
	return scene
}

func (s *State) SwitchTo(id string) (bool, Scene, error) {
	id = strings.TrimSpace(id)
	scene, ok := s.catalog.ByID(id)
	if !ok {
		return false, Scene{}, fmt.Errorf("unknown scene %q", id)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	changed := s.currentID != scene.ID
	s.currentID = scene.ID
	return changed, scene, nil
}

func (s *State) List() []Scene {
	return s.catalog.List()
}

func (s *State) SceneByID(id string) (Scene, bool) {
	return s.catalog.ByID(id)
}
