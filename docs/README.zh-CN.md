# Coe（聲）

[English](../README.md) | [日本語](./README.ja.md)

Coe 是一个 Linux 上面向 GNOME on Wayland 的听写工具。

它是对 [`missuo/koe`](https://github.com/missuo/koe) 的 Linux 向复刻。目标没有变：按下热键，说话，让 LLM 整理转写结果，再把文本放回当前应用。

## 名字

`coe` 故意和 `koe` 很接近。这个项目是在向 Koe 致意，但目标平台是 Linux 和 Wayland。日语汉字古字 `聲` 的意思就是声音，是这个工具要做的事。

## 为什么是 Coe？

多数 Linux 语音输入工具都不咋好用。Coe 想做好用：

- GNOME first，Wayland first
- 后台运行，尽量减少 UI 面
- 用纯 YAML 配置
- 优先复用别人的能力：portal clipboard、portal paste、桌面通知
- 提供降级路径

## 工作方式

运行流程如下：

1. 保持 `coe serve` 运行。
2. 用 `coe trigger toggle` 触发听写（目前由 GNOME 自定义快捷键调用；在没有 `GlobalShortcuts` 时，Coe 会在启动时插入这个自定义快捷键，自动确保这条快捷键存在）
3. 用 `pw-record` 录制麦克风输入。
4. 在音频离开本机前，先拦截接近静音或明显损坏的录音。
5. 把音频发送到 ASR。默认是 OpenAI Audio Transcriptions，但 provider 可配置。
6. 可选：把转写文本发送给一个 OpenAI 兼容文本模型做矫正。
7. 通过剪贴板路径写回修正后的文本。
8. 在运行环境允许时，把文本自动粘贴回当前聚焦应用。

备注：

- ASR：可选的本地 `whisper.cpp`，通过 `whisper-cli`
- ASR：可选的外部 `SenseVoice` FastAPI 服务
- LLM 校正：默认走 `uniai` 上的 OpenAI 兼容 Chat Completions，也可配置为 Responses API
- 输出：优先 portal clipboard 和 portal paste，`wl-copy` 与 `ydotool` 作为 fallback

## 安装

### 安装依赖

运行时依赖：

- Wayland 会话
- GNOME 桌面
- `pw-record`
- `wl-copy`
- `OPENAI_API_KEY`

你可以把 key 放在 `~/.config/coe/env`，也可以直接写进 `config.yaml` 里的 `asr.api_key` 和 `llm.api_key`。

可选依赖：

- `ydotool`，如果你想试命令行粘贴 fallback
- `whisper-cli` 和一个 Whisper 模型文件，如果你想用本地 ASR
- 一个正在运行的 SenseVoice FastAPI 服务，如果你想通过 SenseVoice 做本地网络 ASR

在 Ubuntu 上，可以这样安装命令行依赖：

```bash
sudo apt update
sudo apt install -y pipewire-bin wl-clipboard
```

可选的粘贴 fallback：

```bash
sudo apt install -y ydotool
```

### 下载预编译好的包

[下载](https://github.com/quailyquaily/coe/releases)

### 或者，从源码构建

#### 前置条件

```bash
git clone https://github.com/quailyquaily/coe.git
cd coe
go build -o coe ./cmd/coe
```

## 运行

```bash
./coe serve
```

### 安装为用户 systemd 服务

如果你想把当前 alpha 安装成常驻的用户级服务：

```bash
./scripts/install-user.sh
```

脚本会安装：

- `~/.local/bin/coe`
- `~/.config/systemd/user/coe.service`
- `~/.config/coe/env`
- `~/.local/share/gnome-shell/extensions/coe-focus-helper@quaily.com`

然后把你的 OpenAI key 填进 `~/.config/coe/env`，再重启服务：

```bash
systemctl --user restart coe.service
```

如果你愿意，也可以把 `~/.config/coe/env` 留空，直接把 key 写进 `~/.config/coe/config.yaml` 的 `asr.api_key` 和 `llm.api_key`。

## 热键开启和关闭听写

- 名称：`coe-trigger`
- 默认快捷键：`<Shift><Super>d`
- 在 GNOME fallback 模式下，Coe 会在启动时自动确保一条匹配的自定义快捷键

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
- model：`gpt-4o-mini`
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
- 也可以单次覆盖：`coe serve --log-level debug`

关于 GNOME focus-aware paste，见：

- [config.example.yaml](../config.example.yaml)
- [gnome-focus-helper.md](./gnome-focus-helper.md)

已有配置如果比较旧，可能还需要手动加上 `output.use_gnome_focus_helper: true`。

## 当前状态

已经工作的部分：

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

- [ ] `GlobalShortcuts` portal 支持
- [ ] KDE 或 Hyprland 的验证轮次
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
