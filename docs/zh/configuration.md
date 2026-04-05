# 配置

[English](../configuration.md) | [日本語](../ja/configuration.md)

Coe 用纯文本文件管理配置。

## 文件

配置文件：

- `~/.config/coe/config.yaml`
- 仓库示例：[`config.example.yaml`](../../config.example.yaml)

运行时状态：

- `XDG_STATE_HOME/coe/state.json`
- fallback：`~/.config/coe/state.json`

这个 state 文件会保存 portal restore token。桌面 backend 支持时，可以减少重复授权提示。

## 初始化

生成默认配置：

```bash
go run ./cmd/coe config init
```

除非你用 `COE_CONFIG` 覆盖路径，否则它会写入 `~/.config/coe/config.yaml`。

或者直接从仓库示例开始：

```bash
cp config.example.yaml ~/.config/coe/config.yaml
```

## 当前默认值

### 热键

- 默认触发键：`<Shift><Super>d`
- 可以通过 `coe hotkey pick` 修改
- 默认触发行为：`hotkey.trigger_mode: toggle`
- 可以通过 `coe config set hotkey.trigger_mode toggle` 或 `coe config set hotkey.trigger_mode hold` 修改
- `hold` 表示按下开始录音、松开结束处理，并且只在 `runtime.mode: fcitx` 下生效

### ASR

当前支持的 provider：

| `asr.provider` | 部署方式 | 默认 endpoint / model | 说明 |
| --- | --- | --- | --- |
| `openai` | 云端 API | `https://api.openai.com/v1/audio/transcriptions` / `gpt-4o-mini-transcribe` | 当前默认值；需要 API key |
| `whispercpp` | 本地 CLI | `whisper-cli` / 本地 `model_path` | 通过 `whisper.cpp` 走离线路径 |
| `sensevoice` | 自托管 HTTP | `http://127.0.0.1:50000/api/v1/asr` / 无 | 对接官方 SenseVoice FastAPI 服务 |
| `qwen3-asr-vllm` | 自托管 OpenAI 兼容 chat endpoint | `http://127.0.0.1:8000/v1/chat/completions` / `Qwen3-ASR` | 把 WAV 音频发到兼容 chat completions 的服务，比如 vLLM |

默认 profile：

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
- `prompt` 会先按 Go `text/template` 渲染，再作为初始提示词传入
- `prompt_file` 可以把模板放进单独文件里；相对路径会按 `config.yaml` 所在目录解析
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

如果你要切到通过 OpenAI 兼容 chat endpoint 部署的 Qwen3-ASR：

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

说明：

- `endpoint` 应该指向支持 `input_audio` 的 OpenAI 兼容 chat completions 服务
- 如果 `model` 为空，Coe 会回退到 `Qwen3-ASR`
- `api_key_env` 默认是 `OPENAI_API_KEY`

### LLM 校正

- provider：`openai`
- endpoint type：`chat`
- endpoint：`https://api.openai.com/v1`
- model：`gpt-5.4-nano`
- 直接写 key 的字段：`llm.api_key`
- 环境变量字段：`OPENAI_API_KEY`

如果你想改走 OpenAI Responses API，可以把 `llm.endpoint_type` 设成 `responses`。
`llm.prompt` 也会先按 Go `text/template` 渲染，再作为校正指令使用。
`llm.prompt_file` 也是同样的机制；如果你想把模板放到 YAML 外面，优先用这个。

### 个人词典

- 配置字段：`dictionary.file`
- 文件格式：YAML，字段是 `canonical`、`aliases`，可选 `scenes`
- 字符串建议统一用双引号
- `aliases` 建议用紧凑数组语法，比如 `["system control", "system c t l"]`
- 词典会注入到 LLM correction prompt，并在 LLM 输出后再做一次确定性归一化
- 单字符 alias 不注入 prompt，只走程序里的严格 token 边界替换
- v1 不做热加载；修改词典后执行 `coe restart`
- `coe config init` 会在 `config.yaml` 同目录下创建或补齐 `./dictionary.yaml`，里面带两条起步示例

示例：

```yaml
dictionary:
  file: "./dictionary.yaml"
```

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
- `notify_on_complete: false`
- `notify_on_recording_start: false`

如果开启 `notify_on_complete`，完成通知会带上纠错后的文本。

### Runtime

- `log_level: info`
- 可以设成 `log_level: debug` 打印各阶段耗时和 output fallback 细节
- 新生成的配置默认就是 `runtime.mode: fcitx`；只有想强制走 GNOME fallback 时，才需要改成 `runtime.mode: desktop`
- 也可以单次覆盖：`coe serve --log-level debug`

关于 GNOME focus-aware paste，见：

- [config.example.yaml](../../config.example.yaml)
- [gnome-focus-helper.md](../gnome-focus-helper.md)

新生成的配置默认开启 focus-aware paste。旧配置如果需要，也可以手动覆盖 `output.use_gnome_focus_helper`。
