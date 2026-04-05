package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"coe/internal/config"
	"coe/internal/i18n"
)

var hotkeyOutput io.Writer = os.Stdout
var runHotkeyRestart = func(ctx context.Context) error {
	return runRestart(ctx, nil)
}
var pickHotkeyAccelerator = pickHotkeyAcceleratorDefault

var errHotkeyPickCanceled = errors.New("hotkey pick canceled")

type hotkeyPickerTexts struct {
	Title          string `json:"title"`
	Heading        string `json:"heading"`
	Hint           string `json:"hint"`
	Waiting        string `json:"waiting"`
	CapturedFormat string `json:"captured_format"`
	PressFirst     string `json:"press_first"`
	Confirm        string `json:"confirm"`
	Cancel         string `json:"cancel"`
}

func runHotkey(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: coe hotkey pick")
	}

	switch args[0] {
	case "pick":
		return runHotkeyPick(ctx, args[1:])
	default:
		return errors.New("usage: coe hotkey pick")
	}
}

func runHotkeyPick(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: coe hotkey pick")
	}

	rawValue, err := pickHotkeyAccelerator(ctx)
	if err != nil {
		if errors.Is(err, errHotkeyPickCanceled) {
			return err
		}
		return fmt.Errorf("pick hotkey: %w", err)
	}

	normalized, err := config.NormalizePreferredAccelerator(rawValue)
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
	cfg.Hotkey.PreferredAccelerator = normalized
	if err := config.Save(path, cfg); err != nil {
		return err
	}

	fmt.Fprintf(hotkeyOutput, "updated %s: hotkey.preferred_accelerator=%s\n", path, normalized)
	return runHotkeyRestart(ctx)
}

func pickHotkeyAcceleratorDefault(ctx context.Context) (string, error) {
	const script = `
import sys
import os
import json
try:
    import tkinter as tk
except Exception as exc:
    print(f"tkinter unavailable: {exc}", file=sys.stderr)
    sys.exit(2)

MASK_SHIFT = 0x0001
MASK_CONTROL = 0x0004
MASK_ALT = 0x0008
MASK_SUPER = 0x0040

texts = json.loads(os.environ.get("COE_HOTKEY_PICK_TEXTS", "{}"))
current = {"value": ""}

def text(name, default):
    value = texts.get(name)
    if value is None or value == "":
        return default
    return value

def normalize_key(keysym):
    if keysym in ("Shift_L", "Shift_R", "Control_L", "Control_R", "Alt_L", "Alt_R", "Meta_L", "Meta_R", "Super_L", "Super_R", "Hyper_L", "Hyper_R"):
        return None
    if len(keysym) == 1:
        if keysym.isalpha():
            return keysym.lower()
        return keysym
    lowered = keysym.lower()
    mapping = {
        "return": "Return",
        "escape": "Escape",
        "space": "space",
        "tab": "Tab",
        "iso_left_tab": "Tab",
        "backspace": "BackSpace",
        "delete": "Delete",
        "insert": "Insert",
        "home": "Home",
        "end": "End",
        "prior": "Page_Up",
        "next": "Page_Down",
        "left": "Left",
        "right": "Right",
        "up": "Up",
        "down": "Down",
        "print": "Print",
    }
    if lowered in mapping:
        return mapping[lowered]
    if lowered.startswith("f") and lowered[1:].isdigit():
        return "F" + lowered[1:]
    return keysym

def format_accelerator(event):
    modifiers = []
    if event.state & MASK_CONTROL:
        modifiers.append("Control")
    if event.state & MASK_ALT:
        modifiers.append("Alt")
    if event.state & MASK_SHIFT:
        modifiers.append("Shift")
    if event.state & MASK_SUPER:
        modifiers.append("Super")
    key = normalize_key(event.keysym)
    if key is None:
        return "".join(f"<{modifier}>" for modifier in modifiers)
    return "".join(f"<{modifier}>" for modifier in modifiers) + key

def on_key(event):
    accelerator = format_accelerator(event)
    if accelerator:
        current["value"] = accelerator
        status_var.set(text("captured_format", "Captured: %s") % accelerator)
        confirm_button.configure(state="normal")
    return "break"

def on_confirm(_event=None):
    if not current["value"]:
        status_var.set(text("press_first", "Press a hotkey first."))
        return "break"
    print(current["value"])
    root.destroy()
    return "break"

def on_cancel(_event=None):
    print("__COE_HOTKEY_PICK_CANCELED__")
    root.destroy()
    return "break"

root = tk.Tk()
root.title(text("title", "Coe Hotkey Picker"))
root.geometry("420x190")
root.resizable(False, False)
root.attributes("-topmost", True)

frame = tk.Frame(root, padx=16, pady=16)
frame.pack(fill="both", expand=True)

title = tk.Label(frame, text=text("heading", "Press your trigger hotkey"), font=("TkDefaultFont", 12, "bold"))
title.pack(anchor="w")

hint = tk.Label(frame, text=text("hint", "Press Enter to confirm, or use the buttons below."), justify="left")
hint.pack(anchor="w", pady=(8, 0))

status_var = tk.StringVar(value=text("waiting", "Waiting for key combination..."))
status = tk.Label(frame, textvariable=status_var, justify="left", wraplength=380)
status.pack(anchor="w", pady=(16, 0))

buttons = tk.Frame(frame)
buttons.pack(anchor="e", fill="x", pady=(18, 0))

cancel_button = tk.Button(buttons, text=text("cancel", "Cancel"), width=10, command=on_cancel)
cancel_button.pack(side="right")

confirm_button = tk.Button(buttons, text=text("confirm", "Confirm"), width=10, state="disabled", command=on_confirm)
confirm_button.pack(side="right", padx=(0, 8))

root.bind("<KeyPress>", on_key)
root.bind("<Return>", on_confirm)
root.bind("<Escape>", on_cancel)
root.protocol("WM_DELETE_WINDOW", on_cancel)
root.after(50, lambda: (root.lift(), root.focus_force()))
root.mainloop()
`

	python, err := exec.LookPath("python3")
	if err != nil {
		return "", errors.New("hotkey pick requires python3 with tkinter")
	}

	textsJSON, err := json.Marshal(hotkeyPickerTextsFor(i18n.NewFromEnvironment()))
	if err != nil {
		return "", fmt.Errorf("marshal hotkey picker texts: %w", err)
	}

	cmd := exec.CommandContext(ctx, python, "-c", script)
	cmd.Env = append(os.Environ(), "COE_HOTKEY_PICK_TEXTS="+string(textsJSON))
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			return "", err
		}
		return "", errors.New(text)
	}
	if text == "__COE_HOTKEY_PICK_CANCELED__" {
		return "", errHotkeyPickCanceled
	}
	if text == "" {
		return "", errors.New("hotkey picker returned empty accelerator")
	}
	return text, nil
}

func hotkeyPickerTextsFor(localizer i18n.Localizer) hotkeyPickerTexts {
	return hotkeyPickerTexts{
		Title:          localizer.Text(i18n.HotkeyPickerTitle),
		Heading:        localizer.Text(i18n.HotkeyPickerHeading),
		Hint:           localizer.Text(i18n.HotkeyPickerHint),
		Waiting:        localizer.Text(i18n.HotkeyPickerWaiting),
		CapturedFormat: localizer.Text(i18n.HotkeyPickerCapturedFormat),
		PressFirst:     localizer.Text(i18n.HotkeyPickerPressFirst),
		Confirm:        localizer.Text(i18n.HotkeyPickerConfirm),
		Cancel:         localizer.Text(i18n.HotkeyPickerCancel),
	}
}
