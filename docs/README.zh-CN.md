# Coe（聲）

[English](../README.md) | [日本語](./README.ja.md)

Coe 是一个 Linux 桌面上的语音输入工具。

它是对 [`missuo/koe`](https://github.com/missuo/koe) 的 Linux 向致敬。目标没有变：按下热键，说话，让 LLM 整理转写结果，再把文本放回当前应用。

## 名字

`coe` 故意和 `koe` 很接近（发音也一样）。日语汉字古字 `聲` 的意思就是声音，是这个工具要做的事。

## 为什么是 Coe？

第一作者用的是 Linux，但现在大家不太喜欢给 Linux 开发桌面软件。所以，第一作者希望 Coe 可以：

- 后台运行，尽量减少 UI 面
- 用纯 YAML 配置
- 优先复用别人的能力：fcitx、portal clipboard, etc
- 尽量在限制下把语音输入做好。

## 工作方式

运行流程如下：

1. 保持 `coe serve` 后台运行，默认使用 user level systemd 。
2. 用热键触发听写。
   在 `runtime.mode: fcitx` 下，Fcitx5 模块会通过 D-Bus 调 Coe，并把最终文本直接 `CommitString` 到当前输入上下文。
   在 `runtime.mode: desktop` 下，GNOME 会通过 custom keyboard shortcut 来执行 `coe trigger toggle`。
3. 用 `pw-record` 录制麦克风输入。
4. 拦截接近静音或明显损坏的录音，不发送。
5. 把音频发送到 ASR。支持 OpenAI, SenseVoice, 或者本地 Whisper.cpp。
6. 可选流程：把 ASR 转写文本发送给 LLM 文本模型做矫正。
7. 输出上屏：要么通过 Fcitx 直接上屏，要么把文本自动粘贴回当前焦点的 App。

备注：

- LLM 校正：默认支持所有 OpenAI 兼容 Chat Completion API，也可配置为 OpenAI Responses API
- 输出上屏：desktop 模式下，优先使用 Gnome portal clipboard；不可用时，使用 `wl-copy` 与 `ydotool` 作为 fallback

## 桌面集成路径

当前有两条集成路径：

- `runtime.mode: fcitx`
  - 极薄的 Fcitx5 module
  - 热键在 Fcitx 里处理
  - 最终文本通过 `CommitString` 直接上屏
  - 录音和处理中会在 Fcitx panel 里显示一个很小的状态提示
- `runtime.mode: desktop`
  - GNOME-first 的桌面路径
  - `GlobalShortcuts` 不可用时走 GNOME custom shortcut fallback
  - portal clipboard / paste
  - 通过 GNOME Shell extension 做 terminal-aware paste

## GNOME 专属的部分：

- 安装脚本会安装 GNOME Shell 扩展，用于获取当前焦点的窗口，通过 D-Bus 暴露当前聚焦窗口的 `wm_class`，Coe 需要用它判断目标 App 是普通 App 还是一个 Terminal App
- 当 `GlobalShortcuts` 不可用时，Coe 会自动管理 GNOME 自定义快捷键

## 安装

### 快速安装

最简单的方式是直接用 release 安装脚本：

```bash
curl -fsSL -o /tmp/install.sh https://raw.githubusercontent.com/quailyquaily/coe/refs/heads/master/scripts/install.sh
bash /tmp/install.sh
```

它会下载与你机器架构匹配的 GitHub Release tarball。如果系统里已经装了 `fcitx5`，它会优先走 `fcitx` 模式；否则会自动 fallback 到 `desktop` 模式。你也可以用 `--fcitx` 强制走 `fcitx`，或者用 `--gnome` 强制走 `desktop`。

然后安装：

- `~/.local/bin/coe`
- `~/.config/systemd/user/coe.service`
- `~/.config/coe/env`
- 如果检测到 `fcitx5`，安装 Fcitx5 模块
- 只有在走 `desktop` 模式时，才安装 GNOME Shell 扩展

安装完以后还会：

- 运行一次 `coe doctor`
- 重启 `coe.service`
- 检查 `coe.service` 是否处于 active
- 打印二进制、配置、env、systemd unit，以及桌面相关资源的安装位置

如果你使用云端 ASR 或 LLM provider，把需要的 API key 填进 `~/.config/coe/env`，或者直接写进 `~/.config/coe/config.yaml`。

如果你使用 `fcitx` 模式，Fcitx panel 还会在录音和处理中显示一条简短的 Coe 状态提示。

如果当前走的是 `desktop` 模式，安装完成后先注销再登录一次，让 GNOME Shell 和用户级服务会话都干净地拿到新扩展。

然后打开一个有输入焦点的 App，按下默认快捷键：`<Shift><Super>d`，说话，再按一次，稍等片刻，如果正常的话，会看到说的话变成文字出现在这个 App。

### 安装依赖


**`fcitx5` 模式**

- fcitx5
- `pw-record`

**`desktop` 模式**

运行时依赖：

- `pw-record`
- `wl-copy`

在 Ubuntu 上，可以这样安装命令行依赖：

```bash
sudo apt update
sudo apt install -y pipewire-bin wl-clipboard
sudo apt install -y ydotool
```

**可选依赖**

- LLM：一个 OpenAI 兼容 API 的 LLM 进行文本纠正，把需要的 API key 放在 `~/.config/coe/env` 或者 `config.yaml` 里的 `llm.api_key`
- ASR：`whisper-cli` 和一个 Whisper 模型文件，如果你想用本地 ASR
- ASR：一个正在运行的 SenseVoice FastAPI 服务，如果你想通过 SenseVoice 做本地网络 ASR
- ASR：OpenAI 提供的 transcribe 服务，把需要的 API key 放在 `~/.config/coe/env` 或者 `config.yaml` 里的 `asr.api_key`

## 配置

Coe 的配置是纯文件。

配置文件：

- `~/.config/coe/config.yaml`
- 仓库示例：[config.example.yaml](../config.example.yaml)

运行时状态：

- `XDG_STATE_HOME/coe/state.json`
- fallback：`~/.config/coe/state.json`

这个 state 文件会存储 portal restore token，用来在桌面后端支持时减少重复授权弹窗。

生成默认配置：

```bash
go run ./cmd/coe config init
```

它会写入 `~/.config/coe/config.yaml`，除非你用 `COE_CONFIG` 覆盖了路径。

或者直接从仓库示例开始：

```bash
cp config.example.yaml ~/.config/coe/config.yaml
```

当前默认值如下：

### 热键

- 默认触发键：`<Shift><Super>d`

### ASR

- provider：`openai`
- endpoint：`https://api.openai.com/v1/audio/transcriptions`
- model：`gpt-4o-mini-transcribe`
- 直接写 key 的字段：`asr.api_key`
- 环境变量字段：`OPENAI_API_KEY`

如果你要切到本地 `whisper.cpp`：

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

说明：

- `binary` 默认是 `whisper-cli`
- `model_path` 是 `whisper.cpp` 必填项
- `prompt` 会作为初始提示词传入
- `threads` 默认取 `GOMAXPROCS`
- `use_gpu: false` 会加上 `--no-gpu`

如果你要切到 SenseVoice FastAPI：

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

说明：

- `endpoint` 指向官方 SenseVoice FastAPI 服务
- `language` 会映射到服务的 `lang` 表单字段，比如 `auto`、`zh`、`en`、`yue`、`ja`、`ko`
- Coe 每次上传一个 WAV 文件，并使用返回 `result` 数组里的第一条文本
- 官方仓库用 `uvicorn api:app --host 0.0.0.0 --port 50000` 启动后，默认服务地址就是 `http://127.0.0.1:50000/api/v1/asr`

### LLM 校正

- provider：`openai`
- endpoint type：`chat`
- endpoint：`https://api.openai.com/v1`
- model：`gpt-5.4-nano`
- 直接写 key 的字段：`llm.api_key`
- 环境变量字段：`OPENAI_API_KEY`

如果你想改走 OpenAI Responses API，可以把 `llm.endpoint_type` 设成 `responses`。

### Audio

- recorder：`pw-record`
- sample rate：`16000`
- channels：`1`
- format：`s16`

### Output

- clipboard：`wl-copy`
- 如果运行环境暴露了 portal，剪贴板和粘贴会优先走 portal
- `wl-copy` 和 `ydotool` 保留为命令行 fallback
- 新配置默认开启 GNOME focus-aware paste，可在终端类目标里从 `Ctrl+V` 切到 `Ctrl+Shift+V`

### Notifications

- `enable_system: true`
- `show_text_preview: true`
- `notify_on_recording_start: false`

### Runtime

- `log_level: info`
- 可以设成 `log_level: debug` 打印各阶段耗时和 output fallback 细节
- 如果要让 Fcitx5 模块接管触发路径，而不是走 GNOME fallback，请把 `runtime.mode` 设成 `fcitx`
- 也可以单次覆盖：`coe serve --log-level debug`

关于 GNOME focus-aware paste，见：

- [config.example.yaml](../config.example.yaml)
- [gnome-focus-helper.md](./gnome-focus-helper.md)

新生成的配置默认开启 focus-aware paste。旧配置如果需要，也可以手动覆盖 `output.use_gnome_focus_helper`。

## 当前状态

已经工作的部分：

- [x] 通过 fcitx 5 模块实现对其他桌面环境的兼容
- [x] GNOME Wayland fallback trigger：通过自动管理的 GNOME 自定义快捷键执行 `coe trigger toggle`
- [x] 通过 `pw-record` 录制麦克风
- [x] 通过 OpenAI Audio Transcriptions 做批量转写
- [x] 可选的 SenseVoice FastAPI ASR provider
- [x] 默认通过 OpenAI 兼容 Chat Completions 做文本清洗，也支持 Responses
- [x] 通过 portal clipboard 写回最终文本
- [x] 通过 portal 键盘注入自动粘贴最终文本
- [x] GNOME 桌面通知
- [x] 接近静音的录音会在本地短路，不再发给 ASR
- [x] 严重削波或损坏的录音会在本地短路，不再发给 ASR

还没有的部分：

- [ ] 对上游麦克风 / PipeWire 饱和问题的更强结论

## 其他

Portal 权限持久化：

- 如果 `persist_portal_access` 为 `true`，Coe 会把 portal restore token 存到本地
- 第一次授权成功后，后续运行会尽量复用这个 token，而不是每次都重新弹窗
- 如果 GNOME 或 portal backend 拒绝旧 token，Coe 会回退到重新授权

系统通知：

- 默认会对“听写完成”和“失败”发 GNOME 桌面通知
- 接近静音或损坏的录音会在本地被报告并跳过网络转写
- 默认不在“开始录音”时发通知

## 命令

- `coe doctor`
- `coe config init`
- `coe serve`
- `coe trigger toggle`
- `coe trigger start`
- `coe trigger stop`
- `coe trigger status`
- `coe version`

## 文档

- [docs/README.md](./README.md)
- [docs/install.md](./install.md)
- [docs/architecture.md](./architecture.md)
- [docs/fallbacks.md](./fallbacks.md)
- [docs/gnome-globalshortcuts-matrix.md](./gnome-globalshortcuts-matrix.md)
- [docs/pw-record-exit-status.md](./pw-record-exit-status.md)
- [docs/roadmap.md](./roadmap.md)
