# Coe 场景功能实现计划

## 1. 目标

把 [scene-requirements.md](./scene-requirements.md) 里的产品行为拆成一条可落地的实现路线。

这份文档回答四个问题：

1. v1 先做什么
2. 哪些部分复用现有 `Coe`
3. 场景状态、场景切换和场景化校正怎么接进当前链路
4. 每一步怎么验证

## 2. 已锁定的 v1 决策

实现阶段以这些决策为前提，不再重复讨论：

- 只做两个内置场景：`general` 和 `terminal`
- 场景状态只保存在 daemon 内存中
- 只通过语音命令切换场景，不做自动识别
- 场景路由复用当前 LLM provider / model
- v1 只让场景影响 `LLM correction`
- `terminal` 场景优先保留命令名、路径、flag、环境变量、包名、文件名、版本号
- 切换成功时发短通知，展示名需要本地化
- 不做“查询当前场景”语音命令
- 场景状态至少要通过 D-Bus 暴露

## 3. 设计原则

### 3.1 不重写现有主链

当前仓库已有一条稳定的主链：

- `internal/audio`
- `internal/asr`
- `internal/llm`
- `internal/pipeline`
- `internal/app`
- `internal/ipc/dbus`
- `internal/prompts`
- `internal/i18n`

场景功能应尽量复用这条主链，只在必要位置插入：

- 运行时场景状态
- 场景切换门控
- 场景路由请求
- 场景化 correction prompt 选择

### 3.2 先做“显式切换”，不做“智能猜测”

v1 的价值在于：

- 用户可以显式切到 `terminal`
- 后续 correction 行为立即变
- 用户能看到明确反馈

不要在 v1 里引入：

- 前台应用识别
- 窗口标题识别
- 自动切换策略

### 3.3 控制命令和普通听写分离

“切换场景”本质上是控制命令，不是普通文本输出。

实现时必须保证：

- 场景切换成功后，这次语音不再进入普通输出链路
- 只有路由失败时，才回退成普通听写

## 4. 代码改动面

## 4.1 建议新增目录

- `internal/scene/`
  - `catalog.go`
  - `state.go`
  - `gate.go`
  - `router.go`
  - `types.go`

这个目录负责：

- 内置场景定义
- 当前场景状态
- 切换门控
- 场景路由 JSON 请求和解析

## 4.2 预计新增模板

- `internal/prompts/templates/scene-router.tmpl`
- `internal/prompts/templates/llm-correction-general.tmpl`
- `internal/prompts/templates/llm-correction-terminal.tmpl`

说明：

- `scene-router.tmpl` 用于“切换场景”路由
- `llm-correction-general.tmpl` 对应默认通用 correction
- `llm-correction-terminal.tmpl` 对应命令行保真 correction

如果实现时希望保留现有 `llm-correction.tmpl` 作为兼容入口，也可以让它作为 `general` 的别名，但 v1 主路径建议明确分开。

## 4.3 预计涉及现有文件

- `internal/app/app.go`
- `internal/app/bootstrap.go`
- `internal/app/runtime.go`
- `internal/app/notifications.go`
- `internal/app/ipc.go`
- `internal/ipc/dbus/dictation.go`
- `internal/pipeline/orchestrator.go`
- `internal/llm/client.go`
- `internal/prompts/prompts.go`
- `internal/i18n/locales/*.json`
- `cmd/coe/doctor.go`

## 5. 目标架构

## 5.1 新的运行时状态

`App` 需要新增一块场景状态：

- `current_scene_id`
- `available_scenes`

这块状态只活在 daemon 内存里，不做文件持久化。

推荐用一个独立的小对象管理，而不是把字符串散在 `App` 里：

- `scene.State`

最小接口：

- `Current() Scene`
- `SwitchTo(sceneID string) (changed bool, scene Scene, err error)`
- `List() []Scene`

## 5.2 Scene 定义

建议的 `Scene` 结构：

```go
type Scene struct {
    ID          string
    DisplayName map[i18n.Locale]string
    Aliases     []string
    Description string
}
```

v1 内置：

- `general`
- `terminal`

`Aliases` 用于场景路由判断，不用于前置门控。

## 5.3 主流程

正常听写链路将变成：

1. 录音
2. ASR
3. 使用“当前场景对应的 correction prompt”做 LLM 校正
4. 基于 correction 后文本做“切换场景”前缀门控
5. 若未命中门控：
   - 继续原有输出链路
6. 若命中门控：
   - 发起场景路由 LLM 请求
7. 若路由成功：
   - 更新当前场景
   - 发场景切换通知
   - 不执行普通输出
8. 若路由失败：
   - 回退到普通输出链路

## 6. Phase 1: 场景状态与目录骨架

目标：

- 先把 `current_scene` 这个概念放进 daemon
- 先不接 LLM 路由，也先不改变 correction 行为

### 6.1 新增 `internal/scene`

先实现：

- 场景常量
- 内置场景 catalog
- 运行时状态容器

建议接口：

- `scene.DefaultCatalog()`
- `scene.NewState(initial Scene)`
- `scene.SceneByID(id string)`

### 6.2 在 `App` 中挂入场景状态

`bootstrap.go` 初始化时：

- 默认场景设为 `general`
- 把内置 catalog 和 state 放进 `App`

### 6.3 验证方式

通过标准：

1. `App` 启动后总能拿到一个当前场景
2. 默认场景是 `general`
3. 单元测试覆盖场景切换和非法 scene id

## 7. Phase 2: D-Bus 暴露当前场景

目标：

- 让外部观察者能知道当前场景
- 为后续 doctor 和调试打基础

### 7.1 D-Bus 接口扩展方式

不建议直接修改当前 `Status() -> (state, session_id, detail)` 的签名。

原因：

- 这是已有接口
- 直接改 tuple 容易影响现有调用方

更稳妥的 v1 方案：

- 新增 method：`CurrentScene() -> (scene_id, display_name)`
- 新增 signal：`SceneChanged(scene_id, display_name)`
- 同时在 `Status().detail` 中追加 `scene=<id>`，便于旧调用方最小感知

### 7.2 需要改的现有点

- `internal/ipc/dbus/dictation.go`
- `internal/app/ipc.go`
- `cmd/coe/doctor.go`

### 7.3 doctor 行为

如果 daemon 可达，`doctor` 应增加一项：

- `Current scene`

至少显示：

- configured default: `general`
- daemon current: `general` / `terminal`

### 7.4 验证方式

通过标准：

1. `gdbus call` 能读取 `CurrentScene()`
2. daemon 切换场景时会发 `SceneChanged`
3. `coe doctor` 能展示当前场景

建议命令：

```bash
gdbus call --session \
  --dest com.mistermorph.Coe \
  --object-path /com/mistermorph/Coe \
  --method com.mistermorph.Coe.Dictation1.CurrentScene
```

## 8. Phase 3: 场景切换通知

目标：

- 场景切换成功后给用户明确反馈

### 8.1 通知行为

成功时：

- title: `Scene switched`
- body: 本地化展示名

例如：

- `Terminal`
- `终端`
- `ターミナル`

### 8.2 本地化接入

当前仓库已有：

- `internal/i18n/locales/en.json`
- `internal/i18n/locales/zh.json`
- `internal/i18n/locales/ja.json`

需要新增：

- `scene_switched_title`
- `scene_general_display_name`
- `scene_terminal_display_name`

如果实现中还需要失败通知或 debug 通知，再追加对应 key。

### 8.3 验证方式

通过标准：

1. 手工切换场景时能看到系统通知
2. 通知 body 随 locale 变化
3. 普通“完成通知”开关不影响场景切换通知

## 9. Phase 4: 场景化 correction prompt

目标：

- 让 `current_scene` 真正影响后续 correction 行为

### 9.1 实现方式

当前 `pipeline.Orchestrator` 只有一个 `Corrector`。

要让 correction 随场景变化，v1 推荐最小改法：

- 启动时构造两个 `llm.Corrector`
  - `general`
  - `terminal`
- 它们使用同一个 provider / model / key
- 区别只在 prompt template file

运行时根据当前场景选择对应 `Corrector`。

### 9.2 为什么不先改 `Corrector` 接口

当前接口很小：

```go
Correct(context.Context, string) (Result, error)
```

如果为了场景去改成“每次调用都带 prompt options”，会把变更扩散到：

- `internal/llm`
- `internal/pipeline`
- 所有测试桩

v1 不值得先走这条路。

因此更推荐：

- 在 `App` 或 runtime 层做 corrector 选择
- 让 `pipeline.Orchestrator` 每次执行前拿到一个已经选好的 `Corrector`

### 9.3 prompt 文件建议

建议拆成：

- `llm-correction-general.tmpl`
- `llm-correction-terminal.tmpl`

其中 `terminal` 版本额外强调：

- 保留命令名
- 保留路径
- 保留 flag
- 保留环境变量
- 保留包名与文件名
- 不把命令 token 纠成自然语言

### 9.4 验证方式

通过标准：

1. `general` 场景继续表现为普通文本整理
2. `terminal` 场景下，`grep`, `systemctl`, `--user`, `/var/log` 这类 token 保真明显更高
3. 混合语言 token 仍不被翻译

## 10. Phase 5: 切换门控

目标：

- 先用便宜的字符串规则把“像控制命令”的文本筛出来

### 10.1 门控输入

门控只基于 correction 后文本：

- `processed.Corrected`

### 10.2 前缀词表

v1 直接硬编码在 `internal/scene/gate.go`：

- 中文：
  - `切换场景`
  - `切到场景`
  - `切到`
  - `进入场景`
- 英文：
  - `switch scene`
  - `set scene`
- 日文：
  - `シーン切替`
  - `シーンを`

实现要求：

- 先做 `TrimSpace`
- 大小写不敏感
- 只看前缀，不做全文 fuzzy match

### 10.3 输出

门控函数最小只要回答：

- 命中 / 未命中

不在门控阶段解析具体场景。

### 10.4 验证方式

通过标准：

1. 普通文本不会误命中
2. 中英日固定短语能命中
3. 只说 `terminal` 不应命中，必须是“切换场景”类命令

## 11. Phase 6: 场景路由 LLM 请求

目标：

- 命中门控后，把“切到哪个场景”交给一个小的 LLM 路由步骤

### 11.1 复用现有 LLM provider

v1 不新增独立 provider。

做法：

- 复用当前 LLM provider / model / key
- 另外构造一个“场景路由专用”的 `llm.Corrector`
- 使用 `scene-router.tmpl`
- 输入内容是 JSON 字符串
- 输出必须是 JSON 字符串

### 11.2 输入 JSON

按需求文档固定为：

- `current_scene`
- `available_scenes`
- `utterance`

其中 `available_scenes` 直接来自 `scene.Catalog`。

### 11.3 输出 JSON 解析

新增一个小的解析结构，例如：

```go
type RouteResult struct {
    Intent      string `json:"intent"`
    TargetScene string `json:"target_scene"`
}
```

解析规则：

- 解析失败 -> 视为普通听写
- `intent != switch_scene` -> 视为普通听写
- `target_scene` 不存在 -> 视为普通听写

### 11.4 为什么不把场景解析写死在代码里

原因：

- Alias 会有多语言
- 用户说法不一定固定
- 轻量路由比手写硬编码短语覆盖更稳

### 11.5 验证方式

通过标准：

1. `切换场景到终端` 能路由到 `terminal`
2. `switch scene to terminal` 能路由到 `terminal`
3. `シーンをターミナルに切り替え` 能路由到 `terminal`
4. 路由返回非法 JSON 时会回退成普通听写

## 12. Phase 7: runtime 接线

目标：

- 把前面几块真正接进 `internal/app/runtime.go`

### 12.1 推荐接线位置

当前 runtime 主链大致是：

1. 录音停止
2. `ProcessCapture()`
3. 结果通知
4. 输出

v1 接线建议：

1. 录音停止
2. `ProcessCapture()`，其中 correction 已按当前场景执行
3. 检查 `processed.Corrected` 是否命中场景门控
4. 若命中，则走场景路由
5. 若场景切换成功：
   - 更新场景状态
   - 发 `SceneChanged`
   - 发场景切换通知
   - 提前返回，不执行普通 output
6. 若未命中或路由失败：
   - 继续现有输出流程

### 12.2 混合语句处理

对：

- `切到终端，然后 ls -la`

v1 仍按“场景切换成功”处理：

- 切到 `terminal`
- 不输出 `ls -la`

这条逻辑不要在 runtime 里额外切分文本，而是由 route prompt 直接把它归入“场景切换”。

## 13. Phase 8: 测试与验证

## 13.1 单元测试

建议新增测试覆盖：

- `internal/scene/state_test.go`
- `internal/scene/gate_test.go`
- `internal/scene/router_test.go`
- `internal/app/runtime_test.go`
- `internal/ipc/dbus/dictation_test.go`
- `cmd/coe/doctor_test.go`

重点断言：

- 默认场景
- 场景切换成功
- 场景切换后发通知
- 切换成功后不走普通 output
- 路由失败时回退到普通听写
- D-Bus 能读到当前场景

## 13.2 手工验证

建议最少做这几条：

1. 启动 `coe serve`
2. 通过语音说“切换场景到终端”
3. 看到 `Scene switched` 通知
4. `gdbus` 读到当前场景为 `terminal`
5. 再说一段包含命令的文本，例如：
   - `用 grep 查一下 error log`
6. 观察 correction 结果是否保留 `grep` 和 `error log`
7. 重启 daemon 后确认场景回到 `general`

## 14. 推荐落地顺序

建议按这个顺序做：

1. `internal/scene` 目录与内置 catalog
2. `App` 场景状态
3. D-Bus `CurrentScene()` 与 `SceneChanged`
4. 场景切换通知与 i18n
5. `general` / `terminal` correction prompt 分离
6. 门控
7. 场景路由 LLM 请求
8. runtime 接线
9. doctor 展示与回归测试

这样做的好处是：

- 每一步都有可见结果
- 可以先验证状态与可观测性
- 最后才把控制逻辑插进主链，降低回归风险

## 15. 暂不建议做的事

以下内容即使实现时觉得“顺手”，v1 也先不要带进去：

- 场景持久化到配置文件或 state 文件
- 自动按前台应用切场景
- 支持用户自定义大量场景
- 单次语音里既切场景又输出剩余文本
- 单独加新的 CLI 命令做场景管理

先把“手动切换 + 影响 correction + 可观察”这条主链做稳，再扩。
