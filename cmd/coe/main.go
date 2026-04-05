package main

import (
	"context"
	"fmt"
	"os"

	"coe/internal/config"
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
	case "doctor", "serve":
		if err := config.LoadEnvFile(); err != nil {
			return err
		}
	}

	switch args[0] {
	case "doctor":
		return runDoctor(parent)
	case "config":
		return runConfig(parent, args[1:])
	case "hotkey":
		return runHotkey(parent, args[1:])
	case "restart":
		return runRestart(parent, args[1:])
	case "serve":
		return runServe(parent, args[1:])
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
