# Coe (聲)

[English](./README.md) | [简体中文](./docs/README.zh-CN.md) | [日本語](./docs/README.ja.md)

Coe is a dictation tool for GNOME on Wayland on Linux.

It is a Linux-focused recreation of [`missuo/koe`](https://github.com/missuo/koe). The goal has not changed: press a hotkey, speak, let an LLM clean up the transcript, and put the text back into the active app.

## The Name

`coe` is close to `koe` on purpose. This project nods to Koe, but it targets Linux and Wayland. The old kanji `聲` means voice. That is the job.

## Why Coe?

Most Linux voice input tools are not pleasant to use. Coe tries to make this usable:

- GNOME first, Wayland first
- Background process, minimal UI surface
- Plain YAML config
- Reuse platform capabilities first: portal clipboard, portal paste, desktop notifications
- Keep a degraded fallback path

## How It Works

The runtime flow is:

1. Keep `coe serve` running.
2. Trigger dictation with `coe trigger toggle`. Today GNOME usually calls this through a custom shortcut. When `GlobalShortcuts` is missing, Coe inserts and maintains that shortcut at startup.
3. Record microphone input with `pw-record`.
4. Reject near-silent or obviously corrupt captures before they leave the machine.
5. Send the audio to ASR. OpenAI Audio Transcriptions is the default, but the provider is configurable.
6. Optionally send the transcript to an OpenAI-compatible text model for cleanup.
7. Write the corrected text through the clipboard path.
8. Paste it back into the focused app when the runtime allows it.

Notes:

- ASR: optional local `whisper.cpp` provider through `whisper-cli`
- ASR: optional external `SenseVoice` FastAPI provider
- LLM correction: OpenAI-compatible Chat Completions through `uniai` by default, or Responses API when configured
- Output: portal clipboard and portal paste first, `wl-copy` and `ydotool` as fallbacks

## Installation

### Install Dependencies

Runtime requirements:

- Wayland session
- GNOME desktop
- `pw-record`
- `wl-copy`
- `OPENAI_API_KEY`

You can keep the key in `~/.config/coe/env`, or write it directly into `asr.api_key` and `llm.api_key` in `config.yaml`.

Optional:

- `ydotool`, if you want to try the command-line paste fallback
- `whisper-cli` and a Whisper model file, if you want local ASR
- a running SenseVoice FastAPI service, if you want local network ASR through SenseVoice

On Ubuntu, install the command-line dependencies with:

```bash
sudo apt update
sudo apt install -y pipewire-bin wl-clipboard
```

Optional paste fallback:

```bash
sudo apt install -y ydotool
```

### Download a Prebuilt Package

[Download](https://github.com/quailyquaily/coe/releases)

### Or Build from Source

#### Prerequisites

```bash
git clone https://github.com/quailyquaily/coe.git
cd coe
go build -o coe ./cmd/coe
```

## Run

```bash
./coe serve
```

### Install as a User `systemd` Service

If you want to install the current alpha as a persistent user service:

```bash
./scripts/install.sh
```

The script downloads the matching GitHub Release tarball for your Linux architecture, then installs:

- `~/.local/bin/coe`
- `~/.config/systemd/user/coe.service`
- `~/.config/coe/env`
- `~/.local/share/gnome-shell/extensions/coe-focus-helper@mistermorph.com`

After installation it also:

- runs `coe doctor`
- restarts `coe.service`
- checks whether `coe.service` is active
- prints the installed file locations

You can also pin a version:

```bash
./scripts/install.sh v0.0.4
```

If you use cloud ASR or LLM providers, put the required API key into `~/.config/coe/env` or write it directly into `~/.config/coe/config.yaml`. After install, log out and log back in once so GNOME Shell and the user service session both pick up the new extension cleanly. Then restart the service if needed:

```bash
systemctl --user restart coe.service
```

If you prefer, you can also leave `~/.config/coe/env` empty and write the key directly into `~/.config/coe/config.yaml` under `asr.api_key` and `llm.api_key`.

## Hotkey to Start and Stop Dictation

- name: `coe-trigger`
- default shortcut: `<Shift><Super>d`
- on GNOME fallback, Coe tries to ensure a matching custom shortcut at startup

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
- or override it for one run: `coe serve --log-level debug`

For GNOME focus-aware paste, see:

- [`config.example.yaml`](./config.example.yaml)
- [`docs/gnome-focus-helper.md`](./docs/gnome-focus-helper.md)

New configs enable focus-aware paste by default. Older configs can still override `output.use_gnome_focus_helper` if needed.

## Current Status

Working:

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
