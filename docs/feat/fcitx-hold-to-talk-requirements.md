# Coe Fcitx hold-to-talk 需求与约束

## 0. 文档角色

这份文档只定义一件事：

- `runtime.mode: fcitx` 下的 trigger 语义应该是什么

为避免文档之间再出现冲突，这里明确分工：

- 本文定义目标需求、状态机约束和边界行为
- [fcitx5-module-design.md](./fcitx5-module-design.md) 负责架构与职责边界
- [fcitx5-implementation-plan.md](./fcitx5-implementation-plan.md) 负责落地顺序
- [../../packaging/fcitx5/README.md](../../packaging/fcitx5/README.md) 只描述当前 shipped 行为，不代表目标设计已经落地

## 1. 背景

当前 `Coe` 的 `fcitx` 路径已经具备这些能力：

- Fcitx5 module 在 `PreInputMethod` 监听触发键
- 通过 session D-Bus 调 `Coe` daemon
- daemon 产出最终文本
- module 把文本 `CommitString` 到当前焦点 input context

但当前 shipped 语义仍是 `toggle`：

- 按一次开始录音
- 再按一次结束录音并进入处理

现有设计文档也明确把 `hold-to-talk` 排除在 v1 外，原因是先验证 “Fcitx 触发 + 上屏” 主链，再处理 press/release 语义。

现在要新增的需求是：

- **按住触发键开始录音**
- **松开触发键结束录音并进入处理**

这个能力是 `fcitx` 路径特有的，因为当前只有 Fcitx module 稳定拿到了输入法层的 key press / key release 事件。

## 2. 为什么这次需要单独文档

这个需求建议先写到 `docs/feat/`，原因不是“实现量大”，而是“行为语义会变”：

1. 它直接推翻了当前 `fcitx` 设计里的 `toggle only` 约束。
2. 当前实现明确忽略了 `key release`，需要重新定义事件状态机。
3. `hold-to-talk` 会引入 auto-repeat、丢焦点、异常 stop、跨 source 并发触发等边界。
4. 这项能力是 `fcitx` 特有的，不应在未定义清楚之前污染通用 hotkey 语义。

所以这里不是一次性的前置调研，而是一份需要长期维护的**trigger 语义约束文档**。

## 3. 目标

这个需求的目标是：

1. 在 `runtime.mode: fcitx` 下支持更自然的按住说话体验。
2. 复用现有 daemon `Start()` / `Stop()` D-Bus 方法，不重做主链。
3. 保持当前 Fcitx 提交路径不变，仍由 module 向当前焦点 input context 提交最终文本。
4. 把新行为限制在 `fcitx` 路径内，不影响桌面 fallback 的 `toggle` 热键行为。

## 4. v1 范围

v1 建议只做这些：

- 只在 `runtime.mode: fcitx` 生效
- Fcitx module 收到 trigger key `press` 时调用 `Start()`
- Fcitx module 收到同一触发键 `release` 时调用 `Stop()`
- 继续沿用现有 panel hint / `StateChanged` / `ResultReady` 信号
- 继续把最终文本提交到“结果返回时的当前焦点 input context”

v1 明确不做：

- GNOME 或其他桌面路径的 hold-to-talk
- preedit
- 部分识别结果流式上屏
- 鼠标按住按钮说话
- 跨输入法统一的一套 trigger abstraction

## 5. 推荐的 v1 语义

### 5.1 基本行为

用户在有 Fcitx input context 的文本框里：

1. 按下触发键
2. `Coe` 开始录音
3. 用户持续按住并说话
4. 松开触发键
5. `Coe` 停止录音并开始 ASR / correction
6. 最终文本提交到当前焦点 input context

### 5.2 只消费一次 press 和一次 release

按住期间可能会出现键盘 auto-repeat。

v1 语义应是：

- 第一个匹配的 `press` 才会触发 `Start()`
- 后续 repeat press 全部忽略
- 只有与这次 hold 对应的第一次 `release` 才会触发 `Stop()`

否则会出现：

- 重复 start
- 过早 stop
- 状态机错乱

### 5.3 release 不要求原输入上下文仍然存在

录音开始后，用户可能：

- 切换窗口
- 关闭原输入框
- 暂时失去焦点

这里推荐保持和现有 Fcitx 提交策略一致：

- `release` 负责结束本次录音，不要求原始 input context 仍然存在
- 最终提交目标仍按“结果返回时的当前焦点 input context”决定

### 5.4 保持桌面 fallback 语义不变

`hold-to-talk` 是 `fcitx` 专用行为。

当前桌面 fallback 的 `hotkey.preferred_accelerator` 仍应继续表达：

- 一个通用触发键
- 默认 `toggle` 语义

不要在 v1 里把 GNOME / portal / custom shortcut 也一起改成按住说话。

## 6. 配置建议

这个需求虽然只在 `fcitx` 路径生效，但仍建议有显式配置，不要直接把现有用户的 `toggle` 行为静默改掉。

推荐配置模型：

```yaml
hotkey:
  trigger_mode: toggle
```

允许值：

- `toggle`
- `hold`

推荐理由：

1. 现有用户已经习惯 `toggle`
2. `hold` 和 `toggle` 都只属于 `fcitx` module 的触发语义
3. 比布尔型 `hold_to_talk: true` 更容易扩展

如果后续确认 `hold` 明显更优，再考虑把默认值从 `toggle` 调整为 `hold`。

## 7. 状态机要求

实现前至少要先锁定下面这组状态机语义。

### 7.1 module 本地状态

Fcitx module 侧建议至少维护：

- `idle`
- `holding`
- `waiting_release_cleanup` 或等价布尔状态

需要避免的问题：

- `press` 后 `Start()` 失败，但模块仍错误等待 `release`
- `release` 到来时 daemon 已不是 recording 状态
- 用户快速重复点按导致 press/release 交错

### 7.2 daemon 状态复用

daemon 已有：

- `Start()`
- `Stop()`
- `Toggle()`
- `Status()`

因此 v1 不需要重做 IPC 协议，但需要明确：

1. `Start()` 在已录音状态下的返回语义
2. `Stop()` 在未录音状态下的返回语义
3. module 是否需要在 `press` 前先读一次 `Status()`

推荐 v1 做法：

- module 以本地 hold 状态为主
- daemon 保持幂等
- 仅在异常恢复时查询 `Status()`

## 8. 边界行为

### 8.1 快速点按

如果用户只是快速按下又立刻松开：

- v1 仍然按一次极短录音处理
- 不额外引入“低于 N 毫秒自动取消”的规则

原因：

- 能先保持行为简单
- 取消阈值会引入更多 UX 争议

如果后续短录音噪声明显，再单独加 `minimum_hold_ms` 之类的策略。

### 8.2 中途被其他 source 停止

如果本次录音由 `fcitx hold` 开始，但中途被：

- CLI
- 其他 IPC source
- 未来的桌面触发源

提前 stop 了，则 module 在后续 `release` 到来时应静默 no-op，不要报错打扰用户。

### 8.3 焦点丢失

如果用户按住期间窗口失焦：

- 不应导致录音卡死
- `release` 仍应尽量完成 stop

如果 Fcitx 没有把预期 `release` 事件送回来，需要设计兜底策略，例如：

- 在收到 daemon 状态变回 `idle` 后自动清空 hold 状态
- 或在 Fcitx input context 生命周期变化时清理本地状态

### 8.4 输入法自身按键冲突

需要确认触发键在 `PreInputMethod` 阶段被消费后：

- 不会继续把 modifier / 普通键插入到文本框
- 不会和常见输入法热键语义冲突

这也是这份文档存在的原因之一：这里不是纯实现细节，而是用户可感知行为。

## 9. 实现建议

基于现状，代码层面的最小变更应是：

1. Fcitx module 从“只看 `press` 调 `Toggle()`”改成“`press` 调 `Start()`，`release` 调 `Stop()`”
2. 增加 module 本地 hold 状态，过滤 auto-repeat
3. 给 daemon / module 增加覆盖 press/release 语义的测试
4. 若引入 `hotkey.trigger_mode`，补 config / doctor / README

不建议一开始就做：

- 新 D-Bus signal
- 复杂 cancel 语义
- 大范围 runtime abstraction 重构

## 10. 验证标准

至少覆盖以下验证场景：

1. 正常按住说话，松开后得到文本并成功上屏
2. 长按期间 auto-repeat 不会触发多次 start
3. daemon 已经 active 时再次 press 不会打乱状态
4. `press -> start failed -> release` 不会留下脏状态
5. 录音期间切焦点，结果仍提交到结果返回时的新焦点
6. 快速点按不会崩溃或卡住 module

## 11. 结论

这个需求**值得先写文档**，而且应该放在 `docs/feat/`。

但它不需要一套很重的“大而全调研报告”。更合适的文档分工是：

1. 本文负责 `trigger_mode`、异常语义、边界行为
2. `fcitx5-module-design.md` 负责模块和 daemon 的职责划分
3. `fcitx5-implementation-plan.md` 负责分阶段实现
4. `packaging/fcitx5/README.md` 继续只写当前 shipped 事实

也就是说，这个需求目前最合适的文档形态不是“泛泛调研”，而是“针对现有 fcitx 设计差异的需求与状态机约束说明”。
