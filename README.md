# COE

GNOME-first, Wayland-first dictation assistant in Go.

## Current status

The repository is no longer only a skeleton. The currently verified path is:

`GNOME custom shortcut -> coe trigger toggle -> pw-record -> OpenAI ASR -> OpenAI LLM correction -> wl-copy`

Validated so far:

- GNOME Wayland fallback trigger via `coe trigger toggle`
- microphone capture through `pw-record`
- batch transcription through OpenAI Audio Transcriptions
- transcript cleanup through OpenAI Responses
- final text written to the clipboard through `wl-copy`

Important limits in the current codebase:

- `GlobalShortcuts` portal is not implemented yet
- portal-backed clipboard output is implemented but not yet validated end-to-end on a real GNOME target
- portal-backed auto-paste is implemented but not yet validated end-to-end on a real GNOME target
- `ydotool` remains the command-line paste fallback

## Requirements

- Wayland session
- GNOME desktop
- `pw-record`
- `wl-copy`
- `OPENAI_API_KEY`

Optional:

- `ydotool` if you want to experiment with auto-paste fallback later

## Quick start

Initialize config:

```bash
go run ./cmd/coe config init
```

Export your OpenAI API key:

```bash
export OPENAI_API_KEY=...
```

Inspect runtime capabilities:

```bash
go run ./cmd/coe doctor
```

Start the daemon:

```bash
go run ./cmd/coe serve
```

Trigger dictation manually:

```bash
go run ./cmd/coe trigger toggle
```

On GNOME Wayland without `GlobalShortcuts`, add a GNOME custom shortcut that runs:

```bash
coe trigger toggle
```

## Defaults

ASR defaults:

- endpoint: `https://api.openai.com/v1/audio/transcriptions`
- model: `gpt-4o-mini-transcribe`
- api key env: `OPENAI_API_KEY`

LLM correction defaults:

- endpoint: `https://api.openai.com/v1/responses`
- model: `gpt-4o-mini`
- api key env: `OPENAI_API_KEY`

Audio defaults:

- recorder: `pw-record`
- sample rate: `16000`
- channels: `1`
- format: `s16`

Output defaults:

- clipboard: `wl-copy`
- clipboard and paste will prefer portal paths when the runtime exposes them
- `wl-copy` and `ydotool` remain command-line fallbacks

## Commands

- `go run ./cmd/coe doctor`
- `go run ./cmd/coe config init`
- `go run ./cmd/coe serve`
- `go run ./cmd/coe trigger toggle`
- `go run ./cmd/coe trigger start`
- `go run ./cmd/coe trigger stop`
- `go run ./cmd/coe trigger status`

## Docs

- [`docs/README.md`](./docs/README.md)
- [`docs/architecture.md`](./docs/architecture.md)
- [`docs/fallbacks.md`](./docs/fallbacks.md)
- [`docs/gnome-globalshortcuts-matrix.md`](./docs/gnome-globalshortcuts-matrix.md)
- [`docs/pw-record-exit-status.md`](./docs/pw-record-exit-status.md)
- [`docs/roadmap.md`](./docs/roadmap.md)
