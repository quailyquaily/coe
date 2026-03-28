# Coe (聲)

[English](./README.md) | [简体中文](./docs/README.zh-CN.md) | [日本語](./docs/README.ja.md)

Coe is a voice input tool for Linux desktops.

It is a Linux-focused tribute to [`missuo/koe`](https://github.com/missuo/koe). The goal has not changed: press a hotkey, speak, let an LLM clean up the transcript, and put the text back into the active app.

> Today, two paths are real: Fcitx5 on Linux desktop sessions, and GNOME on Wayland. Other desktops or X11 sessions may still run parts of the pipeline, but they are not polished targets.

## The Name

`coe` is close to `koe` on purpose. This project nods to Koe, but it targets Linux and Wayland. The old kanji `聲` means voice. That is the job.

## Why Coe?

The first author uses Linux, but people do not love building desktop software for Linux. Coe tries to make this practical:

- Background process, minimal UI surface
- Plain YAML config
- Reuse platform capabilities first: Fcitx commit, portal clipboard, portal paste, desktop notifications
- Make voice input work as well as possible inside those limits

## How It Works

The runtime flow is:

1. Keep `coe serve` running in the background, usually through a user-level `systemd` service.
2. Trigger dictation.
   In `runtime.mode: fcitx`, the Fcitx5 module calls Coe over D-Bus and commits the final text back through the current input context.
   In `runtime.mode: desktop`, GNOME usually calls `coe trigger toggle` through a custom shortcut fallback.
3. Record microphone input with `pw-record`.
4. Reject near-silent or obviously corrupt captures instead of sending them out.
5. Send the audio to ASR. Coe supports OpenAI, SenseVoice, or local `whisper.cpp`.
6. Optionally send the transcript to an OpenAI-compatible text model for cleanup.
7. Either commit the corrected text through Fcitx, or write it through the clipboard path.
8. Paste it back into the focused app when the runtime allows it.

Notes:

- LLM cleanup: any OpenAI-compatible Chat Completions API by default, or the OpenAI Responses API when configured
- Output: prefer GNOME portal clipboard and portal paste; fall back to `wl-copy` and `ydotool` when needed

## Desktop Integration

Two integration paths exist today:

- `runtime.mode: fcitx`
  - thin Fcitx5 module
  - hotkey handled inside Fcitx
  - final text committed with `CommitString`
  - small Fcitx panel status while listening or processing
- `runtime.mode: desktop`
  - GNOME-first desktop path
  - GNOME custom shortcut fallback when `GlobalShortcuts` is unavailable
  - portal clipboard and portal paste
  - GNOME Shell focus helper for terminal-aware paste

These parts are GNOME-specific today:

- the install script installs a GNOME Shell extension for focus-aware paste
- when `GlobalShortcuts` is unavailable, Coe auto-manages a GNOME custom shortcut fallback
- focus-aware paste depends on a GNOME Shell extension that exposes the focused window `wm_class` over D-Bus

The core dictation pipeline is less desktop-specific:

- recording through `pw-record`
- ASR through OpenAI, `whisper.cpp`, or SenseVoice
- LLM cleanup
- clipboard and paste delivery, as far as the current desktop runtime allows

## Installation

### Quick Install

The simplest path is the release installer:

```bash
curl -fsSL -o /tmp/install.sh https://raw.githubusercontent.com/quailyquaily/coe/refs/heads/master/scripts/install.sh
bash /tmp/install.sh
```

It downloads the matching GitHub Release tarball for your Linux architecture. If `fcitx5` is installed, it prefers the Fcitx5 path automatically. Otherwise it falls back to the GNOME desktop path. You can force Fcitx with `--fcitx` or GNOME with `--gnome`.

It then installs:

- `~/.local/bin/coe`
- `~/.config/systemd/user/coe.service`
- `~/.config/coe/env`
- the Fcitx5 module when `fcitx5` is available
- the GNOME focus helper extension only when the GNOME path is selected

After installation it also:

- runs `coe doctor`
- restarts `coe.service`
- checks whether `coe.service` is active
- prints where the binary, config, env file, systemd unit, and desktop-specific assets were installed

If you use cloud ASR or LLM providers, put the required API key into `~/.config/coe/env` or write it directly into `~/.config/coe/config.yaml`.

On the GNOME path, log out and log back in once so GNOME Shell and the user service session both pick up the new extension cleanly.

Then open any app with an input focus, press the default shortcut `<Shift><Super>d`, speak, then press it again. If all is well, your speech should come back as text in that app. In `runtime.mode: fcitx`, the Fcitx panel will show a small Coe status hint while it is listening or processing.

### Install Dependencies

Runtime requirements:

- Linux desktop session
- `pw-record`
- `wl-copy`

Recommended desktop integrations:

- Fcitx5 for the primary install path
- GNOME on Wayland for the desktop fallback path

Optional:

- clipboard: `ydotool`, if you want the command-line paste fallback
- LLM: any OpenAI-compatible API for text cleanup, with the key in `~/.config/coe/env` or `llm.api_key`
- ASR: `whisper-cli` and a Whisper model file, if you want local ASR
- ASR: a running SenseVoice FastAPI service, if you want local network ASR
- ASR: OpenAI transcription, with the key in `~/.config/coe/env` or `asr.api_key`

On Ubuntu, install the command-line dependencies with:

```bash
sudo apt update
sudo apt install -y pipewire-bin wl-clipboard
sudo apt install -y ydotool
```

## Hotkey to Start and Stop Dictation

- name: `coe-trigger`
- default shortcut: `<Shift><Super>d`
- in `runtime.mode: desktop`, Coe tries to ensure a matching GNOME custom shortcut at startup
- in `runtime.mode: fcitx`, the Fcitx5 module reads the same `hotkey.preferred_accelerator` over D-Bus
- the module converts the GNOME-style value like `<Shift><Super>d` to Fcitx key syntax internally

## Configuration

Coe uses plain files for configuration.

Config file:

- `~/.config/coe/config.yaml`
- repo example: [`config.example.yaml`](./config.example.yaml)

Runtime state:

- `XDG_STATE_HOME/coe/state.json`
- fallback: `~/.config/coe/state.json`

The state file stores the portal restore token so repeated authorization prompts can be reduced when the desktop backend supports persistence.

Create the default config with:

```bash
go run ./cmd/coe config init
```

That writes `~/.config/coe/config.yaml`, unless you override the path with `COE_CONFIG`.

Or start from the repo example:

```bash
cp config.example.yaml ~/.config/coe/config.yaml
```

Current defaults:

### ASR

- provider: `openai`
- endpoint: `https://api.openai.com/v1/audio/transcriptions`
- model: `gpt-4o-mini-transcribe`
- direct key field: `asr.api_key`
- environment field: `OPENAI_API_KEY`

To switch to local `whisper.cpp`:

```yaml
asr:
  provider: whispercpp
  endpoint: ""
  model: ""
  language: zh
  api_key: ""
  api_key_env: ""
  binary: whisper-cli
  model_path: /absolute/path/to/ggml-base.bin
  threads: 4
  use_gpu: false
```

Notes:

- `binary` defaults to `whisper-cli`
- `model_path` is required for `whisper.cpp`
- `prompt` is passed through as the initial prompt
- `threads` defaults to `GOMAXPROCS`
- `use_gpu: false` adds `--no-gpu`

To switch to SenseVoice FastAPI:

```yaml
asr:
  provider: sensevoice
  endpoint: http://127.0.0.1:50000/api/v1/asr
  model: ""
  language: auto
  api_key: ""
  api_key_env: ""
  binary: ""
  model_path: ""
  threads: 0
  use_gpu: false
```

Notes:

- `endpoint` should point at the official SenseVoice FastAPI service
- `language` maps to the service `lang` form field, for example `auto`, `zh`, `en`, `yue`, `ja`, `ko`
- Coe uploads one WAV file per request and uses the first entry from the returned `result` array
- the official repo exposes this service at `http://127.0.0.1:50000/api/v1/asr` when started with `uvicorn api:app --host 0.0.0.0 --port 50000`

### LLM Correction

- provider: `openai`
- endpoint type: `chat`
- endpoint: `https://api.openai.com/v1`
- model: `gpt-4o-mini`
- direct key field: `llm.api_key`
- environment field: `OPENAI_API_KEY`

If you want to use the OpenAI Responses API instead, set `llm.endpoint_type` to `responses`.

### Audio

- recorder: `pw-record`
- sample rate: `16000`
- channels: `1`
- format: `s16`

### Output

- clipboard: `wl-copy`
- when the runtime exposes portal support, clipboard and paste prefer portal
- `wl-copy` and `ydotool` remain command-line fallbacks
- new configs enable GNOME focus-aware paste by default, so terminal-like targets can switch from `Ctrl+V` to `Ctrl+Shift+V`

### Notifications

- `enable_system: true`
- `show_text_preview: true`
- `notify_on_recording_start: false`

### Runtime

- `log_level: info`
- set `log_level: debug` to print per-stage timings and output fallback details
- set `runtime.mode: fcitx` when you want the Fcitx5 module to drive dictation instead of the GNOME fallback shortcut
- or override it for one run: `coe serve --log-level debug`

For GNOME focus-aware paste, see:

- [`config.example.yaml`](./config.example.yaml)
- [`docs/gnome-focus-helper.md`](./docs/gnome-focus-helper.md)

New configs enable focus-aware paste by default. Older configs can still override `output.use_gnome_focus_helper` if needed.

## Current Status

Working:

- [x] Fcitx5 module trigger and `CommitString` path
- [x] Fcitx panel status while listening or processing
- [x] GNOME Wayland fallback trigger through an auto-managed GNOME custom shortcut that runs `coe trigger toggle`
- [x] microphone capture through `pw-record`
- [x] batch transcription through OpenAI Audio Transcriptions
- [x] optional SenseVoice FastAPI ASR provider
- [x] transcript cleanup through OpenAI-compatible Chat Completions by default, with Responses as an option
- [x] final text written through portal clipboard
- [x] final text auto-pasted through portal keyboard injection
- [x] GNOME desktop notifications
- [x] near-silent captures are short-circuited locally before ASR
- [x] severely clipped or corrupted captures are short-circuited locally before ASR

Missing:

- [ ] `GlobalShortcuts` portal support
- [ ] a KDE or Hyprland validation pass
- [ ] a stronger answer for the upstream microphone / PipeWire saturation issue

## Other

Portal access persistence:

- If `persist_portal_access` is `true`, Coe stores the portal restore token locally
- After the first successful authorization, later runs try to reuse that token instead of prompting every time
- If GNOME or the portal backend rejects the stored token, Coe falls back to a fresh authorization flow

System notifications:

- By default, Coe sends GNOME desktop notifications for successful dictation and failure cases
- Near-silent or corrupt captures are reported locally and skipped before network transcription
- Recording-start notifications stay off by default

## Commands

- `go run ./cmd/coe doctor`
- `go run ./cmd/coe config init`
- `go run ./cmd/coe config set runtime.mode fcitx`
- `go run ./cmd/coe config set runtime.mode desktop`
- `go run ./cmd/coe serve`
- `go run ./cmd/coe trigger toggle`
- `go run ./cmd/coe trigger start`
- `go run ./cmd/coe trigger stop`
- `go run ./cmd/coe trigger status`
- `go run ./cmd/coe version`

## Docs

- [`docs/README.md`](./docs/README.md)
- [`docs/install.md`](./docs/install.md)
- [`docs/architecture.md`](./docs/architecture.md)
- [`docs/fallbacks.md`](./docs/fallbacks.md)
- [`docs/gnome-globalshortcuts-matrix.md`](./docs/gnome-globalshortcuts-matrix.md)
- [`docs/pw-record-exit-status.md`](./docs/pw-record-exit-status.md)
- [`docs/roadmap.md`](./docs/roadmap.md)
