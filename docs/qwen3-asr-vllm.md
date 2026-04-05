# Qwen3-ASR + vLLM + Coe

这篇文档说明如何在本机启动 `Qwen/Qwen3-ASR-1.7B` 的 vLLM 服务，并让 Coe 通过 `qwen3-asr-vllm` provider 调它做语音转写。

当前实现不是 OpenAI 的 `audio/transcriptions` 形状，而是：

- Coe 先把录到的 PCM 编码成 WAV
- 再把 WAV 做 base64
- 最后通过 OpenAI 兼容的 `/v1/chat/completions` 发送 `input_audio`

对应代码见 [internal/asr/qwen3_vllm.go](../internal/asr/qwen3_vllm.go)。

## 适用场景

- 你想在本机跑 Qwen3 ASR，不想把音频发到 OpenAI
- 你已经有可用的 NVIDIA CUDA 环境
- 你接受 vLLM 的资源占用和模型预热时间

## 1. 准备 Python 环境

在一个单独目录里准备 vLLM 环境：

```bash
uv venv
source .venv/bin/activate
```

安装 vLLM nightly 和音频依赖：

```bash
uv pip install -U vllm --pre \
    --extra-index-url https://wheels.vllm.ai/nightly/cu129 \
    --extra-index-url https://download.pytorch.org/whl/cu129 \
    --index-strategy unsafe-best-match

uv pip install "vllm[audio]"
```

如果你不是 CUDA 12.9 环境，需要把上面的 wheel 源换成你机器对应的 CUDA 版本。

## 2. 启动 Qwen3-ASR vLLM 服务

用户给出的最小可用命令如下：

```bash
vllm serve Qwen/Qwen3-ASR-1.7B \
  --gpu-memory-utilization 0.3 \
  --max-model-len 8192
```

默认监听地址一般是：

```text
http://127.0.0.1:8000
```

Coe 默认使用的完整 endpoint 是：

```text
http://127.0.0.1:8000/v1/chat/completions
```

如果你把 vLLM 绑定到了别的地址或端口，对应修改 Coe 的 `asr.endpoint` 即可。

## 3. 先单独验证 vLLM 服务

在接入 Coe 之前，建议先直接请求一次 `chat/completions`，确认 vLLM 端本身是通的。

可以先用公开的音频 URL 做一次最小验证：

```bash
curl http://localhost:8000/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{
    "messages": [
    {"role": "user", "content": [
        {"type": "audio_url", "audio_url": {"url": "https://qianwen-res.oss-cn-beijing.aliyuncs.com/Qwen3-ASR-Repo/asr_en.wav"}}
    ]}
    ]
    }'
```

如果这一步已经失败，不要先怀疑 Coe，先检查：

- `vllm serve` 进程是否真的在监听 `localhost:8000`
- 模型是否已经加载完成
- 当前 vLLM 版本是否支持你使用的音频输入格式

## 4. 配置 Coe

先初始化配置：

```bash
coe config init
```

然后编辑 `~/.config/coe/config.yaml`，把 `asr` 改成下面这样：

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

- `provider` 必须是 `qwen3-asr-vllm`
- `endpoint` 留空时，Coe 会默认回退到 `http://127.0.0.1:8000/v1/chat/completions`
- `model` 留空时，代码内部会默认成 `Qwen3-ASR`
- `prompt` 是可选项，会作为一段文本和音频一起发给 chat completions
- 如果你的 vLLM 没开鉴权，`api_key` 和 `api_key_env` 都可以留空

一个更完整的示例：

```yaml
runtime:
  mode: desktop
  target_desktop: gnome
  log_level: info

audio:
  recorder_binary: pw-record
  sample_rate: 16000
  channels: 1
  format: s16

asr:
  provider: qwen3-asr-vllm
  endpoint: http://127.0.0.1:8000/v1/chat/completions
  model: Qwen/Qwen3-ASR-1.7B
  language: ""
  prompt: ""
  api_key: ""
  api_key_env: OPENAI_API_KEY

llm:
  provider: stub
  endpoint_type: chat
  endpoint: ""
  model: ""
  api_key: ""
  api_key_env: ""
  prompt: ""
```

如果你想先验证纯 ASR 效果，建议先把 `llm.provider` 设成 `stub`，避免把 ASR 和后处理问题混在一起。

## 5. 启动 Coe

前台调试：

```bash
coe serve --log-level debug
```

或者如果你已经装成了用户服务：

```bash
coe restart
journalctl --user -u coe.service -f
```

## 6. 验证链路

最小验证步骤：

1. 确认 vLLM 进程已经启动，并且在监听 `127.0.0.1:8000`
2. 运行 `coe doctor`，确认录音和桌面集成依赖都正常
3. 启动 `coe serve --log-level debug`
4. 触发一次听写，说一句短文本，再停止录音
5. 观察 Coe 日志里是否出现对 `qwen3-asr-vllm` 的请求和返回

如果配置正确，最终你应该看到：

- Coe 成功录音
- Coe 把 WAV 音频发到 vLLM chat completions
- 返回文本被直接上屏，或者再经过 LLM 清洗后上屏

## 7. 常见问题

### vLLM 能启动，但 Coe 返回空文本

先看 Coe debug 日志。当前实现里如果 vLLM 返回：

- `choices` 为空
- `message.content` 为空
- 返回结构不是预期的 chat completions 形状

Coe 会把它当成空转写或 warning。

相关实现见 [internal/asr/qwen3_vllm.go](../internal/asr/qwen3_vllm.go) 和测试 [internal/asr/qwen3_vllm_test.go](../internal/asr/qwen3_vllm_test.go)。

### 返回文本前面带 `<asr_text>`

Coe 已经会自动去掉尾部结果里的 `<asr_text>` 前缀标记。

### 想加提示词

直接写 `asr.prompt` 即可，例如：

```yaml
asr:
  provider: qwen3-asr-vllm
  endpoint: http://127.0.0.1:8000/v1/chat/completions
  model: Qwen/Qwen3-ASR-1.7B
  prompt: 请直接输出转写文本，不要解释，不要补充说明。
```

### 显存不够

先从更保守的参数开始：

```bash
vllm serve Qwen/Qwen3-ASR-1.7B \
  --gpu-memory-utilization 0.2 \
  --max-model-len 4096
```

### 想确认是不是 Coe 配置错了

先把 `llm.provider` 设成 `stub`，只保留 ASR；
再确认 `asr.provider`、`asr.endpoint`、`asr.model` 三个字段都和上面示例一致。

## 8. 建议

- 先让 `vllm serve` 单独稳定运行，再接 Coe
- 第一次联调时把 `llm.provider` 设成 `stub`
- 先验证短句转写，再调 prompt、清洗和自动粘贴
- 如果你是 Arch 用户，先看 [arch-install.md](./arch-install.md) 把 Coe 和桌面依赖装好
