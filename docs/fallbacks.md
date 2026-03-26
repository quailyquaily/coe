# Coe Fallbacks

## Goal

Fallbacks exist to keep the GNOME-first build usable when a portal path is missing.

The most important constraint is:

- Third-party binaries can help with audio capture, clipboard write, and synthetic input.
- They do **not** provide a clean GNOME Wayland replacement for portal-backed global hotkey capture.

## Current shipped behavior vs target behavior

| Capability | Target preferred path | Current shipped path | Notes |
| --- | --- | --- | --- |
| Global trigger | `GlobalShortcuts` portal | GNOME custom shortcut that runs `coe trigger toggle` | Shipped path is toggle-style, not true hold-to-talk |
| Audio capture | `pw-record` | `pw-record` | This is already the production implementation |
| Clipboard write | `Clipboard` portal | portal first, `wl-copy` fallback | Portal path is implemented, but still needs end-to-end GNOME validation |
| Auto-paste | `RemoteDesktop` portal | portal first | Implemented, but still needs end-to-end GNOME validation |
| Auto-paste fallback | `RemoteDesktop` portal | `ydotool` | Command fallback remains available |
| wlroots-specific paste | `RemoteDesktop` portal | `wtype` planned only | Outside the GNOME-first scope |

## GNOME trigger fallback

GNOME Wayland does not expose a general-purpose, app-level global key listener outside the portal path.

For GNOME, the supported fallback is:

1. Run `coe serve`
2. Add a GNOME custom shortcut
3. Point the shortcut command at `coe trigger toggle`

This gives:

- one shortcut
- one long-lived daemon
- local state that survives across shortcut invocations

What it does **not** give:

- press/release semantics
- true hold-to-talk

Those remain portal-only goals.

The shipped control surface is:

- `coe trigger toggle`
- `coe trigger start`
- `coe trigger stop`
- `coe trigger status`

These commands talk to the running daemon over a local Unix socket.

## Third-party binary policy

### `pw-record`

Role:

- microphone capture

Why it is accepted:

- stable PipeWire command-line tool
- avoids premature cgo binding work

Current nuance:

- in this repository's validated GNOME environment, stopping `pw-record` after a successful recording currently yields `exit status 1`
- this is being treated as a non-fatal warning while audio bytes are present
- the root cause is documented in [`pw-record-exit-status.md`](./pw-record-exit-status.md)

### `wl-copy`

Role:

- clipboard write fallback

Why it is accepted:

- simple interface
- already common on Wayland desktops

Tradeoff:

- still depends on compositor behavior and a background clipboard owner process
- it is the fallback clipboard delivery path when portal output is unavailable or fails

### `ydotool`

Role:

- auto-paste fallback when portal injection is missing or undesired

Why it is accepted:

- works through `/dev/uinput`, so it is not tied to one compositor family

Tradeoffs:

- needs `ydotoold`
- needs root or suitable `uinput`/group permissions
- is less user-friendly than the portal path
- command fallback remains useful when portal paste is unavailable or denied

### `wtype`

Role:

- compositor-specific paste fallback outside GNOME-first scope

Why it is not the primary GNOME fallback:

- oriented around wlroots virtual-keyboard support
- not a dependable GNOME baseline

## Runtime behavior

The daemon should prefer capabilities in this order:

1. Portal
2. Supported external command
3. Degraded workflow

Examples:

- Missing `GlobalShortcuts` on GNOME:
  use `coe trigger toggle`
- Missing or failing portal clipboard delivery:
  use `wl-copy`
- Missing or denied portal paste:
  prefer `ydotool` if installed and configured

## Near-term implementation shape

The repository should support:

- `coe serve`
- `coe trigger toggle`
- `coe trigger start`
- `coe trigger stop`
- `coe trigger status`

These commands talk to the running daemon over a local Unix socket.

That socket-based control plane is the fallback foundation for GNOME.

## Current repository status

Implemented now:

- external-trigger fallback through `coe trigger toggle`
- `pw-record`-backed capture lifecycle
- OpenAI ASR from recorded audio
- OpenAI LLM correction from transcript text
- portal-backed clipboard and paste session wiring
- `wl-copy` clipboard fallback from the pipeline output stage
- `ydotool` command fallback for auto-paste

Validated on a real GNOME target machine:

- portal clipboard delivery on a focused text editor
- portal auto-paste through `RemoteDesktop` into a focused text editor
