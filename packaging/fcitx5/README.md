# Coe Fcitx5 Module

This directory contains the thin Fcitx5 module for Coe.

Current scope:

- registers as a Fcitx5 module
- watches key events in `PreInputMethod`
- reads the trigger key from Coe over session D-Bus
- falls back to `Shift+Super+D` if Coe is unavailable during module init
- calls `com.mistermorph.Coe.Dictation1.Toggle()` over session D-Bus
- subscribes to `StateChanged` / `ResultReady` / `ErrorRaised` over session D-Bus
- dispatches the result back to the Fcitx main event loop
- shows a small Fcitx panel hint while Coe is listening or processing
- commits the final text to the current focused input context

It does not do these things yet:

- clipboard fallback when no input context exists at commit time

## Build

```bash
./scripts/build-fcitx-module.sh
```

## Install

For release installs, use:

```bash
./scripts/install.sh
```

If `fcitx5` is installed, that script prefers the Fcitx path automatically and
installs the module into the system addon directory. Use `--fcitx` to force
that path, or `--gnome` if you want the GNOME desktop path instead.

For local development, build with:

```bash
./scripts/build-fcitx-module.sh --system
sudo cmake --install /tmp/coe-fcitx5-build
```

## Hotkey

The module does not keep its own hotkey file. It reads the trigger from Coe
over D-Bus, so the single source of truth is still:

- `~/.config/coe/config.yaml`

Example:

```yaml
hotkey:
  preferred_accelerator: <Shift><Super>d
```

In `runtime.mode: fcitx`, the module converts that GNOME-style accelerator to
the Fcitx key format internally.

Set this in `~/.config/coe/config.yaml` before testing the module:

```yaml
runtime:
  mode: fcitx
```

The install script will try to restart Fcitx5 with:

```bash
fcitx5 -rd
```

If that does not pick up the new module, log out and back in.
