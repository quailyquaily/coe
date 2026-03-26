package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"coe/internal/app"
	"coe/internal/capabilities"
	"coe/internal/config"
	"coe/internal/control"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(parent context.Context, args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "doctor":
		return runDoctor(parent)
	case "config":
		return runConfig(parent, args[1:])
	case "serve":
		return runServe(parent)
	case "trigger":
		return runTrigger(parent, args[1:])
	case "version":
		printVersion()
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runDoctor(ctx context.Context) error {
	caps, err := capabilities.Probe(ctx)
	if err != nil {
		return err
	}

	fmt.Print(caps.Report())
	return nil
}

func runConfig(_ context.Context, args []string) error {
	if len(args) == 0 || args[0] != "init" {
		return errors.New("usage: coe config init")
	}

	path, err := config.ResolvePath()
	if err != nil {
		return err
	}

	written, err := config.WriteDefault(path, false)
	if err != nil {
		return err
	}

	if written {
		fmt.Printf("wrote default config to %s\n", path)
		return nil
	}

	fmt.Printf("config already exists at %s\n", path)
	return nil
}

func runServe(parent context.Context) error {
	path, err := config.ResolvePath()
	if err != nil {
		return err
	}

	cfg, err := config.LoadOrDefault(path)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	instance, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}

	return instance.Serve(ctx, os.Stdout)
}

func runTrigger(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: coe trigger <toggle|start|stop|status>")
	}

	socketPath, err := control.ResolveSocketPath()
	if err != nil {
		return err
	}

	command, err := parseTriggerCommand(args[0])
	if err != nil {
		return err
	}

	resp, err := control.Send(ctx, socketPath, control.Request{Command: command})
	if err != nil {
		return err
	}

	fmt.Printf("%s (active=%t)\n", resp.Message, resp.Active)
	if !resp.OK {
		return errors.New(resp.Message)
	}
	return nil
}

func parseTriggerCommand(arg string) (control.Command, error) {
	switch arg {
	case "toggle":
		return control.CommandTriggerToggle, nil
	case "start":
		return control.CommandTriggerStart, nil
	case "stop":
		return control.CommandTriggerStop, nil
	case "status":
		return control.CommandTriggerStatus, nil
	default:
		return "", fmt.Errorf("unknown trigger command %q", arg)
	}
}

func printUsage() {
	fmt.Println("coe - Coe dictation assistant for GNOME on Wayland")
	fmt.Println()
	fmt.Println("usage:")
	fmt.Println("  coe doctor")
	fmt.Println("  coe config init")
	fmt.Println("  coe serve")
	fmt.Println("  coe trigger <toggle|start|stop|status>")
	fmt.Println("  coe version")
}

func printVersion() {
	fmt.Printf("coe %s\n", version)
	fmt.Printf("commit: %s\n", commit)
	fmt.Printf("date: %s\n", date)
	fmt.Printf("built_by: %s\n", builtBy)
}
