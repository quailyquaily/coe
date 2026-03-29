# Coe 按当前 App 自动切换场景需求

## 1. 背景

当前 `Coe` 已经有两种场景：

- `general`
- `terminal`

并且已经具备两项现有能力：

1. 可以通过语音命令或 GNOME 托盘手动切换场景
2. 在 GNOME 下可以通过 GNOME Shell extension 读取当前焦点窗口的 `wm_class`

与此同时，系统已经能较稳定地判断“当前聚焦目标是否像一个 terminal”，这一能力目前主要用于 paste shortcut 的切换：

- 普通 App 走 `Ctrl+V`
- terminal-like App 走 `Ctrl+Shift+V`

既然当前链路已经能知道“当前 App 是不是 terminal”，那么场景也应该能利用同一份信号，在用户开始一次听写时自动切换到更合适的场景。

## 2. 目标

这个需求的目标是：

1. 在一次听写开始处理时，根据当前聚焦 App 自动选择场景
2. 如果当前 App 是 terminal-like，则自动切到 `terminal`
3. 如果当前 App 不是 terminal-like，则自动切到 `general`
4. 这种自动切换不需要用户说“切换场景”
5. 自动切换应尽量复用现有焦点识别能力，不额外引入新的窗口识别系统

## 3. v1 范围

v1 只做最小能力：

- 只支持两个内置场景：
  - `general`
  - `terminal`
- 只在支持焦点识别的桌面运行时生效
- 判断依据直接复用当前已有的 terminal-like App 识别逻辑
- 自动切换发生在一次听写处理开始时

v1 明确不做：

- 任意自定义 App 到场景的映射配置
- 超过两个场景的自动分类
- 基于窗口标题、进程命令行或控件内容的复杂识别
- 在 unsupported desktop 上模拟焦点识别

## 4. 用户体验

## 4.1 基本行为

如果用户当前焦点在 terminal，例如：

- GNOME Terminal
- Ptyxis
- Xfce Terminal
- WezTerm

当用户触发一次新的听写时，`Coe` 应自动把当前场景设为 `terminal`，然后再继续后面的 LLM correction / output 链路。

如果用户当前焦点在普通 GUI App，例如浏览器、聊天工具、编辑器普通文本框，则自动把场景设为 `general`。

## 4.2 自动切换时机

自动切换应该发生在“这一次听写真正开始处理”之前，推荐时机是：

1. 录音结束
2. 准备进入 ASR / correction 之前，读取当前焦点 App
3. 决定本次听写应该用哪个场景
4. 如果场景发生变化，则更新当前场景
5. 然后继续这一次听写的 correction 和 output

这样可以保证：

- 本次听写从一开始就使用正确场景的 correction prompt
- 自动切换和 paste 行为看到的是同一份焦点信息

## 4.3 用户可感知反馈

如果自动切换真的把场景从 `general` 改成了 `terminal`，或从 `terminal` 改成了 `general`，应该发出一条通知。

通知语义建议和现有手动切换保持一致：

- 标题沿用 `Scene switched`
- 正文显示目标场景展示名

如果自动判断结果和当前场景相同，则不发通知。

## 4.4 手动切换与自动切换的关系

自动切换是“每次听写开始时重新判断”的行为，不是一个一次性推荐。

也就是说：

- 用户即使刚刚手动切到 `general`
- 如果下一次听写开始时焦点在 terminal
- 系统仍然应自动切回 `terminal`

反过来也一样：

- 用户手动切到 `terminal`
- 但下一次听写开始时焦点在普通 App
- 系统应自动切回 `general`

这意味着 v1 的语义是：

- 手动切换只影响“当前此刻的场景状态”
- 自动切换在下一次听写开始时重新覆盖

## 4.5 不应影响现有语音切换命令

已有“切换场景”语音命令仍然保留。

但自动切换先于普通听写 correction 生效，作用于“本次听写该用哪个场景”。

而语音切换命令本身仍然是显式控制命令，属于另一条链路。

## 5. 与现有 terminal 识别能力的关系

当前项目已经有 terminal-like App 判断逻辑，用于 paste shortcut 选择。

v1 的自动切换必须直接复用这套判断，不应该再新写一份独立的 terminal matcher。

原因：

- 避免两套 terminal 列表逐渐漂移
- 避免“paste 认为是 terminal，但 scene 认为不是”的行为分裂
- 让焦点判断只维护一个真相来源

## 6. 配置模型

自动切换如果需要配置入口，应该使用一套独立的 auto-switch 规则，而不是把 `matcher` 直接塞进 scene 定义。

原因：

- scene 里已经有 `aliases`，它服务于“语音显式切换场景”的命令匹配
- 当前 App 匹配属于“焦点上下文 -> scene”的路由逻辑，语义和 `aliases` 不同
- 如果再给 scene 增加一个泛化的 `matcher` 字段，容易把“语音匹配”和“App 匹配”混成一层

因此推荐的模型是：

```yaml
scene_auto_switch:
  enabled: true
  rules:
    - when: "terminal_like"
      scene: "terminal"
    - when: "default"
      scene: "general"
```

其中：

- `terminal_like` 代表直接复用现有 terminal 判断能力
- `default` 代表 fallback
- `scene` 必须引用已有 scene id

v1 可以先把这套规则硬编码在程序里，但语义上应按这套独立规则模型设计，而不是扩展 scene catalog 的结构。

## 7. 作用范围

自动切换只影响：

- 当前场景状态
- 本次听写使用哪个 correction prompt
- 场景切换通知

自动切换不应直接改变：

- 用户配置文件
- 场景 catalog
- 语音切换命令本身的解析方式

## 8. 边界行为

## 8.1 焦点信息不可用

如果当前运行环境无法读取焦点 App，例如：

- 不是 GNOME
- GNOME focus helper 不可用
- D-Bus 调用失败

则自动切换应静默跳过。

行为：

- 保持当前场景不变
- 不发自动切换通知
- 继续当前听写链路

## 8.2 录音前后焦点变化

用户可能在录音过程中切换窗口。

v1 不要求处理“录音开始时”和“录音结束时”两个焦点的冲突，只取单一时点判断即可。

建议 v1 统一使用“录音结束、处理开始前”的当前焦点。

## 8.3 当前场景已正确

如果自动判断出的目标场景和当前场景一致：

- 不重复写状态
- 不发通知
- 直接继续处理

## 8.4 语音切换命令的优先级

如果这次用户说的话本身就是“切换场景到终端”，则仍按现有语音切换命令链路处理。

自动切换只负责决定“本次默认使用哪个场景做 correction”，不替代显式控制命令。

## 9. 推荐的 v1 语义

v1 推荐语义如下：

1. 每次录音结束后，在进入 correction 前读取当前焦点 App
2. 用已有 terminal matcher 判断目标场景：
   - terminal-like -> `terminal`
   - otherwise -> `general`
3. 如果目标场景和当前场景不同：
   - 切换当前场景
   - 发场景切换通知
4. 然后继续本次听写处理

## 10. 验收标准

满足以下条件则认为 v1 完成：

1. 焦点在 terminal 时，新一次听写会自动使用 `terminal` 场景
2. 焦点在普通 App 时，新一次听写会自动使用 `general` 场景
3. 自动切换与 paste shortcut 的 terminal 判断结果一致
4. 焦点判断失败时，系统平稳退化，不影响听写主链
5. 只有场景真正变化时才发通知
6. 语音显式切换场景功能仍然正常
