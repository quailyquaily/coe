# Coe 个人词典设计

## 1. 目标

给 `Coe` 增加一份用户可编辑的“个人词典”文件，用来解决这几类问题：

- 专有名词经常被 ASR 或 LLM 校正错
- terminal 场景里的命令、路径、flag、包名需要更稳定地保留
- 混合语言口述时，希望某些词严格按用户习惯输出
- 用户希望通过一个简单文件，长期积累自己的纠错偏好

v1 的设计目标是：

1. 用户可以通过单独文件维护词典
2. 词典可以按场景生效
3. 词典既能影响 LLM correction，也能在 LLM 之后做确定性收口
4. 词典实现不额外引入运行时热加载机制

## 2. 设计原则

### 2.1 不只靠 prompt

只把词典塞进 prompt，不够稳。

原因：

- LLM 可能忽略个别词条
- LLM 可能仍然做翻译或改写
- terminal 场景里命令类 token 需要更强的保真

因此 v1 采用双层方案：

1. 在 LLM correction prompt 里注入当前场景相关的词典提示
2. 在 LLM 输出之后，再做一次确定性的词典归一化

## 2.2 文件要简单、可手写、可 review

词典文件应该满足：

- 人可以直接编辑
- diff 清晰
- 不需要专门 UI
- 能表达“标准写法”和“常见误识别”

因此 v1 选 `YAML`，而不是数据库、正则脚本或 DSL。

## 2.3 词典是“归一化”，不是“翻译”

词典的作用是把用户说的某些词或短语，稳定收束到一个标准写法。

它不应该：

- 把一种语言翻译成另一种语言
- 重写整句语义
- 参与复杂意图分类

## 3. 文件位置和配置

建议新增配置：

```yaml
dictionary:
  file: "~/.config/coe/dictionary.yaml"
```

语义：

- 为空：不启用个人词典
- 有值：从该路径加载词典

相对路径可以沿用现有配置规则，相对于 `config.yaml` 所在目录解析。

## 4. 文件格式

## 4.1 v1 结构

建议格式：

```yaml
entries:
  - canonical: "Coe"
    aliases: ["扣一", "口诶", "coe"]
    scenes: ["general", "terminal"]

  - canonical: "systemctl"
    aliases: ["system control", "system c t l", "系统ctl"]
    scenes: ["terminal"]

  - canonical: "/var/log/coe"
    aliases: ["slash var slash log slash coe"]
    scenes: ["terminal"]
```

字段定义：

- `canonical`
  词条的标准输出写法
- `aliases`
  会被归一化到 `canonical` 的口述形式、误识别形式、旧写法
- `scenes`
  可选。为空或省略表示全局生效；有值表示只在这些场景里生效

## 4.2 所有文本统一使用双引号

“文档强约定”，不强制校验；如果实现成本低，也可以在加载时对未加引号的写法给出 warning。

建议 v1 明确要求：

- 所有字符串都用双引号 `"` 包起来
- `aliases` 和 `scenes` 这类短数组，优先使用紧凑语法 `[...]`

例如：

```yaml
canonical: "systemctl"
```

而不是：

```yaml
canonical: systemctl
```

原因：

- 路径、冒号、连字符、数字、布尔样式文本更安全
- 命令、flag、多语言片段在 YAML 里不容易产生歧义
- 用户在 review diff 时也更直观
- 常见词条通常不长，紧凑数组更省空间，浏览更快

## 4.3 v1 不支持的格式能力

v1 明确不做：

- 正则表达式
- 替换优先级手动配置
- 嵌套词典
- 导入其他词典文件
- 词条级别的语言标签
- 不接入 ASR prompt

## 5. 数据模型

建议新增 `internal/dictionary/`，但 v1 先只保留最小骨架：

- `dictionary.go`
- `dictionary_test.go`

建议结构：

```go
type Entry struct {
    Canonical string   `yaml:"canonical"`
    Aliases   []string `yaml:"aliases"`
    Scenes    []string `yaml:"scenes"`
}

type File struct {
    Entries []Entry `yaml:"entries"`
}
```

运行时建议增加一层已编译视图：

```go
type CompiledEntry struct {
    Canonical string
    Aliases   []string
    Scenes    map[string]struct{}
}
```

原因：

- 运行时匹配不需要反复 trim / lower / 去重
- scene 过滤更快

## 6. 运行时行为

## 6.1 加载时机

daemon 启动时加载一次词典。

如果词典文件不存在：

- 不报错
- 记录 debug 日志
- 视为未启用词典

如果词典格式错误：

- 启动不应直接失败
- 应记录 warning
- 本次运行把词典视为不可用

这里不要因为一个用户维护的可选文件，把整条听写链路打死。

## 6.2 生效方式

v1 不做热加载。

实现方式：

- daemon 启动时加载词典
- 用户修改词典后，重启 `coe.service` 生效

这样可以避免再引入一层文件监测和运行时缓存刷新逻辑。

## 6.3 scene 过滤

词典在使用前先按当前场景过滤：

- `scenes` 为空：所有场景可见
- `scenes` 包含当前场景：当前场景可见
- 其他：当前场景不可见

例如：

- `systemctl` 在 `terminal` 可见
- 在 `general` 不应参与归一化

## 7. 接入点

## 7.1 Prompt 注入

在当前场景的 LLM correction prompt 中注入当前场景可见的 glossary。

建议给模板数据增加类似字段：

```go
type LLMTemplateData struct {
    Provider     string
    Model        string
    EndpointType string
    Dictionary   string
}
```

注入到 prompt 的文本可以是：

```text
PERSONAL DICTIONARY:
- "system control" => "systemctl"
- "slash var slash log slash coe" => "/var/log/coe"
- "扣一" => "Coe"
```

规则要求：

- 只注入当前场景可见的词条
- 按 alias 展开，而不是只列 canonical
- 单字符 alias 不注入 prompt
- 词条数量过多时要做截断，避免 prompt 无限增长

v1 可以先设一个保守上限，例如：

- 最多 100 条 alias 映射
- 或最多 2000 字符

超出的部分记录 debug 日志即可。

## 7.2 LLM 后的确定性归一化

在 `ApplyCorrection()` 之后，增加一次词典替换。

顺序建议：

1. 先拿到 `result.Corrected`
2. 用当前场景可见词典做归一化
3. 再把结果交给 output

原因：

- 这是对 LLM 的保底纠偏
- 对 terminal 命令尤其重要

## 8. 匹配规则

## 8.1 匹配顺序

为了避免短 alias 抢掉长 alias，运行时替换应按以下顺序：

1. 先按 alias 长度从长到短排序
2. 长度相同则按原始定义顺序

例如：

- `"system control"` 应先于 `"system"`

## 8.2 匹配语义

v1 建议只做字面匹配，不做正则。

最小语义：

- 大小写敏感地保留 `canonical`
- alias 匹配时可以做一个“弱归一化”视图，例如：
  - trim 首尾空白
  - 把连续空白压成一个空格

是否做大小写不敏感 alias 匹配，建议保守处理：

- `general`：可以更宽松
- `terminal`：应更保守，避免误改命令大小写

v1 如果想控制复杂度，可以统一先做“原样匹配 + 空白归一化匹配”，不做大小写折叠。

## 8.3 边界控制

为了降低误替换，约束：

- 不建议配置单字符 alias
- 不建议配置过短的高频词

实现上可以先做简单规则：

- 单字符 alias 不注入 prompt，只参与程序后处理
- 单字符 alias 的后处理必须按严格 token 边界匹配，不能做任意子串替换

这样能先挡掉最危险的误伤情况。

## 9. Prompt 模板改动建议

当前已有：

- `llm-correction-common.tmpl`
- `llm-correction-general.tmpl`
- `llm-correction-terminal.tmpl`

建议在 common 或 scene-specific 模板里增加一段：

```text
PERSONAL DICTIONARY:
{{.Dictionary}}
```

并明确写规则：

- if an alias appears, prefer the mapped canonical form
- do not translate dictionary entries into another language
- preserve canonical spelling exactly

这样能和现有“保留原语言、不翻译”的约束保持一致。

## 10. 失败策略

## 10.1 词典文件不可读

行为：

- 记录 warning
- 跳过词典能力
- 主链继续工作

## 10.2 词典文件格式错误

行为：

- 记录 warning
- 本次运行视为无词典

因为 v1 没有热加载，不需要维护“上一版已成功加载的词典”。

## 10.3 词典过长

行为：

- prompt 注入部分做截断
- 确定性后处理仍然可用

这样即使 prompt budget 不够，词典也不是完全失效。

## 11. 建议的日志和可观测性

建议记录这些 debug 信息：

- 词典文件是否启用
- 加载成功的词条数
- 当前场景命中的词条数
- prompt 注入时截断了多少词条
- 后处理实际替换了哪些 alias

同时 `doctor` 后续可以考虑增加一项：

- `Personal dictionary: enabled / disabled / invalid`

v1 可以先不做 `doctor`，但日志建议一开始就有。

## 12. 测试范围

至少覆盖：

1. 加载有效词典文件
2. 词典文件不存在时平稳退化
3. scene 过滤正确
4. alias 长度排序正确
5. terminal 场景里的命令词归一化
6. 混合语言内容不被翻译
7. 单字符 alias 不进 prompt，但后处理仍可命中
8. prompt 注入文本正确

## 13. v1 建议的实现顺序

1. 新增 `dictionary.file` 配置
2. 新增 `internal/dictionary` 的加载和编译逻辑
3. 在 app/bootstrap 里加载词典，并把 scene-aware dictionary 挂进 app
4. 给 LLM prompt 模板注入 glossary
5. 在 correction 后做确定性归一化
6. 补测试和样例词典文件

## 14. 推荐结论

推荐采用：

- 单独 YAML 文件
- 所有字符串统一用双引号
- 支持 `canonical + aliases + scenes`
- scene-aware prompt 注入
- LLM 后确定性归一化

不推荐 v1 采用：

- 只做 prompt 注入
- 只做后处理但不给 LLM 任何词典上下文
- 正则替换
- UI 编辑器
- 持久化数据库
- 热加载
