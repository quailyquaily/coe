package main

import (
	"context"
	"errors"
	"fmt"

	"coe/internal/config"
)

func runConfig(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: coe config <init|set>")
	}

	switch args[0] {
	case "init":
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
	case "set":
		if len(args) != 3 {
			return errors.New("usage: coe config set <key> <value>")
		}

		path, err := config.ResolvePath()
		if err != nil {
			return err
		}

		cfg, err := config.LoadOrDefault(path)
		if err != nil {
			return err
		}
		if err := config.SetValue(&cfg, args[1], args[2]); err != nil {
			return err
		}
		if err := config.Save(path, cfg); err != nil {
			return err
		}

		fmt.Printf("updated %s: %s=%s\n", path, args[1], args[2])
		return nil
	default:
		return errors.New("usage: coe config <init|set>")
	}
}
