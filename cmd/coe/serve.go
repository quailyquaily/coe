package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"coe/internal/app"
	"coe/internal/config"
)

type serveOptions struct {
	LogLevel string
}

func runServe(parent context.Context, args []string) error {
	options, err := parseServeOptions(args)
	if err != nil {
		return err
	}

	path, err := config.ResolvePath()
	if err != nil {
		return err
	}

	cfg, err := config.LoadOrDefault(path)
	if err != nil {
		return err
	}
	if options.LogLevel != "" {
		cfg.Runtime.LogLevel = options.LogLevel
	}

	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	instance, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}

	return instance.Serve(ctx, os.Stdout)
}

func parseServeOptions(args []string) (serveOptions, error) {
	var opts serveOptions

	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.LogLevel, "log-level", "", "override runtime log level")

	if err := fs.Parse(args); err != nil {
		return serveOptions{}, fmt.Errorf("usage: coe serve [--log-level <debug|info|warn|error>]")
	}
	if fs.NArg() != 0 {
		return serveOptions{}, errors.New("usage: coe serve [--log-level <debug|info|warn|error>]")
	}
	if opts.LogLevel != "" && !isSupportedLogLevel(opts.LogLevel) {
		return serveOptions{}, fmt.Errorf("unsupported log level %q", opts.LogLevel)
	}

	return opts, nil
}

func isSupportedLogLevel(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug", "info", "warn", "warning", "error":
		return true
	default:
		return false
	}
}
