package output

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

const envPortalStatePath = "COE_STATE_PATH"

type PortalAccess struct {
	RemoteDesktopRestoreToken string `json:"remote_desktop_restore_token"`
}

type PortalStateStore struct {
	path string
	mu   sync.Mutex
}

func ResolvePortalStatePath() (string, error) {
	if path := os.Getenv(envPortalStatePath); path != "" {
		return path, nil
	}

	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "coe", "state.json"), nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "coe", "state.json"), nil
}

func NewPortalStateStore(path string) *PortalStateStore {
	return &PortalStateStore{path: path}
}

func (s *PortalStateStore) Load() (PortalAccess, error) {
	if s == nil {
		return PortalAccess{}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return PortalAccess{}, nil
	}
	if err != nil {
		return PortalAccess{}, err
	}

	var state PortalAccess
	if err := json.Unmarshal(data, &state); err != nil {
		return PortalAccess{}, err
	}
	return state, nil
}

func (s *PortalStateStore) Save(state PortalAccess) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, append(data, '\n'), 0o644)
}
