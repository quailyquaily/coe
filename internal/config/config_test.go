package config

import (
	"path/filepath"
	"testing"
)

func TestWriteDefaultAndLoad(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")

	written, err := WriteDefault(path, false)
	if err != nil {
		t.Fatalf("WriteDefault() error = %v", err)
	}
	if !written {
		t.Fatal("WriteDefault() reported not written")
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Runtime.TargetDesktop != "gnome" {
		t.Fatalf("unexpected target desktop %q", cfg.Runtime.TargetDesktop)
	}
	if cfg.Audio.RecorderBinary != "pw-record" {
		t.Fatalf("unexpected recorder %q", cfg.Audio.RecorderBinary)
	}
}
