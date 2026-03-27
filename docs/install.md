# Install

## Goal

These steps turn the repository into a repeatable GNOME Wayland alpha install instead of a manually started daemon.

The current install target is:

- one user-scoped binary in `~/.local/bin/coe`
- one `systemd --user` service
- one env file for secrets
- one GNOME Shell extension for focus-aware paste
- one GNOME custom shortcut that Coe ensures at startup when `GlobalShortcuts` is unavailable

## Quick install

From the repository root:

```bash
./scripts/install.sh
```

This downloads the matching GitHub Release tarball for your machine and does five things:

1. downloads the release archive for your Linux architecture
2. installs the binary to `~/.local/bin/coe`
3. installs `packaging/systemd/coe.service` into `~/.config/systemd/user/`
4. installs the GNOME focus helper extension into `~/.local/share/gnome-shell/extensions/`
5. enables and starts the user service

After that it also:

- runs `coe doctor`
- restarts `coe.service`
- verifies that `coe.service` is active
- prints where the binary, config, env file, systemd unit, and GNOME extension were installed

You can pin a version explicitly:

```bash
./scripts/install.sh v0.0.4
```

## Credentials

If you use cloud ASR or LLM providers, put the required API key in:

- `~/.config/coe/env`

Example:

```bash
OPENAI_API_KEY=sk-...
```

Then restart the service:

```bash
systemctl --user restart coe.service
```

If you prefer, you can keep `~/.config/coe/env` empty and store provider-specific keys directly in `asr.api_key` and `llm.api_key` inside `~/.config/coe/config.yaml`.

## Default config and state

Config file:

- `~/.config/coe/config.yaml`

Runtime state:

- `XDG_STATE_HOME/coe/state.json`
- fallback: `~/.config/coe/state.json`

The state file stores the portal restore token used to avoid repeated authorization prompts when the desktop backend accepts persistence.

Desktop notifications:

- successful dictation and failure cases are reported through `org.freedesktop.Notifications`
- recording-start notifications are disabled by default

Runtime logging:

- set `runtime.log_level: debug` in `~/.config/coe/config.yaml` to print per-stage timings and output fallback details
- or override it for one run with `coe serve --log-level debug`

## GNOME shortcut

On GNOME systems that do not expose `GlobalShortcuts`, Coe now tries to ensure a GNOME custom shortcut at startup. It uses:

- the executable path for `coe trigger toggle`
- `hotkey.name` as the displayed shortcut name
- `hotkey.preferred_accelerator` as the binding

If startup cannot write GNOME shortcut settings, Coe logs a startup warning and you can still add the shortcut manually.

## GNOME focus helper

The install script also copies the Coe GNOME Shell extension to:

- `~/.local/share/gnome-shell/extensions/coe-focus-helper@mistermorph.com`

If `gnome-extensions` is available, the script will try to enable it. New configs enable focus-aware paste by default. If you already had a config file before this change, make sure it contains:

```yaml
output:
  use_gnome_focus_helper: true
```

After installation, log out and log back in once. That gives GNOME Shell and the `systemd --user` session a clean chance to pick up the new extension and user service environment.

## Useful commands

Check runtime capability detection:

```bash
~/.local/bin/coe doctor
```

Follow service logs:

```bash
journalctl --user -u coe.service -f
```

Restart after changing config or env:

```bash
systemctl --user restart coe.service
```

Stop the daemon:

```bash
systemctl --user stop coe.service
```

Disable automatic start:

```bash
systemctl --user disable --now coe.service
```

## Notes

- The user service imports common graphical session environment variables during installation, but on some desktops you may still need a re-login before all variables are visible to `systemd --user`.
- The first successful portal authorization may still prompt once. Later runs should reuse the saved restore token when GNOME accepts restoration.
- GNOME custom shortcut fallback is idempotent. Repeated starts update the same shortcut instead of appending duplicates.
