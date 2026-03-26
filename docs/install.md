# Install

## Goal

These steps turn the repository into a repeatable GNOME Wayland alpha install instead of a manually started daemon.

The current install target is:

- one user-scoped binary in `~/.local/bin/coe`
- one `systemd --user` service
- one env file for secrets
- one GNOME custom shortcut that triggers the daemon

## Quick install

From the repository root:

```bash
./scripts/install-user.sh
```

This does four things:

1. builds `./cmd/coe`
2. installs the binary to `~/.local/bin/coe`
3. installs `packaging/systemd/coe.service` into `~/.config/systemd/user/`
4. enables and starts the user service

## Required secret

Put your OpenAI key in:

- `~/.config/coe/env`

Example:

```bash
OPENAI_API_KEY=sk-...
```

Then restart the service:

```bash
systemctl --user restart coe.service
```

## Default config and state

Config file:

- `~/.config/coe/config.yaml`

Runtime state:

- `XDG_STATE_HOME/coe/state.json`
- fallback: `~/.config/coe/state.json`

The state file stores the portal restore token used to avoid repeated authorization prompts when the desktop backend accepts persistence.

## GNOME shortcut

Add a GNOME custom shortcut that runs:

```bash
~/.local/bin/coe trigger toggle
```

This is the current fallback trigger path on GNOME systems that do not expose `GlobalShortcuts`.

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
