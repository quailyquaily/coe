# Coe Fcitx5 Module 实现计划

## 1. 目标

把 [fcitx5-module-design.md](./fcitx5-module-design.md) 里的设计落成一条可交付的实现路线。

这份文档回答三个问题：

1. 先做什么
2. 哪些代码复用现有 `Coe`
3. 每一步怎么验证

## 2. 实施原则

### 2.1 先通最小主链

先打通：

- Fcitx 热键
- D-Bus 调 daemon
- daemon 返回文本
- module `CommitString`

不要一开始就做：

- preedit
- hold-to-talk
- 自动启动 daemon
- 复杂 UI

### 2.2 不重写现有 pipeline

现有 `Coe` 的这些部分应尽量原样复用：

- `internal/audio`
- `internal/asr`
- `internal/llm`
- `internal/pipeline`
- `internal/config`
- `internal/output/portal_state.go`

Fcitx 集成应该只新增：

- 新触发源
- 新 IPC
- 新 output target

### 2.3 先从“能用”再到“顺手”

v1 标准：

- 在 Fcitx 输入上下文中按热键
- 录音
- 拿到文本
- 提交到当前焦点输入框

只要这条链稳定，就算成立。

## 3. 代码改动面

## 3.1 Go 侧

建议新增目录：

- `internal/ipc/dbus/`
- `internal/session/`
- `internal/output/targets/` 或同等级抽象

预计涉及现有文件：

- `internal/app/app.go`
- `internal/pipeline/orchestrator.go`
- `internal/output/output.go`
- `internal/config/config.go`
- `cmd/coe/main.go`

### 3.2 Fcitx5 侧

建议新增目录：

- `packaging/fcitx5/`
  - `src/`
  - `cmake/` 如有需要
  - `CMakeLists.txt`
  - addon 配置文件
  - 安装说明

## 4. 分阶段计划

## Phase 1: 守护进程 D-Bus 化

目标：

- 让 `Coe daemon` 暴露一个最小可用 D-Bus 接口
- 先不接 Fcitx，也能用 `gdbus` 手工联调

### 4.1 新增 D-Bus service

新增：

- service: `com.mistermorph.Coe`
- path: `/com/mistermorph/Coe`
- interface: `com.mistermorph.Coe.Dictation1`

### 4.2 最小 methods

先实现：

- `Toggle()`
- `Status() -> (state, session_id, detail)`

`Start()` / `Stop()` 可以第二步补，不必和 `Toggle()` 一起首发。

### 4.3 最小 signals

先实现：

- `StateChanged(state, session_id, detail)`
- `ResultReady(session_id, text)`
- `ErrorRaised(session_id, message)`

`ResultCommitted` 可以后补。

### 4.4 daemon 内部改造

需要补一层会话对象，至少包含：

- `session_id`
- `source`
- `state`
- `result_text`
- `error`

`source` 先只要支持：

- `external-trigger`
- `fcitx-module`

### 4.5 输出策略调整

当前 daemon 是“自己最终负责输出”。  
Fcitx 路线要改成“有时只产出文本，不直接 paste”。

建议增加 output mode：

- `desktop`
- `fcitx`

规则：

- `desktop`: 走现有 portal / clipboard / paste
- `fcitx`: 不执行桌面 paste，只发 `ResultReady`

### 4.6 验证方式

阶段通过标准：

1. `coe serve` 启动后可以在 session bus 上看到 service
2. `gdbus call` 可以触发 `Toggle()`
3. daemon 会发 `StateChanged`
4. 完成后能收到 `ResultReady`

建议命令：

```bash
gdbus call --session \
  --dest com.mistermorph.Coe \
  --object-path /com/mistermorph/Coe \
  --method com.mistermorph.Coe.Dictation1.Toggle
```

## Phase 2: Fcitx5 module 骨架

目标：

- 让 Fcitx addon 能被编译、加载、注册热键

### 5.1 先搭极薄 module

内容只包括：

- addon 元数据
- module 类
- 初始化与销毁
- 最小日志

不要在这一步写 D-Bus。

### 5.2 注册热键

先注册一个固定热键，对应：

- 默认 `<Shift><Super>d`

注意：

- 这一步只要能在 Fcitx 内接到热键事件
- 不需要一开始就做完整配置界面

### 5.3 获取当前 input context

最小要求：

- 触发时确认存在当前 input context
- 没有时不向 daemon 发请求

### 5.4 验证方式

阶段通过标准：

1. module 能被 Fcitx 加载
2. 热键触发时可以打印可见日志
3. 没有输入上下文时不会崩

## Phase 3: module 接 D-Bus

目标：

- Fcitx module 能驱动 daemon
- daemon 能把结果回给 module

### 6.1 module 调 `Toggle()`

热键触发时：

- 如果 module 本地状态是 `Idle`，调用 `Toggle()`
- 如果是 `Recording`，再次调用 `Toggle()`

### 6.2 监听 daemon signal

module 订阅：

- `StateChanged`
- `ResultReady`
- `ErrorRaised`

### 6.3 module 本地状态机

最小状态：

- `Idle`
- `Recording`
- `Processing`

同步规则：

- 调 `Toggle()` 后不立刻信任本地状态
- 以 daemon 的 `StateChanged` 为准

这样可以避免双方状态漂移。

### 6.4 验证方式

阶段通过标准：

1. 在 Fcitx 输入框里按热键可以开始录音
2. 再按一次可以结束录音
3. module 能收到 `ResultReady`

## Phase 4: CommitString

目标：

- 让最终文本进入当前焦点输入框

### 7.1 提交逻辑

收到 `ResultReady` 时：

1. 重新取当前焦点 input context
2. 如果存在，直接 `CommitString`
3. 如果不存在，走 fallback

### 7.2 fallback 逻辑

v1 建议：

- fallback 到剪贴板

方式有两种：

1. module 自己写剪贴板
2. module 调 daemon 请求 desktop fallback

推荐第 2 种：

- 复用 daemon 已有 clipboard/output 能力
- Fcitx module 保持够薄

所以 Phase 4 需要补一个 daemon method：

- `FallbackToClipboard(text)` 或更窄的 `RequestDesktopFallback(session_id)`

我推荐：

- `RequestDesktopFallback(session_id)`

理由：

- module 不需要知道输出细节
- daemon 继续掌握输出策略

### 7.3 验证方式

阶段通过标准：

1. 普通文本框里能直接上屏
2. 中途切换到另一个输入框时，结果进入新焦点
3. 焦点消失时，文本至少能落到剪贴板

## Phase 5: 配置与构建

目标：

- 能稳定构建和安装

### 8.1 daemon 配置

配置新增建议：

```yaml
fcitx:
  enabled: true
  dbus_service: com.mistermorph.Coe
  use_current_focus_context: true
  fallback_to_clipboard: true
```

是否一定要加 `fcitx.enabled`：

- 不一定
- 如果 daemon 总是暴露 D-Bus，也可以不加

我的建议：

- v1 先不加 `enabled`
- 默认总是暴露这套接口

### 8.2 Fcitx module 构建

需要明确：

- 依赖哪些 `fcitx5` 开发包
- CMake 怎么找 `Fcitx5Core`
- 生成什么产物

### 8.3 安装方式

后续应该补到发布体系里，但不是 v1 blocker。

v1 先做到：

- 本地构建
- 手动安装

之后再考虑：

- 打包到 release tarball
- 和 `install.sh` 集成

## Phase 6: 回归验证

## 9.1 必测场景

1. 当前输入法为英文键盘，触发 Coe，文本可上屏
2. 当前输入法为中文拼音，触发 Coe，文本可上屏
3. 当前输入法为日文输入法，触发 Coe，文本可上屏
4. 录音时切换输入框，结果进入新焦点
5. 录音结束前输入框关闭，结果 fallback 到剪贴板
6. daemon 不在线，module 不崩溃
7. ASR 返回空文本，module 能恢复到 `Idle`
8. daemon 报错，module 能恢复到 `Idle`

### 9.2 可延后测试

- 多显示器
- 多 seat
- X11 会话
- 非 GNOME 桌面 + Fcitx 组合

## 10. 风险清单

### 10.1 Fcitx API 学习成本

风险：

- C++ addon 学习曲线高于 Lua

控制方式：

- 保持 module 足够薄
- 不在 module 内做业务逻辑

### 10.2 状态漂移

风险：

- module 认为正在录音，但 daemon 实际已经失败

控制方式：

- 以 daemon signal 为准
- 本地状态只做 UI 辅助

### 10.3 提交目标不一致

风险：

- 用户以为会打回原输入框，实际打到了新焦点

控制方式：

- 文档明确规则
- v1 固定为“提交到当前焦点”

### 10.4 剪贴板 fallback 体验不一致

风险：

- 没有输入上下文时，用户需要手动粘贴

控制方式：

- 这是接受的降级路径
- 不把它隐藏成“看起来成功”

## 11. 推荐落地顺序

如果只按最短路径开工，推荐顺序是：

1. Phase 1: daemon D-Bus
2. Phase 2: Fcitx module skeleton
3. Phase 3: module <-> daemon 通信
4. Phase 4: `CommitString`
5. Phase 6: 回归测试
6. Phase 5: 打包和安装

这个顺序故意把“打包”和“安装脚本”放后面。  
因为现在最大的未知数不是发布，而是 Fcitx 提交链路本身。

## 12. 当前建议

下一步代码实现时，不要一口气同时碰：

- daemon D-Bus
- module 热键
- module 提交
- 安装脚本

建议按最小增量拆 commit：

1. daemon D-Bus skeleton
2. Fcitx module skeleton
3. toggle 联通
4. `CommitString`
5. clipboard fallback
6. packaging

这样每一步都能单独验证，也更容易回退。
