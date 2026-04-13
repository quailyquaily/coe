# Coe（聲）

[English](../README.md) | [简体中文](./README.zh-CN.md)

Coe は Linux デスクトップ向けの音声入力ツールです。

これは [`missuo/koe`](https://github.com/missuo/koe) への Linux 向けオマージュです。目標は変わりません。ホットキーを押し、話し、LLM に文字起こしを整えさせ、そのテキストを今使っているアプリに戻します。

## デモ

![Coe screencast on Linux desktop](../docs/screencast.gif)

デモ提供: [@ilovesusu](https://github.com/ilovesusu)

## 名前

`coe` が `koe` に近いのは意図的です。このプロジェクトは Koe へのオマージュですが、対象は Linux と Wayland です。古い漢字の `聲` は「声」を意味します。それがこのツールの仕事です。

## なぜ Coe か

第一作者は Linux を使っていますが、いま Linux 向けのデスクトップソフトを作りたがる人は多くありません。だから Coe は次を目指します。

- バックグラウンドで動かし、設定はプレーンな YAML にし、UI をできるだけ小さくする
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
5. 音声を ASR に送ります。`openai`、`doubao`、`sensevoice`、ローカル `whisper.cpp`、`voxtype`、`qwen3-asr-vllm` に対応します。
6. 必要なら、転写結果を OpenAI 互換のテキストモデルへ送り、整形します。
7. 最終テキストを画面に出します。Fcitx 経由でそのまま入力するか、フォーカス中のアプリへ貼り付けます。

## インストール

### クイックインストール

一番簡単なのは release 用インストーラを使う方法です。

```bash
curl -fsSL -o /tmp/install.sh https://raw.githubusercontent.com/quailyquaily/coe/refs/heads/master/scripts/install.sh
bash /tmp/install.sh
```

これは、マシンのアーキテクチャに合った GitHub Release tarball をダウンロードします。`fcitx5` が入っていれば `fcitx` モードを優先し、なければ `desktop` モードに自動でフォールバックします。

インストール後は `~/.config/coe/config.yaml` を編集し、少なくとも `asr` と `llm` のセクションを設定してください。詳しくは [docs/ja/configuration.md](./ja/configuration.md) を参照してください。

GNOME Shell 上で使う場合は、インストール後に一度ログアウトして再ログインし、GNOME Shell が Coe 拡張を読み込めるようにしてください。

そのあと入力欄のあるアプリを開き、デフォルトショートカット `<Shift><Super>d` を押して話し、もう一度押してください。うまくいけば、そのアプリに話した内容がテキストとして入ります。

別のショートカットにしたい場合は `coe hotkey pick` を使えます。

### Arch Linux

```bash
yay -S coe-git
```

### 依存のインストール

**`fcitx5` モード**

- `fcitx5`
- `pw-record`

**`desktop` モード**

- `pw-record`
- `wl-copy`

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

デフォルト設定を生成するには:

```bash
go run ./cmd/coe config init
```

`~/.config/coe/config.yaml` が作られます。

完全な設定リファレンスは [docs/ja/configuration.md](./ja/configuration.md) を参照してください。

Doubao クラウド ASR の設定手順は [docs/doubao-asr.md](./doubao-asr.md) を参照してください。

要点だけ先に書くと:

- デフォルトホットキー: `<Shift><Super>d`
- デフォルトのホットキー動作は `hotkey.trigger_mode: toggle` です。1 回押して開始、もう 1 回押して終了します。オプションの `hold` は押して開始、離して終了で、`runtime.mode: fcitx` のときだけ有効です
- サポートする ASR provider: `openai`, `doubao`, `whispercpp`, `sensevoice`, `voxtype`, `qwen3-asr-vllm`
- LLM 整形は OpenAI 互換 API 配下の上流モデルをサポートします

## デスクトップ統合

現在は二つの統合経路があります。

- `runtime.mode: fcitx`: ホットキー、テキスト入力、ディクテーション状態は fcitx が扱います
- `runtime.mode: desktop`: `GlobalShortcuts` または GNOME custom shortcut fallback がホットキーを扱い、portal clipboard / paste が出力を担当します

**GNOME 専用の部分**

インストールスクリプトは GNOME Shell 拡張も入れます。この拡張はフォーカス中ウィンドウの `wm_class` を D-Bus 経由で公開し、Coe はそれを使って通常アプリか端末系ターゲットかを判定します。

## 現在の状態

動いているもの:

- [x] fcitx 5 モジュールによる他デスクトップ環境への互換
- [x] 自動管理される GNOME カスタムショートカットから `coe trigger toggle` を実行する GNOME Wayland fallback trigger
- [x] `pw-record` によるマイク録音
- [x] LLM による重複語やフィラーの整形
- [x] ASR provider としての SenseVoice FastAPI
- [x] GNOME デスクトップ通知
- [x] 無音または破損録音のフィルタリング
- [x] 組み込みの基本シーン

まだないもの:

- [ ] 上流のマイク / PipeWire 飽和問題に対する強い結論
- [ ] カスタム指示
- [ ] カスタムシーンとシーン切り替え

## その他

Portal 権限の持続:

- `persist_portal_access` が `true` なら、Coe は portal restore token をローカルに保存します
- 最初の承認が成功した後は、毎回再承認する代わりにその token の再利用を試みます

## コマンド

- `coe doctor`
- `coe config init`
- `coe hotkey pick`
- `coe restart`
- `coe serve`
- `coe trigger toggle`
- `coe trigger start`
- `coe trigger stop`
- `coe trigger status`
- `coe version`

## ドキュメント

- [docs/ja/development.md](./ja/development.md)
- [docs/ja/configuration.md](./ja/configuration.md)
- [docs/README.md](./README.md)
- [docs/install.md](./install.md)
- [docs/architecture.md](./architecture.md)
- [docs/fallbacks.md](./fallbacks.md)
- [docs/gnome-globalshortcuts-matrix.md](./gnome-globalshortcuts-matrix.md)
- [docs/doubao-asr.md](./doubao-asr.md)
- [docs/qwen3-asr-vllm.md](./qwen3-asr-vllm.md)
