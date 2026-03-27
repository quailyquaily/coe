# Coe（聲）

[English](../README.md) | [简体中文](./README.zh-CN.md)

Coe は Linux 上で GNOME on Wayland 向けに動くディクテーションツールです。

これは [`missuo/koe`](https://github.com/missuo/koe) の Linux 向け再構成版です。目標は変わりません。ホットキーを押し、話し、LLM に文字起こしを整えさせ、そのテキストを今使っているアプリに戻します。

## 名前

`coe` が `koe` に近いのは意図的です。このプロジェクトは Koe へのオマージュですが、対象は Linux と Wayland です。古い漢字の `聲` は「声」を意味します。それがこのツールの仕事です。

## なぜ Coe か

Linux の音声入力ツールは、だいたい次のどれかで使いづらくなります。

- X11 時代の前提に依存している
- 基本設定が GUI の奥に隠れている
- Wayland のセキュリティモデルに合っていない

Coe は、もっと実用的な形を狙います。

- GNOME first、Wayland first
- バックグラウンドで動かし、UI 面をできるだけ小さくする
- 設定はプレーンな YAML
- portal clipboard、portal paste、デスクトップ通知のような既存機能を優先的に使う
- 降格経路をきちんと用意する

## 仕組み

実行フローは次の通りです。

1. `coe serve` を起動したままにします。
2. `coe trigger toggle` でディクテーションを開始します。現在は GNOME のカスタムショートカットから呼ばれます。`GlobalShortcuts` がない場合、Coe は起動時にそのショートカットを挿入し、存在を維持します。
3. `pw-record` でマイク入力を録音します。
4. 音声を外へ送る前に、ほぼ無音または明らかに壊れた録音を弾きます。
5. 音声を ASR に送ります。デフォルトは OpenAI Audio Transcriptions ですが、provider は差し替え可能です。
6. 必要なら、転写結果を OpenAI 互換のテキストモデルへ送り、整形します。
7. 整形したテキストをクリップボード経由で書き戻します。
8. 実行環境が許す場合、フォーカス中のアプリへそのまま貼り付けます。

補足:

- ASR: `whisper-cli` 経由のローカル `whisper.cpp` にも対応
- ASR: 外部の `SenseVoice` FastAPI にも対応可能
- LLM 整形: デフォルトでは `uniai` 経由の OpenAI 互換 Chat Completions。設定で Responses API にも切り替え可能
- 出力: portal clipboard と portal paste を優先し、`wl-copy` と `ydotool` を fallback として残す

## インストール

### 依存のインストール

実行時に必要なもの:

- Wayland セッション
- GNOME デスクトップ
- `pw-record`
- `wl-copy`
- `OPENAI_API_KEY`

キーは `~/.config/coe/env` に置いてもよいですし、`config.yaml` の `asr.api_key` と `llm.api_key` に直接書いてもかまいません。

任意:

- `ydotool`。コマンドラインの paste fallback を試したい場合
- `whisper-cli` と Whisper モデルファイル。ローカル ASR を使いたい場合
- 動作中の SenseVoice FastAPI サービス。SenseVoice 経由のローカルネットワーク ASR を使いたい場合

Ubuntu では、コマンドライン依存を次のように入れられます。

```bash
sudo apt update
sudo apt install -y pipewire-bin wl-clipboard
```

任意の paste fallback:

```bash
sudo apt install -y ydotool
```

### 事前ビルド済みパッケージをダウンロード

[Download](https://github.com/quailyquaily/coe/releases)

### またはソースからビルド

#### 前提

```bash
git clone https://github.com/quailyquaily/coe.git
cd coe
go build -o coe ./cmd/coe
```

## 実行

```bash
./coe serve
```

### ユーザー `systemd` サービスとしてインストール

現在の alpha を常駐するユーザーサービスとして入れるには:

```bash
./scripts/install.sh
```

スクリプトは、マシンのアーキテクチャに合った GitHub Release tarball をダウンロードしてから、次を入れます。

- `~/.local/bin/coe`
- `~/.config/systemd/user/coe.service`
- `~/.config/coe/env`
- `~/.local/share/gnome-shell/extensions/coe-focus-helper@mistermorph.com`

その後さらに:

- `coe doctor` を実行
- `coe.service` を再起動
- `coe.service` が active か確認
- バイナリ、設定、env、systemd unit、GNOME 拡張のインストール先を表示

バージョンを固定することもできます。

```bash
./scripts/install.sh v0.0.4
```

クラウド ASR や LLM provider を使う場合は、必要な API キーを `~/.config/coe/env` に書くか、`~/.config/coe/config.yaml` に直接書いてください。インストール後は一度ログアウトして再ログインしてください。GNOME Shell とユーザーサービスセッションの両方が新しい拡張をきれいに読み直せます。必要ならその後でサービスを再起動します。

```bash
systemctl --user restart coe.service
```

必要なら `~/.config/coe/env` を空のままにし、`~/.config/coe/config.yaml` の `asr.api_key` と `llm.api_key` に直接書いてもかまいません。

## ディクテーション開始・終了用ホットキー

- 名前: `coe-trigger`
- デフォルトショートカット: `<Shift><Super>d`
- GNOME fallback では、Coe が起動時に対応するカスタムショートカットを自動で揃えます

## 設定

Coe の設定はプレーンなファイルです。

設定ファイル:

- `~/.config/coe/config.yaml`
- リポジトリ例: [config.example.yaml](../config.example.yaml)

実行時状態:

- `XDG_STATE_HOME/coe/state.json`
- fallback: `~/.config/coe/state.json`

この state ファイルには portal restore token が保存されます。デスクトップ backend が許す場合、再承認の回数を減らすためです。

デフォルト設定を生成するには:

```bash
go run ./cmd/coe config init
```

`COE_CONFIG` で上書きしていない限り、`~/.config/coe/config.yaml` が作られます。

あるいは、リポジトリの例から始めてもかまいません。

```bash
cp config.example.yaml ~/.config/coe/config.yaml
```

現在のデフォルトは次の通りです。

### ASR

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
- `prompt` は初期プロンプトとして渡されます
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

### LLM 整形

- provider: `openai`
- endpoint type: `chat`
- endpoint: `https://api.openai.com/v1`
- model: `gpt-4o-mini`
- 直接キーを書くフィールド: `llm.api_key`
- 環境変数フィールド: `OPENAI_API_KEY`

OpenAI Responses API を使いたい場合は、`llm.endpoint_type` を `responses` にしてください。

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
- `show_text_preview: true`
- `notify_on_recording_start: false`

### Runtime

- `log_level: info`
- `log_level: debug` にすると各段階の所要時間や output fallback の詳細を出します
- 1 回だけ上書きするなら `coe serve --log-level debug`

GNOME focus-aware paste については次を参照してください。

- [config.example.yaml](../config.example.yaml)
- [gnome-focus-helper.md](./gnome-focus-helper.md)

新しく生成した設定では focus-aware paste はデフォルトで有効です。古い設定では、必要に応じて `output.use_gnome_focus_helper` を上書きできます。

## 現在の状態

動いているもの:

- [x] 自動管理される GNOME カスタムショートカットから `coe trigger toggle` を実行する GNOME Wayland fallback trigger
- [x] `pw-record` によるマイク録音
- [x] OpenAI Audio Transcriptions によるバッチ転写
- [x] 任意の SenseVoice FastAPI ASR provider
- [x] デフォルトでは OpenAI 互換 Chat Completions による整形。Responses も選択可能
- [x] portal clipboard による最終テキストの書き込み
- [x] portal のキーボード注入による自動貼り付け
- [x] GNOME デスクトップ通知
- [x] ほぼ無音の録音はローカルで短絡され、ASR へ送られない
- [x] クリップが激しい、または壊れた録音もローカルで短絡され、ASR へ送られない

まだないもの:

- [ ] `GlobalShortcuts` portal 対応
- [ ] KDE / Hyprland での検証
- [ ] 上流のマイク / PipeWire 飽和問題に対する強い結論

## その他

Portal 権限の持続:

- `persist_portal_access` が `true` なら、Coe は portal restore token をローカルに保存します
- 最初の承認が成功した後は、毎回再承認する代わりにその token の再利用を試みます
- GNOME や portal backend がその token を拒否した場合は、新しい承認フローに戻ります

システム通知:

- デフォルトで、完了と失敗に対して GNOME デスクトップ通知を送ります
- ほぼ無音や壊れた録音はローカルで報告され、ネットワーク転写は行いません
- 録音開始時の通知はデフォルトで無効です

## コマンド

- `go run ./cmd/coe doctor`
- `go run ./cmd/coe config init`
- `go run ./cmd/coe serve`
- `go run ./cmd/coe trigger toggle`
- `go run ./cmd/coe trigger start`
- `go run ./cmd/coe trigger stop`
- `go run ./cmd/coe trigger status`
- `go run ./cmd/coe version`

## ドキュメント

- [docs/README.md](./README.md)
- [docs/install.md](./install.md)
- [docs/architecture.md](./architecture.md)
- [docs/fallbacks.md](./fallbacks.md)
- [docs/gnome-globalshortcuts-matrix.md](./gnome-globalshortcuts-matrix.md)
- [docs/pw-record-exit-status.md](./pw-record-exit-status.md)
- [docs/roadmap.md](./roadmap.md)
