# Configuration

[简体中文](./zh/configuration.md) | [日本語](./ja/configuration.md)

Coe uses plain files for configuration.

## Files

Config file:

- `~/.config/coe/config.yaml`
- repo example: [`config.example.yaml`](../config.example.yaml)

Runtime state:

- `XDG_STATE_HOME/coe/state.json`
- fallback: `~/.config/coe/state.json`

The state file stores the portal restore token so repeated authorization prompts can be reduced when the desktop backend supports persistence.

## Initialize

Create the default config with:

```bash
go run ./cmd/coe config init
```

That writes `~/.config/coe/config.yaml`, unless you override the path with `COE_CONFIG`.

Or start from the repo example:

```bash
cp config.example.yaml ~/.config/coe/config.yaml
```

## Current Defaults

### Hotkey

- default trigger key: `<Shift><Super>d`
- change it with `coe hotkey pick`
- default trigger behavior: `hotkey.trigger_mode: toggle`
- change it with `coe config set hotkey.trigger_mode toggle` or `coe config set hotkey.trigger_mode hold`
- `hold` means press to start recording and release to stop, and only takes effect in `runtime.mode: fcitx`

### ASR

Supported providers:

| `asr.provider` | Deployment | Default endpoint / model | Notes |
| --- | --- | --- | --- |
| `openai` | Hosted API | `https://api.openai.com/v1/audio/transcriptions` / `gpt-4o-mini-transcribe` | Current default; requires API key |
| `whispercpp` | Local CLI | `whisper-cli` / local `model_path` | Offline path through `whisper.cpp` |
| `sensevoice` | Self-hosted HTTP | `http://127.0.0.1:50000/api/v1/asr` / none | Talks to the official SenseVoice FastAPI service |
| `qwen3-asr-vllm` | Self-hosted OpenAI-compatible chat endpoint | `http://127.0.0.1:8000/v1/chat/completions` / `Qwen3-ASR` | Sends WAV audio to a chat-completions server such as vLLM |

Default profile:

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

To switch to Qwen3-ASR over an OpenAI-compatible chat endpoint:

```yaml
asr:
  provider: qwen3-asr-vllm
  endpoint: http://127.0.0.1:8000/v1/chat/completions
  model: Qwen/Qwen3-ASR-1.7B
  language: ""
  prompt: ""
  api_key: ""
  api_key_env: OPENAI_API_KEY
```

Notes:

- `endpoint` should point at an OpenAI-compatible chat completions server that accepts `input_audio`
- if `model` is empty, Coe falls back to `Qwen3-ASR`
- `api_key_env` defaults to `OPENAI_API_KEY`

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
- v1 does not hot-reload the dictionary; run `coe restart` after editing it
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

- [`config.example.yaml`](../config.example.yaml)
- [`gnome-focus-helper.md`](./gnome-focus-helper.md)

New configs enable focus-aware paste by default. Older configs can still override `output.use_gnome_focus_helper` if needed.
