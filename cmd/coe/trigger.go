package main

import (
	"context"
	"errors"
	"fmt"

	"coe/internal/control"
)

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
