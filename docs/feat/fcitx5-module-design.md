# Coe Fcitx5 Module 设计草案

## 1. 目标

为 `Coe` 增加一条新的输入路径：通过 `Fcitx5` 在当前输入上下文里触发语音输入，并在结果返回后把文本直接上屏。

这条路径的目标不是替代现有的 GNOME Wayland daemon 方案，而是提供一个更贴近输入法工作流的集成方式：

1. 用户当前继续使用自己已有的输入法。
2. 在任何有 Fcitx 输入上下文的文本框里触发 `Coe`。
3. `Coe` 录音、转写、校正。
4. 最终文本由 Fcitx 直接 `CommitString` 到当前焦点输入框。

v1 范围明确如下：

- Fcitx5 `Module`，不是独立输入法引擎
- `toggle` 触发，不做 hold-to-talk
- 不做 preedit UI
- 结果默认提交到当前焦点 input context
- 如果当前没有可提交目标，则 fallback 到剪贴板

## 2. 为什么不是独立输入法

`Coe` 不应该做成一个新的 Fcitx 输入法引擎。

原因：

1. 用户不应该为了语音输入先切换到另一个输入法。
2. 语音输入更像 `QuickPhrase` 或 `Unicode` 这类“在当前输入法之上临时触发的子模式”。
3. `Coe` 的核心价值不在候选词和输入状态，而在“获取一段文本然后提交”。
4. 做成 `Module` 后，用户继续保留当前拼音、日文、韩文或英文输入法，不会打断原有习惯。

因此，设计结论是：

- `Coe daemon` 负责思考
- `Fcitx5 Module` 负责触发和上屏

## 3. 总体架构

系统拆成两个进程：

### 3.1 Coe daemon

继续沿用当前 Go 进程。

职责：

- 音频录制
- 本地音频有效性检测
- ASR
- LLM 校正
- 输出结果和状态广播

新增职责：

- 暴露一个面向 Fcitx 的 D-Bus 接口
- 接收 `toggle/start/stop` 指令
- 对外发布状态变化和最终文本

### 3.2 fcitx5-coe Module

新增一个极薄的 Fcitx5 C++ module。

职责：

- 注册触发热键
- 只在当前有输入上下文时响应
- 通过 D-Bus 调用 daemon 开始或停止录音
- 监听 daemon 状态和结果
- 将最终文本提交到当前焦点 input context
- 如果无法提交，则走模块内 fallback

## 4. 为什么必须这样拆

### 4.1 稳定性

录音、网络调用、模型推理都不应该跑在 Fcitx 进程里。

如果守护进程卡死：

- Fcitx 仍然可以正常输入
- 守护进程可以独立重启
- 问题边界更清楚

### 4.2 调试效率

`Coe daemon` 可以完全脱离 Fcitx 单独调试：

- 音频链路
- ASR provider
- LLM provider
- 错误处理
- 性能日志

最后只需要把“触发”和“提交字符串”接到 Fcitx 上。

### 4.3 复用现有代码

当前仓库里这部分已经存在并可复用：

- `internal/audio`
- `internal/asr`
- `internal/llm`
- `internal/pipeline`
- `internal/config`
- `internal/output/portal_state.go`

新的 Fcitx 集成不应该重写这些逻辑。

## 5. Fcitx 侧形态

### 5.1 选择 Module，而不是 Input Method Engine

推荐实现为 `Module`，不是 `InputMethod`。

Module 模式下，`Coe` 是“当前输入法上的一个语音动作”。

这意味着：

- 用户不需要切换到 `Coe`
- 当前输入法保持不变
- `Coe` 只在有输入上下文时介入

### 5.2 用户体验

用户当前可能正在用：

- 拼音
- 五笔
- Mozc
- Hangul
- US keyboard

此时按下 `Coe` 触发键：

1. Fcitx module 调 daemon 开始录音
2. 用户说话
3. 再按一次触发键停止
4. 结果直接提交到当前焦点输入框

对用户来说，这不是“切换输入法”，而是“当前输入法多了一个语音动作”。

## 6. 触发模型

v1 只做 `toggle`。

即：

- 第一次触发：开始录音
- 第二次触发：结束录音并进入转写

不做 hold-to-talk 的理由：

1. 当前 `Coe` 现有 fallback 也是 toggle，复用最多
2. Fcitx 内部做 press/release 语义更容易踩边界
3. 先证明“Fcitx 提交路径”成立，比先追求输入体验更重要

后续如果需要再研究 press/release。

## 7. 提交策略

这里明确采用：

**结果提交给当前焦点 input context，而不是录音开始时的原始 input context。**

原因：

1. 对用户来说更直观
2. 用户在说话过程中切换输入框是合理行为
3. 语音输入更像“把这段话输入到现在所在的位置”

行为规则：

1. 触发开始时，只要求当下存在 Fcitx 输入上下文
2. 结果返回时，模块重新查询当前焦点 input context
3. 如果存在，则 `CommitString`
4. 如果不存在，则 fallback 到剪贴板

这条规则的 tradeoff：

- 优点：符合用户直觉
- 缺点：结果可能被送到录音开始后切换到的新位置

当前认为这是合理的默认行为。

## 8. UI 设计

v1 不做 preedit。

原因：

1. preedit 会引入额外状态同步和 UI 细节
2. `[正在听...]` 这种临时文字不应通过真实字符提交再删除
3. 先保证“触发 -> 录音 -> 上屏”是稳的

v1 的 UI 反馈只保留：

- Fcitx 自己的可选状态图标或状态文本
- daemon 侧日志
- 必要时桌面通知

v2 再考虑：

- panel 提示
- preedit 提示
- 错误态可视化

## 9. D-Bus 协议

推荐新增一个专门的 D-Bus 接口，而不是复用当前 Unix socket。

原因：

1. Fcitx module 需要异步收状态和结果
2. D-Bus 自带 method 和 signal，协议更清晰
3. 调试方便，可用 `gdbus` 或 `busctl`
4. 后续扩展状态字段更自然

### 9.1 service / path / interface

- service: `com.mistermorph.Coe`
- path: `/com/mistermorph/Coe`
- interface: `com.mistermorph.Coe.Dictation1`

### 9.2 methods

- `Toggle()`
- `Start()`
- `Stop()`
- `Status() -> (state, session_id, detail)`

### 9.3 signals

- `StateChanged(state, session_id, detail)`
- `ResultCommitted(session_id, text)`
- `ResultReady(session_id, text)`
- `ErrorRaised(session_id, message)`

### 9.4 状态枚举

- `idle`
- `recording`
- `processing`
- `completed`
- `error`

### 9.5 说明

`ResultReady` 用于 Fcitx module 获取最终文本。  
`ResultCommitted` 保留给 daemon 自己的现有输出路径或未来统计用途。  
如果最终决定由 Fcitx module 全权负责上屏，则 `ResultCommitted` 在 Fcitx 路径里可以不消费。

## 10. 输出职责划分

引入 Fcitx module 后，输出会变成两类：

### 10.1 原有桌面输出路径

适用于当前 GNOME daemon 路线：

- portal clipboard
- portal paste
- `wl-copy`
- `ydotool`

### 10.2 Fcitx 提交路径

适用于 Fcitx module 路线：

- module 收到最终文本
- 查询当前焦点 input context
- `CommitString`

因此需要新增一个运行模式判断：

- 如果当前请求来自 Fcitx module，daemon 不应再自己执行桌面 paste
- 最终输出由 module 负责

换句话说：

**Fcitx 集成不是新增一种 ASR 模式，而是新增一种 output target。**

## 11. 守护进程侧需要新增的东西

### 11.1 D-Bus server

新增一个 Fcitx-facing D-Bus server，负责：

- 接收 toggle/start/stop
- 广播状态
- 广播结果

### 11.2 会话状态机

当前 daemon 已经有录音和 pipeline 状态，但需要明确抽成对外可观察的会话状态。

建议新增：

- `session_id`
- `source`
- `state`
- `started_at`
- `completed_at`

其中 `source` 至少区分：

- `external-trigger`
- `fcitx-module`

### 11.3 输出分流

当 `source=fcitx-module` 时：

- daemon 负责产出文本
- daemon 不直接执行 portal paste
- daemon 通过 D-Bus 把文本发给 module

fallback 规则：

- 如果 module 明确请求 `clipboard_fallback`
- 或者 module 在提交失败后回调请求
- 再由 daemon 执行剪贴板写入

## 12. Fcitx module 状态机

推荐最小状态机：

- `Idle`
- `Recording`
- `Processing`

流程：

1. `Idle`
   - 触发热键
   - 如果当前有 input context，则调用 `Toggle()` 并进入 `Recording`
2. `Recording`
   - 再次触发热键
   - 调用 `Toggle()` 并进入 `Processing`
3. `Processing`
   - 忽略新的开始请求，或给出轻量提示
   - 等待 `ResultReady` 或 `ErrorRaised`
4. 收到 `ResultReady`
   - 查询当前焦点 input context
   - 成功 `CommitString` 后回到 `Idle`
   - 若失败，则 fallback 到剪贴板后回到 `Idle`
5. 收到 `ErrorRaised`
   - 回到 `Idle`

## 13. 失败与降级策略

### 13.1 daemon 不在线

模块行为：

- 不崩溃
- 可以尝试自动启动 daemon，或先只提示错误

v1 建议：

- 先只报错，不做自动拉起

### 13.2 当前没有 input context

模块行为：

- 不触发录音
- 直接忽略或轻量提示

### 13.3 结果返回时没有 input context

模块行为：

- 走剪贴板 fallback

### 13.4 当前焦点发生变化

模块行为：

- 结果提交到新的当前焦点

### 13.5 daemon 报错

模块行为：

- 回到 `Idle`
- 给出最小可见反馈

## 14. 与现有 GNOME 路线的关系

这不是替换关系，而是并存关系。

现有路线：

- GNOME custom shortcut
- daemon 录音
- daemon 输出到 portal clipboard / paste

新增路线：

- Fcitx module 热键
- daemon 录音
- module `CommitString`

两条路线可以共享：

- 音频层
- ASR 层
- LLM 层
- pipeline
- 配置

真正新增的是：

- 新的触发源
- 新的输出目标
- 新的 IPC

## 15. 代码布局建议

### 15.1 Go 侧

建议新增：

- `internal/ipc/dbus/`
- `internal/session/`
- `internal/output/fcitx.go` 或新的 output target 抽象

### 15.2 Fcitx5 侧

建议新增：

- `packaging/fcitx5/`
  - `src/`
  - `CMakeLists.txt`
  - addon 配置文件
  - 安装说明

## 16. 里程碑

### M1: 设计冻结

- 确认 `Module` 路线
- 确认 D-Bus 协议
- 确认输出责任边界

### M2: daemon 暴露 D-Bus

- 实现 `Toggle/Start/Stop/Status`
- 实现状态与结果 signal
- 用 `gdbus` 手工联调

### M3: Fcitx5 module 最小可用

- 注册热键
- 调 daemon
- 收结果
- `CommitString`

### M4: 错误和降级

- daemon 不在线
- 当前无 input context
- fallback 到剪贴板

### M5: 打包与文档

- Fcitx5 module 构建
- 安装说明
- 与现有安装脚本的关系

## 17. 非目标

v1 不做：

- Lua 版本
- 独立输入法引擎
- hold-to-talk
- preedit UI
- 候选词或流式识别面板
- 跨桌面统一的全局热键

## 18. 当前结论

这条路线是可行的，而且比“继续在 Wayland 上硬凿全局热键和自动粘贴”更贴近输入法场景。

最重要的结论有三个：

1. `Coe` 应该是 `Fcitx5 Module`，不是独立输入法
2. daemon 继续负责录音、ASR、LLM；module 只负责触发和提交
3. 结果默认提交给当前焦点 input context；没有目标时再 fallback 到剪贴板

这三个点一旦成立，后续实现路径就是清晰的。
