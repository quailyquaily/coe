# Coe（聲）

[English](../README.md) | [日本語](./README.ja.md)

Coe 是一个 Linux 桌面上的语音输入工具。

它是对 [`missuo/koe`](https://github.com/missuo/koe) 的 Linux 向致敬。目标没有变：按下热键，说话，让 LLM 整理转写结果，再把文本放回当前应用。

## 名字

`coe` 故意和 `koe` 很接近（发音也一样）。日语汉字古字 `聲` 的意思就是声音，是这个工具要做的事。

## 为什么是 Coe？

第一作者用的是 Linux，但现在大家不太喜欢给 Linux 开发桌面软件。所以，第一作者希望 Coe 可以：

- 后台运行，纯 YAML 配置，尽量减少 UI 
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

## 安装

### 快速安装

最简单的方式是直接用 release 安装脚本：

```bash
curl -fsSL -o /tmp/install.sh https://raw.githubusercontent.com/quailyquaily/coe/refs/heads/master/scripts/install.sh
bash /tmp/install.sh
```

它会下载与你机器架构匹配的 GitHub Release tarball。如果系统里已经装了 `fcitx5`，它会优先走 `fcitx` 模式；否则会自动 fallback 到 `desktop` 模式。

安装完成以后需要编辑 `~/.config/coe/config.yaml`，至少要配置其中的 `asr` 和 `llm` 章节，具体请参考[docs/zh/configuration.md](./zh/configuration.md)。

如果当前在 GNOME Shell 下，安装完成后先注销再登录一次，让 GNOME Shell 使用 Coe 扩展。

然后打开一个有输入焦点的 App，按下默认快捷键：`<Shift><Super>d`，说话，再按一次，稍等片刻，如果正常的话，会看到说的话变成文字出现在这个 App。

### Arch Linux

```bash
yay -S coe-git
```

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

## 配置

Coe 的配置是纯文件。

配置文件：

- `~/.config/coe/config.yaml`
- 仓库示例：[config.example.yaml](../config.example.yaml)

生成默认配置：

```bash
go run ./cmd/coe config init
```

它会写入 `~/.config/coe/config.yaml`。

完整配置说明见 [docs/zh/configuration.md](./zh/configuration.md)。

快速摘要：

- 默认热键：`<Shift><Super>d`
- 默认热键行为：`hotkey.trigger_mode: toggle`，即按一次开始听写，再按一次结束听写。另有可选值为 `hold`，即长按热键开始听写，释放热键结束听写（只在 `runtime.mode: fcitx` 下生效）
- 支持的 ASR provider：`openai`、`whispercpp`、`sensevoice`、`qwen3-asr-vllm`
- LLM 校正：支持所有 openai 兼容 API 的上游模型

## 桌面集成

当前有两条集成路径：

- `runtime.mode: fcitx`：由 fcitx 处理热键、上屏、听写处理状态。
- `runtime.mode: desktop`：由 `GlobalShortcuts` 或者  GNOME custom shortcut fallback 处理热键，由 portal clipboard / paste 处理上屏

**GNOME 专属的部分**

安装脚本会安装 GNOME Shell 扩展，用于获取当前焦点的窗口，通过 D-Bus 暴露当前聚焦窗口的 `wm_class`，Coe 需要用它判断目标 App 是普通 App 还是一个 Terminal App

## 当前状态

已经工作的部分：

- [x] 通过 fcitx 5 模块实现对其他桌面环境的兼容
- [x] GNOME Wayland fallback trigger：通过自动管理的 GNOME 自定义快捷键执行 `coe trigger toggle`
- [x] 通过 `pw-record` 录制麦克风
- [x] LLM 转写，去除重复词、语气词
- [x] SenseVoice FastAPI 作为 ASR provider
- [x] GNOME 桌面通知
- [x] 过滤静音或者损坏的的录音
- [x] 内置的基础场景 

还没有的部分：

- [ ] 对上游麦克风 / PipeWire 饱和问题的更强结论
- [ ] 自定义指令
- [ ] 自定义场景以及场景切换

## 其他

Portal 权限持久化：

- 如果 `persist_portal_access` 为 `true`，Coe 会把 portal restore token 存到本地
- 第一次授权成功后，后续运行会尽量复用这个 token，而不是每次都重新弹窗

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

- [docs/zh/development.md](./zh/development.md)
- [docs/zh/configuration.md](./zh/configuration.md)
- [docs/README.md](./README.md)
- [docs/install.md](./install.md)
- [docs/arch-install.md](./arch-install.md)
- [docs/architecture.md](./architecture.md)
- [docs/fallbacks.md](./fallbacks.md)
- [docs/gnome-globalshortcuts-matrix.md](./gnome-globalshortcuts-matrix.md)
- [docs/qwen3-asr-vllm.md](./qwen3-asr-vllm.md)
