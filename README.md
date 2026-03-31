# Coe (聲)

[English](./README.md) | [简体中文](./docs/README.zh-CN.md) | [日本語](./docs/README.ja.md)

Coe is a voice input tool for Linux desktops.

It is a Linux-focused tribute to [`missuo/koe`](https://github.com/missuo/koe). The goal has not changed: press a hotkey, speak, let an LLM clean up the transcript, and put the text back into the active app.

## The Name

`coe` is close to `koe` on purpose. This project nods to Koe, but it targets Linux and Wayland. The old kanji `聲` means voice. That is the job.

## Why Coe?

The first author uses Linux, but people do not love building desktop software for Linux. Coe tries to make this practical:

- Background process, minimal UI surface
- Plain YAML config
- Reuse other people's capabilities first: fcitx, portal clipboard, and so on
- Make voice input work as well as possible inside those limits

## How It Works

The runtime flow is:

1. Keep `coe serve` running in the background, usually through a user-level `systemd` service.
2. Trigger dictation.
   In `runtime.mode: fcitx`, the Fcitx5 module calls Coe over D-Bus and commits the final text back through the current input context.
   In `runtime.mode: desktop`, GNOME calls `coe trigger toggle` through a custom keyboard shortcut.
3. Record microphone input with `pw-record`.
4. Reject near-silent or obviously corrupt captures instead of sending them out.
5. Send the audio to ASR. Coe supports OpenAI, SenseVoice, or local `whisper.cpp`.
6. Optionally send the transcript to an OpenAI-compatible text model for cleanup.
7. Deliver the final text on screen: either commit it through Fcitx, or paste it back into the focused app.

Notes:

- LLM cleanup: any OpenAI-compatible Chat Completions API by default, or the OpenAI Responses API when configured
- Output delivery: in `desktop` mode, prefer GNOME portal clipboard; fall back to `wl-copy` and `ydotool` when needed

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

It downloads the matching GitHub Release tarball for your Linux architecture. If `fcitx5` is installed, it prefers `fcitx` mode automatically. Otherwise it falls back to `desktop` mode. You can force `fcitx` with `--fcitx`, or force `desktop` with `--gnome`.

For local development, the installer can also use a local release bundle instead of downloading from GitHub Releases. Pass `--bundle` with either:

- a local tarball built by `./scripts/build-release-bundle.sh`
- an extracted bundle directory such as `dist/release/bundle-amd64`

Example:

```bash
./scripts/build-release-bundle.sh dev
./scripts/install.sh --bundle ./dist/release/coe_dev_linux_amd64.tar.gz
```

It then installs:

- `~/.local/bin/coe`
- `~/.config/systemd/user/coe.service`
- `~/.config/coe/env`
- the Fcitx5 module when `fcitx5` is available
- the GNOME Shell extension only when the `desktop` path is selected

After installation it also:

- runs `coe doctor`
- restarts `coe.service`
- checks whether `coe.service` is active
- prints where the binary, config, env file, systemd unit, and desktop-specific assets were installed

If you use a cloud ASR or LLM provider, put the required API key into `~/.config/coe/env`, or write it directly into `~/.config/coe/config.yaml`.

If you use `fcitx` mode, the Fcitx panel will also show a short Coe status hint while listening and processing.

If the current path is `desktop`, log out and log back in once so GNOME Shell and the user service session both pick up the new extension cleanly.

Then open any app with an input focus, press the default shortcut `<Shift><Super>d`, speak, then press it again. If all is well, your speech should come back as text in that app.


### Install Dependencies

**`fcitx5` mode**

- `fcitx5`
- `pw-record`

**`desktop` mode**

- `pw-record`
- `wl-copy`

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

### Hotkey

- default trigger key: `<Shift><Super>d`
- default trigger behavior: `hotkey.trigger_mode: toggle`
- change it with `coe config set hotkey.trigger_mode toggle` or `coe config set hotkey.trigger_mode hold`
- `hold` means press to start recording and release to stop, and only takes effect in `runtime.mode: fcitx`

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
- `prompt` is rendered as a Go `text/template` before being sent as the initial prompt
- `prompt_file` lets you keep that template in a separate file; relative paths are resolved from `config.yaml`
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
- model: `gpt-5.4-nano`
- direct key field: `llm.api_key`
- environment field: `OPENAI_API_KEY`

If you want to use the OpenAI Responses API instead, set `llm.endpoint_type` to `responses`.
`llm.prompt` is also rendered as a Go `text/template` before it is used as correction instructions.
`llm.prompt_file` works the same way and is preferred when you want the template outside YAML.

### Personal Dictionary

- config field: `dictionary.file`
- format: YAML with `canonical`, `aliases`, and optional `scenes`
- wrap strings in double quotes
- use compact arrays for `aliases` when possible, for example `["system control", "system c t l"]`
- dictionary entries are injected into the LLM correction prompt and applied again as deterministic post-correction normalization
- single-character aliases are not injected into prompts; they only use strict token-boundary replacement in code
- v1 does not hot-reload the dictionary; restart `coe.service` after editing it
- `coe config init` creates or backfills `./dictionary.yaml` next to `config.yaml` with two starter entries

Example:

```yaml
dictionary:
  file: "./dictionary.yaml"
```

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
- `notify_on_complete: false`
- `notify_on_recording_start: false`

If `notify_on_complete` is enabled, the completion notification includes the corrected text.

### Runtime

- `log_level: info`
- set `log_level: debug` to print per-stage timings and output fallback details
- new configs default to `runtime.mode: fcitx`; set `runtime.mode: desktop` only if you want to force the GNOME fallback path
- or override it for one run: `coe serve --log-level debug`

For GNOME focus-aware paste, see:

- [`config.example.yaml`](./config.example.yaml)
- [`docs/gnome-focus-helper.md`](./docs/gnome-focus-helper.md)

New configs enable focus-aware paste by default. Older configs can still override `output.use_gnome_focus_helper` if needed.

## Current Status

Working:

- [x] compatibility with other desktop environments through the Fcitx5 module
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

- `coe doctor`
- `coe config init`
- `coe serve`
- `coe trigger toggle`
- `coe trigger start`
- `coe trigger stop`
- `coe trigger status`
- `coe version`

## Docs

- [`docs/README.md`](./docs/README.md)
- [`docs/install.md`](./docs/install.md)
- [`docs/architecture.md`](./docs/architecture.md)
- [`docs/fallbacks.md`](./docs/fallbacks.md)
- [`docs/gnome-globalshortcuts-matrix.md`](./docs/gnome-globalshortcuts-matrix.md)
- [`docs/pw-record-exit-status.md`](./docs/pw-record-exit-status.md)
- [`docs/roadmap.md`](./docs/roadmap.md)
