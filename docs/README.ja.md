# Coe（聲）

[English](../README.md) | [简体中文](./README.zh-CN.md)

Coe は Linux デスクトップ向けの音声入力ツールです。

これは [`missuo/koe`](https://github.com/missuo/koe) への Linux 向けオマージュです。目標は変わりません。ホットキーを押し、話し、LLM に文字起こしを整えさせ、そのテキストを今使っているアプリに戻します。

## 名前

`coe` が `koe` に近いのは意図的です。このプロジェクトは Koe へのオマージュですが、対象は Linux と Wayland です。古い漢字の `聲` は「声」を意味します。それがこのツールの仕事です。

## なぜ Coe か

第一作者は Linux を使っていますが、いま Linux 向けのデスクトップソフトを作りたがる人は多くありません。だから Coe は次を目指します。

- バックグラウンドで動かし、UI 面をできるだけ小さくする
- 設定はプレーンな YAML
- fcitx、portal clipboard など、すでにある力を先に使う
- 制約の中でも、できるだけちゃんと音声入力を成立させる

## 仕組み

実行フローは次の通りです。

1. 通常は user-level `systemd` 経由で、`coe serve` をバックグラウンドで動かし続けます。
2. ホットキーでディクテーションを開始します。
   `runtime.mode: fcitx` では、Fcitx5 モジュールが D-Bus 経由で Coe を呼び、最終テキストを現在の input context に `CommitString` します。
   `runtime.mode: desktop` では、GNOME が custom keyboard shortcut で `coe trigger toggle` を呼びます。
3. `pw-record` でマイク入力を録音します。
4. ほぼ無音または明らかに壊れた録音は送らずに止めます。
5. 音声を ASR に送ります。OpenAI、SenseVoice、ローカル `whisper.cpp` に対応します。
6. 必要なら、転写結果を OpenAI 互換のテキストモデルへ送り、整形します。
7. 最終テキストを画面に出します。Fcitx 経由でそのまま入力するか、フォーカス中のアプリへ貼り付けます。

補足:

- LLM 整形: デフォルトでは OpenAI 互換 Chat Completions API を使い、必要なら OpenAI Responses API にも切り替え可能です
- 出力: `desktop` モードではまず GNOME portal clipboard を使い、無理なら `wl-copy` と `ydotool` に落とします

## デスクトップ統合の経路

現時点では二つの経路があります。

- `runtime.mode: fcitx`
  - 薄い Fcitx5 モジュール
  - ホットキーは Fcitx 内で処理
  - 最終テキストは `CommitString` でそのまま入力
  - 録音中と処理中だけ Fcitx パネルに小さな状態表示
- `runtime.mode: desktop`
  - GNOME-first のデスクトップ経路
  - `GlobalShortcuts` がない場合は GNOME custom shortcut fallback
  - portal clipboard / paste
  - terminal-aware paste のための GNOME Shell extension

## GNOME 専用の部分

現時点で GNOME 専用なのは次です。

- インストールスクリプトが GNOME Shell 拡張を入れて、focus-aware paste を有効にすること
- `GlobalShortcuts` が使えないとき、Coe が GNOME のカスタムショートカット fallback を自動管理すること
- focus-aware paste が、GNOME Shell 拡張から D-Bus 経由で `wm_class` を読むこと

GNOME 依存が比較的薄いのは、コアの音声入力パイプラインです。

- `pw-record` による録音
- OpenAI、`whisper.cpp`、SenseVoice による ASR
- LLM 整形
- そのデスクトップが許す範囲でのクリップボードと貼り付け

## インストール

### クイックインストール

一番簡単なのは release 用インストーラを使う方法です。

```bash
curl -fsSL -o /tmp/install.sh https://raw.githubusercontent.com/quailyquaily/coe/refs/heads/master/scripts/install.sh
bash /tmp/install.sh
```

これは、マシンのアーキテクチャに合った GitHub Release tarball をダウンロードします。`fcitx5` が入っていれば `fcitx` モードを優先し、なければ `desktop` モードに自動でフォールバックします。`--fcitx` で `fcitx` を強制でき、`--gnome` で `desktop` を強制できます。

ローカル開発では、GitHub Release から取得せずに `--bundle` でローカルの build 結果をそのままインストールすることもできます。`--bundle` は次のどちらも受け付けます。

- `./scripts/build-release-bundle.sh` が生成したローカル tarball
- 展開済み bundle ディレクトリ。例えば `dist/release/bundle-amd64`

例:

```bash
./scripts/build-release-bundle.sh dev
./scripts/install.sh --bundle ./dist/release/coe_dev_linux_amd64.tar.gz
```

その後、次を入れます。

- `~/.local/bin/coe`
- `~/.config/systemd/user/coe.service`
- `~/.config/coe/env`
- `fcitx5` があれば Fcitx5 モジュール
- `desktop` パスを使う場合だけ GNOME Shell 拡張

その後さらに:

- `coe doctor` を実行
- `coe.service` を再起動
- `coe.service` が active か確認
- バイナリ、設定、env、systemd unit、デスクトップ固有アセットのインストール先を表示

クラウド ASR や LLM provider を使う場合は、必要な API キーを `~/.config/coe/env` に書くか、`~/.config/coe/config.yaml` に直接書いてください。

`fcitx` モードを使う場合は、Fcitx パネルにも録音中と処理中の短い Coe 状態表示が出ます。

現在のパスが `desktop` なら、インストール後に一度ログアウトして再ログインしてください。GNOME Shell とユーザーサービスセッションの両方が新しい拡張をきれいに読み直せます。

そのあと入力欄のあるアプリを開き、デフォルトショートカット `<Shift><Super>d` を押して話し、もう一度押してください。うまくいけば、そのアプリに話した内容がテキストとして入ります。

### 依存のインストール

**`fcitx5` モード**

- `fcitx5`
- `pw-record`

**`desktop` モード**

- `pw-record`
- `wl-copy`

任意:

- クリップボード: `ydotool`。コマンドラインの paste fallback を試したい場合
- LLM: OpenAI 互換 API。必要なキーは `~/.config/coe/env` または `llm.api_key`
- ASR: `whisper-cli` と Whisper モデルファイル。ローカル ASR を使いたい場合
- ASR: 動作中の SenseVoice FastAPI サービス。SenseVoice 経由のローカルネットワーク ASR を使いたい場合
- ASR: OpenAI transcription。必要なキーは `~/.config/coe/env` または `asr.api_key`

Ubuntu では、コマンドライン依存を次のように入れられます。

```bash
sudo apt update
sudo apt install -y pipewire-bin wl-clipboard
sudo apt install -y ydotool
```

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

### ホットキー

- デフォルトトリガーキー: `<Shift><Super>d`

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

### LLM 整形

- provider: `openai`
- endpoint type: `chat`
- endpoint: `https://api.openai.com/v1`
- model: `gpt-5.4-nano`
- 直接キーを書くフィールド: `llm.api_key`
- 環境変数フィールド: `OPENAI_API_KEY`

`llm.prompt` も Go の `text/template` としてレンダリングされたうえで補正指示に使われます。
`llm.prompt_file` も同じ仕組みで、テンプレートを YAML の外に置きたい場合はこちらを優先してください。

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
- `notify_on_complete: false`
- `notify_on_recording_start: false`

`notify_on_complete` を有効にすると、完了通知に補正後のテキストが含まれます。

### Runtime

- `log_level: info`
- `log_level: debug` にすると各段階の所要時間や output fallback の詳細を出します
- Fcitx5 モジュール側で trigger path を引き受けたい場合は、`runtime.mode` を `fcitx` に設定してください
- 1 回だけ上書きするなら `coe serve --log-level debug`

GNOME focus-aware paste については次を参照してください。

- [config.example.yaml](../config.example.yaml)
- [gnome-focus-helper.md](./gnome-focus-helper.md)

新しく生成した設定では focus-aware paste はデフォルトで有効です。古い設定では、必要に応じて `output.use_gnome_focus_helper` を上書きできます。

## 現在の状態

動いているもの:

- [x] fcitx 5 モジュールによる他デスクトップ環境への互換
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

- `coe doctor`
- `coe config init`
- `coe serve`
- `coe trigger toggle`
- `coe trigger start`
- `coe trigger stop`
- `coe trigger status`
- `coe version`

## ドキュメント

- [docs/README.md](./README.md)
- [docs/install.md](./install.md)
- [docs/architecture.md](./architecture.md)
- [docs/fallbacks.md](./fallbacks.md)
- [docs/gnome-globalshortcuts-matrix.md](./gnome-globalshortcuts-matrix.md)
- [docs/pw-record-exit-status.md](./pw-record-exit-status.md)
- [docs/roadmap.md](./roadmap.md)
