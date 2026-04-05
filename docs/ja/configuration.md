# 設定

[English](../configuration.md) | [简体中文](../zh/configuration.md)

Coe の設定はプレーンなファイルです。

## ファイル

設定ファイル:

- `~/.config/coe/config.yaml`
- リポジトリ例: [config.example.yaml](../../config.example.yaml)

実行時状態:

- `XDG_STATE_HOME/coe/state.json`
- fallback: `~/.config/coe/state.json`

この state ファイルには portal restore token が保存されます。デスクトップ backend が許す場合、再承認の回数を減らすためです。

## 初期化

デフォルト設定を生成するには:

```bash
go run ./cmd/coe config init
```

`COE_CONFIG` で上書きしていない限り、`~/.config/coe/config.yaml` が作られます。

あるいは、リポジトリの例から始めてもかまいません。

```bash
cp config.example.yaml ~/.config/coe/config.yaml
```

## 現在のデフォルト

### ホットキー

- デフォルトトリガーキー: `<Shift><Super>d`
- `coe hotkey pick` で変更できます
- デフォルトのトリガー動作: `hotkey.trigger_mode: toggle`
- `coe config set hotkey.trigger_mode toggle` または `coe config set hotkey.trigger_mode hold` で変更できます
- `hold` は押して録音開始、離して終了という意味で、`runtime.mode: fcitx` のときだけ有効です

### ASR

現在サポートしている provider:

| `asr.provider` | 配置形態 | 既定の endpoint / model | 補足 |
| --- | --- | --- | --- |
| `openai` | Hosted API | `https://api.openai.com/v1/audio/transcriptions` / `gpt-4o-mini-transcribe` | 現在のデフォルト。API キーが必要 |
| `whispercpp` | ローカル CLI | `whisper-cli` / ローカル `model_path` | `whisper.cpp` を直接使うオフライン経路 |
| `sensevoice` | セルフホスト HTTP | `http://127.0.0.1:50000/api/v1/asr` / なし | 公式 SenseVoice FastAPI サービスに接続 |
| `qwen3-asr-vllm` | セルフホスト OpenAI 互換 chat endpoint | `http://127.0.0.1:8000/v1/chat/completions` / `Qwen3-ASR` | WAV 音声を vLLM などの chat completions サーバーに送ります |

デフォルト profile:

- provider: `openai`
- endpoint: `https://api.openai.com/v1/audio/transcriptions`
- model: `gpt-4o-mini-transcribe`
- 直接キーを書くフィールド: `asr.api_key`
- 環境変数フィールド: `OPENAI_API_KEY`

ローカル `whisper.cpp` に切り替えるには:

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

補足:

- `binary` のデフォルトは `whisper-cli`
- `model_path` は `whisper.cpp` で必須
- `prompt` は Go の `text/template` としてレンダリングされたうえで初期プロンプトとして渡されます
- `prompt_file` を使うとテンプレートを別ファイルに置けます。相対パスは `config.yaml` から解決されます
- `threads` のデフォルトは `GOMAXPROCS`
- `use_gpu: false` は `--no-gpu` を付けます

SenseVoice FastAPI に切り替えるには:

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

補足:

- `endpoint` は公式 SenseVoice FastAPI サービスを指す必要があります
- `language` はサービスの `lang` フォームフィールドに対応します。例: `auto`, `zh`, `en`, `yue`, `ja`, `ko`
- Coe は毎回 1 つの WAV を送り、返ってきた `result` 配列の最初の要素を使います
- 公式リポジトリでは `uvicorn api:app --host 0.0.0.0 --port 50000` で起動すると、既定の URL は `http://127.0.0.1:50000/api/v1/asr` です

OpenAI 互換 chat endpoint 経由の Qwen3-ASR に切り替えるには:

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

補足:

- `endpoint` は `input_audio` を受け付ける OpenAI 互換 chat completions サーバーを指す必要があります
- `model` が空なら Coe は `Qwen3-ASR` にフォールバックします
- `api_key_env` の既定値は `OPENAI_API_KEY` です

### LLM 整形

- provider: `openai`
- endpoint type: `chat`
- endpoint: `https://api.openai.com/v1`
- model: `gpt-5.4-nano`
- 直接キーを書くフィールド: `llm.api_key`
- 環境変数フィールド: `OPENAI_API_KEY`

OpenAI Responses API を使いたい場合は、`llm.endpoint_type` を `responses` にしてください。
`llm.prompt` も Go の `text/template` としてレンダリングされたうえで補正指示に使われます。
`llm.prompt_file` も同じ仕組みで、テンプレートを YAML の外に置きたい場合はこちらを優先してください。

### Personal Dictionary

- 設定フィールド: `dictionary.file`
- 形式: `canonical`、`aliases`、任意の `scenes` を持つ YAML
- 文字列はダブルクォートで囲むのを推奨
- `aliases` は `["system control", "system c t l"]` のようなコンパクト配列で書けます
- 辞書は LLM correction prompt に注入され、さらに LLM 出力後に決定的な正規化をもう一度行います
- 1 文字 alias は prompt には入れず、コード側の厳格な token 境界置換だけで扱います
- v1 ではホットリロードしません。編集後は `coe restart` を実行してください
- `coe config init` は `config.yaml` と同じ場所に `./dictionary.yaml` を生成または補完し、2 つのスターター項目を入れます

例:

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
- 実行環境が portal を出していれば、clipboard と paste は portal を優先します
- `wl-copy` と `ydotool` はコマンドライン fallback として残ります
- 新しい設定では GNOME focus-aware paste がデフォルトで有効で、端末系ターゲットでは `Ctrl+V` から `Ctrl+Shift+V` に切り替えられます

### Notifications

- `enable_system: true`
- `notify_on_complete: false`
- `notify_on_recording_start: false`

`notify_on_complete` を有効にすると、完了通知に補正後のテキストが含まれます。

### Runtime

- `log_level: info`
- `log_level: debug` にすると各段階の所要時間や output fallback の詳細を出します
- 新規設定では `runtime.mode: fcitx` がデフォルトです。GNOME fallback を強制したい場合だけ `runtime.mode: desktop` に変更してください
- 1 回だけ上書きするなら `coe serve --log-level debug`

GNOME focus-aware paste については次を参照してください。

- [config.example.yaml](../../config.example.yaml)
- [gnome-focus-helper.md](../gnome-focus-helper.md)

新しく生成した設定では focus-aware paste はデフォルトで有効です。古い設定では、必要に応じて `output.use_gnome_focus_helper` を上書きできます。
