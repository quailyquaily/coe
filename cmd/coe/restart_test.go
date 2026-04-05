package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"coe/internal/config"
	dbusipc "coe/internal/ipc/dbus"
)

func TestRunRestart(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		cfg       config.Config
		cfgErr    error
		waitErr   error
		runErrAt  int
		runErr    error
		wantErr   string
		wantOut   string
		wantCalls [][]string
	}{
		{
			name: "desktop mode restarts service only",
			cfg: config.Config{
				Runtime: config.RuntimeConfig{Mode: config.RuntimeModeDesktop},
			},
			wantOut: "restarted coe.service\n",
			wantCalls: [][]string{
				{"systemctl", "--user", "restart", "coe.service"},
			},
		},
		{
			name: "fcitx mode restarts service and fcitx",
			cfg: config.Config{
				Runtime: config.RuntimeConfig{Mode: config.RuntimeModeFcitx},
			},
			wantOut: "restarted coe.service\nrestarted fcitx5\n",
			wantCalls: [][]string{
				{"systemctl", "--user", "restart", "coe.service"},
				{"fcitx5", "-rd"},
			},
		},
		{
			name: "service restart error surfaces",
			cfg: config.Config{
				Runtime: config.RuntimeConfig{Mode: config.RuntimeModeDesktop},
			},
			runErrAt: 1,
			runErr:   errors.New("systemctl failed"),
			wantErr:  "restart coe.service: systemctl failed",
			wantCalls: [][]string{
				{"systemctl", "--user", "restart", "coe.service"},
			},
		},
		{
			name: "fcitx restart error surfaces after service restart",
			cfg: config.Config{
				Runtime: config.RuntimeConfig{Mode: config.RuntimeModeFcitx},
			},
			runErrAt: 2,
			runErr:   errors.New("fcitx restart failed"),
			wantErr:  "restart fcitx5: fcitx restart failed",
			wantOut:  "restarted coe.service\n",
			wantCalls: [][]string{
				{"systemctl", "--user", "restart", "coe.service"},
				{"fcitx5", "-rd"},
			},
		},
		{
			name: "fcitx restart waits for daemon readiness",
			cfg: config.Config{
				Runtime: config.RuntimeConfig{Mode: config.RuntimeModeFcitx},
			},
			waitErr: errors.New("name has no owner"),
			wantErr: "wait for coe daemon ready before restarting fcitx5: name has no owner",
			wantOut: "restarted coe.service\n",
			wantCalls: [][]string{
				{"systemctl", "--user", "restart", "coe.service"},
			},
		},
		{
			name:    "config load error surfaces before restart",
			cfgErr:  errors.New("load config for restart: bad yaml"),
			wantErr: "load config for restart: bad yaml",
		},
		{
			name:    "rejects extra args",
			args:    []string{"now"},
			wantErr: "usage: coe restart",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			originalLoadConfig := loadRestartConfig
			originalRun := runRestartCommand
			originalWait := waitRestartReady
			defer func() {
				loadRestartConfig = originalLoadConfig
				runRestartCommand = originalRun
				waitRestartReady = originalWait
			}()

			loadRestartConfig = func() (config.Config, error) {
				if tt.cfgErr != nil {
					return config.Config{}, tt.cfgErr
				}
				return tt.cfg, nil
			}
			waitRestartReady = func(context.Context) error {
				return tt.waitErr
			}

			var gotCalls [][]string
			runRestartCommand = func(_ context.Context, name string, args ...string) error {
				call := append([]string{name}, args...)
				gotCalls = append(gotCalls, call)
				if tt.runErrAt == len(gotCalls) {
					return tt.runErr
				}
				return nil
			}

			output := captureStdout(t, func() error {
				return runRestart(context.Background(), tt.args)
			})

			if len(gotCalls) != len(tt.wantCalls) {
				t.Fatalf("calls = %#v, want %#v", gotCalls, tt.wantCalls)
			}
			for i := range gotCalls {
				if len(gotCalls[i]) != len(tt.wantCalls[i]) {
					t.Fatalf("calls[%d] = %#v, want %#v", i, gotCalls[i], tt.wantCalls[i])
				}
				for j := range gotCalls[i] {
					if gotCalls[i][j] != tt.wantCalls[i][j] {
						t.Fatalf("calls[%d][%d] = %q, want %q", i, j, gotCalls[i][j], tt.wantCalls[i][j])
					}
				}
			}
			if output.out != tt.wantOut {
				t.Fatalf("stdout = %q, want %q", output.out, tt.wantOut)
			}
			if tt.wantErr == "" {
				if output.err != nil {
					t.Fatalf("runRestart() error = %v", output.err)
				}
				return
			}
			if output.err == nil || output.err.Error() != tt.wantErr {
				t.Fatalf("runRestart() error = %v, want %q", output.err, tt.wantErr)
			}
		})
	}
}

func TestWaitRestartReadyDefault(t *testing.T) {
	originalSendTrigger := sendTrigger
	defer func() {
		sendTrigger = originalSendTrigger
	}()

	t.Run("returns when daemon responds", func(t *testing.T) {
		t.Cleanup(func() {
			sendTrigger = originalSendTrigger
		})
		calls := 0
		sendTrigger = func(_ context.Context, _ dbusipc.TriggerCommand) (dbusipc.TriggerResponse, error) {
			calls++
			if calls < 3 {
				return dbusipc.TriggerResponse{}, errors.New("name has no owner")
			}
			return dbusipc.TriggerResponse{OK: true}, nil
		}

		if err := waitRestartReadyDefault(context.Background()); err != nil {
			t.Fatalf("waitRestartReadyDefault() error = %v", err)
		}
		if calls != 3 {
			t.Fatalf("calls = %d, want 3", calls)
		}
	})
}

func TestRunRestartCommandDefaultIncludesStderr(t *testing.T) {
	t.Parallel()

	err := runRestartCommandDefault(context.Background(), "sh", "-c", "echo boom >&2; exit 1")
	if err == nil {
		t.Fatal("runRestartCommandDefault() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("runRestartCommandDefault() error = %q, want stderr detail", err)
	}
	if !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("runRestartCommandDefault() error = %q, want exit status", err)
	}
}

func TestRunRestartCommandDefaultDoesNotHangOnBackgroundChild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "daemon-like.sh")
	script := "#!/bin/sh\n(sleep 2) &\nexit 0\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	if err := runRestartCommandDefault(context.Background(), scriptPath); err != nil {
		t.Fatalf("runRestartCommandDefault() error = %v, want nil", err)
	}
}
