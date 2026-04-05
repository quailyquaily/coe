package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"coe/internal/config"
	dbusipc "coe/internal/ipc/dbus"
)

var loadRestartConfig = loadRestartConfigDefault
var runRestartCommand = runRestartCommandDefault
var waitRestartReady = waitRestartReadyDefault

func runRestart(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: coe restart")
	}

	cfg, err := loadRestartConfig()
	if err != nil {
		return err
	}
	runtimeMode := config.NormalizeRuntimeMode(cfg.Runtime.Mode)

	if err := runRestartCommand(ctx, "systemctl", "--user", "restart", "coe.service"); err != nil {
		return fmt.Errorf("restart coe.service: %w", err)
	}
	fmt.Println("restarted coe.service")

	if runtimeMode != config.RuntimeModeFcitx {
		return nil
	}

	if err := waitRestartReady(ctx); err != nil {
		return fmt.Errorf("wait for coe daemon ready before restarting fcitx5: %w", err)
	}

	if err := runRestartCommand(ctx, "fcitx5", "-rd"); err != nil {
		return fmt.Errorf("restart fcitx5: %w", err)
	}
	fmt.Println("restarted fcitx5")
	return nil
}

func loadRestartConfigDefault() (config.Config, error) {
	path, err := config.ResolvePath()
	if err != nil {
		return config.Config{}, fmt.Errorf("resolve config path for restart: %w", err)
	}

	cfg, err := config.LoadOrDefault(path)
	if err != nil {
		return config.Config{}, fmt.Errorf("load config for restart: %w", err)
	}
	return cfg, nil
}

func waitRestartReadyDefault(ctx context.Context) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		probeCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		_, err := sendTrigger(probeCtx, dbusipc.TriggerCommandStatus)
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func runRestartCommandDefault(ctx context.Context, name string, args ...string) error {
	logFile, err := os.CreateTemp("", "coe-restart-command-*")
	if err != nil {
		return fmt.Errorf("create temp log file: %w", err)
	}
	logPath := logFile.Name()
	defer os.Remove(logPath)
	defer logFile.Close()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	err = cmd.Run()
	if err == nil {
		return nil
	}

	if _, seekErr := logFile.Seek(0, io.SeekStart); seekErr != nil {
		return err
	}
	output, readErr := io.ReadAll(logFile)
	if readErr != nil {
		return err
	}

	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return err
	}
	return fmt.Errorf("%s: %w", detail, err)
}
