# Skills 集成优化方案

> **设计原则**：零配置、自动发现、智能触发、通用适用
>
> **状态**: ✅ 已完成实现 (2026-02-27)

---

## 实现完成

### 已实现的功能

| 模块 | 文件 | 状态 |
|------|------|------|
| 自动发现 | `internal/skills/discovery.go` | ✅ |
| Skill 加载 | `internal/skills/loader.go` | ✅ |
| 索引管理 | `internal/skills/skill.go` | ✅ |
| 触发引擎 | `internal/skills/trigger.go` | ✅ |
| 上下文注入 | `internal/skills/context.go` | ✅ |
| Manager API | `internal/skills/manager.go` | ✅ |
| App 集成 | `internal/app/app.go` | ✅ |
| Agent 集成 | `internal/llm/agent/agent.go` | ✅ |
| TUI 命令 | `internal/tui/components/chat/editor.go` | ✅ |
| 单元测试 | `internal/skills/skills_test.go` | ✅ |

### 测试结果

```
=== RUN   TestDiscoverSkills
--- PASS: TestDiscoverSkills (0.00s)
=== RUN   TestLoadSkill
--- PASS: TestLoadSkill (0.00s)
=== RUN   TestCalculateSkillScore
--- PASS: TestCalculateSkillScore (0.00s)
=== RUN   TestSkillIndex
--- PASS: TestSkillIndex (0.00s)
PASS
```

### 使用方式

**1. 自动触发（默认）**

无需任何配置，将 skill 放入 `~/.my-opencode/skills/` 目录后，程序会自动：
- 启动时发现并加载所有 skills
- 根据用户输入内容自动匹配相关 skills
- 将 skill 内容注入到 LLM 上下文中

**2. TUI 命令**

```
/skills              - 列出所有已加载的 skills
/skills search <q>   - 搜索 skills
/skills reload       - 重新加载 skills
/help                - 显示帮助
```

---

## 1. 项目现状分析

### 1.1 当前项目架构

**MyOpenCode** 是一个基于 Go 的终端 AI 助手，核心架构包括：

```
├── internal/
│   ├── app/           # 应用层服务组合
│   ├── config/        # 配置管理（支持多层级配置加载）
│   ├── llm/
│   │   ├── agent/     # Agent 系统（coder/summarizer/task/title）
│   │   ├── tools/     # 内置工具（Bash/Edit/Write/View 等）
│   │   └── provider/  # LLM 提供商接口
│   ├── tui/           # TUI 界面系统
│   └── session/       # 会话管理（SQLite 持久化）
```

### 1.2 Skills 现状

**用户 Skills 目录**: `C:\Users\Hiworld\.my-opencode\skills\`

现有 Skills 示例：
- `frontend-design/` - 前端界面设计指南
- `mcp-builder/` - MCP 服务器创建指南
- `skill-creator/` - Skill 创建与优化指南
- `pdf/`, `pptx/`, `xlsx/` - 文档处理技能
- 等 20+ 个技能

**Skill 文件格式** (SKILL.md):
```markdown
---
name: skill-name
description: 触发条件和使用说明
license: Complete terms in LICENSE.txt
---

技能主体内容（Markdown 格式）
- 工作流程指导
- 最佳实践
- 示例代码
```

**Skill 目录结构**:
```
skill-name/
├── SKILL.md           # 必需：技能定义
├── LICENSE.txt        # 许可协议
├── scripts/           # 可选：可执行脚本
├── references/        # 可选：参考文档
└── assets/            # 可选：资源文件（模板、字体等）
```

### 1.3 当前问题分析

| 问题 | 描述 | 影响 |
|------|------|------|
| **无 Skills 加载机制** | 项目没有读取和解析用户 skills 目录的代码 | Skills 完全无法使用 |
| **无 Skill 触发系统** | 没有根据用户输入自动匹配/触发 Skill 的机制 | 用户需手动指定使用哪个 Skill |
| **无上下文注入** | Skill 内容无法注入到 LLM 对话上下文中 | Claude 无法获取 Skill 知识 |

---

## 2. 优化目标

### 2.1 核心设计理念

**零配置 (Zero Configuration)**
- 用户无需在任何配置文件中添加 skills 相关设置
- 程序启动时自动检测 skills 目录是否存在
- 目录不存在时不报错、静默跳过

**自动发现 (Auto Discovery)**
- 自动扫描标准路径：`~/.my-opencode/skills/`
- 识别任何包含 `SKILL.md` 的子目录为有效 skill
- 动态加载新增 skills，无需重启

**智能触发 (Intelligent Triggering)**
- 根据用户输入内容自动判断使用哪些 skills
- 无需用户手动指定或启用
- 支持多 skill 同时触发（如同时涉及前端设计和文档处理）

**通用适用 (Universal)**
- 适用于任何遵循 SKILL.md 格式的 skill
- 不依赖特定 skill 的内部实现
- 支持未来新增的任何 skill

### 2.2 功能特性

| 功能 | 说明 | 配置需求 |
|------|------|----------|
| **自动发现** | 启动时扫描 `~/.my-opencode/skills/` | 无 |
| **智能触发** | 根据输入自动匹配 skills | 无 |
| **上下文注入** | 将 skill 内容注入 LLM 请求 | 无 |
| **手动覆盖** | `/skills` 命令手动管理 | 无 |

---

## 3. 技术设计方案

### 3.1 自动发现机制（零配置）

**核心设计**：不需要任何配置文件，程序自动发现和加载 skills

```go
// internal/skills/discovery.go

// 标准 Skills 目录路径（用户环境目录）
var defaultSkillPaths = []string{
    "~/.my-opencode/skills",           // Windows: C:\Users\Hiworld\.my-opencode\skills
    "$XDG_CONFIG_HOME/my-opencode/skills",  // Linux/macOS
}

// DiscoverSkills 自动发现 skills 目录
func DiscoverSkills() ([]string, error) {
    var skills []string

    for _, pathPattern := range defaultSkillPaths {
        // 展开环境变量
        path := expandPath(pathPattern)

        // 检查目录是否存在
        if _, err := os.Stat(path); os.IsNotExist(err) {
            // 目录不存在，静默跳过 - 不影响程序运行
            continue
        }

        // 遍历子目录，查找有效的 skills
        entries, err := os.ReadDir(path)
        if err != nil {
            continue
        }

        for _, entry := range entries {
            if !entry.IsDir() {
                continue
            }

            // 检查是否包含 SKILL.md
            skillPath := filepath.Join(path, entry.Name())
            if _, err := os.Stat(filepath.Join(skillPath, "SKILL.md")); err == nil {
                skills = append(skills, skillPath)
            }
        }
    }

    return skills, nil
}

// expandPath 展开路径中的环境变量
func expandPath(path string) string {
    if strings.HasPrefix(path, "~") {
        home, _ := os.UserHomeDir()
        return filepath.Join(home, path[1:])
    }
    return os.ExpandEnv(path)
}
```

**可选配置**：如果用户想自定义，仍可通过配置文件覆盖

```json
{
  "skills": {
    "directory": "/custom/path/to/skills",  // 可选：自定义路径
    "maxContextSkills": 3                    // 可选：最多注入的 skill 数量
  }
}
```

### 3.2 数据结构设计

```go
// internal/skills/skill.go

// SkillMetadata defines the frontmatter structure
type SkillMetadata struct {
    Name        string   `json:"name" yaml:"name"`
    Description string   `json:"description" yaml:"description"`
    License     string   `json:"license,omitempty" yaml:"license"`
    Version     string   `json:"version,omitempty" yaml:"version"`
    Tags        []string `json:"tags,omitempty" yaml:"tags"`
}

// Skill represents a loaded skill
type Skill struct {
    Metadata   SkillMetadata
    Path       string
    Body       string   // Full markdown body
    Scripts    []string // Available scripts
    References []string // Reference documents
    Assets     []string // Asset files
    LoadedAt   time.Time
}

// SkillIndex is the in-memory index of all loaded skills
type SkillIndex struct {
    mu     sync.RWMutex
    skills map[string]*Skill  // name -> Skill
    byTag  map[string][]string // tag -> []name
}
```

### 3.3 智能触发机制

**核心设计**：根据用户输入内容自动判断是否使用 skills

```go
// internal/skills/trigger.go

// TriggerEngine 智能触发引擎
type TriggerEngine struct {
    index *SkillIndex
}

// MatchSkills 根据用户输入自动匹配 skills
func (e *TriggerEngine) MatchSkills(userInput string) []MatchResult {
    keywords := extractKeywords(userInput)
    var results []MatchResult

    e.index.mu.RLock()
    defer e.index.mu.RUnlock()

    for name, skill := range e.index.skills {
        score := calculateSkillScore(keywords, skill)
        if score >= 0.3 { // 阈值
            results = append(results, MatchResult{
                SkillName: name,
                Score:     score,
                Reason:    generateMatchReason(keywords, skill),
            })
        }
    }

    // 按分数降序排序
    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })

    return results
}

// calculateSkillScore 计算 skill 匹配分数
func calculateSkillScore(keywords []string, skill *Skill) float64 {
    score := 0.0
    descLower := strings.ToLower(skill.Metadata.Description)
    nameLower := strings.ToLower(skill.Metadata.Name)

    for _, kw := range keywords {
        kwLower := strings.ToLower(kw)

        // 完全匹配 description
        if strings.Contains(descLower, kwLower) {
            score += 1.0
        }

        // name 匹配权重更高
        if strings.Contains(nameLower, kwLower) {
            score += 2.0
        }

        // tag 匹配
        for _, tag := range skill.Metadata.Tags {
            if strings.Contains(strings.ToLower(tag), kwLower) {
                score += 1.5
            }
        }
    }

    // 归一化
    if len(keywords) > 0 {
        score = score / float64(len(keywords))
    }

    return score
}

// MatchResult 匹配结果
type MatchResult struct {
    SkillName string
    Score     float64
    Reason    string
}
```

**触发示例**：

| 用户输入 | 触发的 Skill | 匹配原因 |
|----------|-------------|----------|
| "帮我创建一个 React Dashboard" | frontend-design | description 包含"React", "dashboard" |
| "把这个数据导出成 Excel" | xlsx | description 包含"Excel", "导出" |
| "创建一个 MCP 服务器连接 API" | mcp-builder | description 包含"MCP", "server", "API" |
| "做个漂亮的网页，要有动画效果" | frontend-design | description 包含"web", "animation" |
| "处理这个 PDF 文件" | pdf | description 包含"PDF" |
| "你好，介绍一下项目" | (无) | 没有匹配的 skill |

### 3.4 目录结构设计

```
internal/
├── skills/
│   ├── discovery.go      # 自动发现（零配置）
│   ├── skill.go          # Skill 数据结构
│   ├── loader.go         # 文件加载和解析
│   ├── index.go          # 内存索引管理
│   ├── trigger.go        # 触发匹配逻辑（智能）
│   ├── context.go        # 上下文注入
│   └── manager.go        # 对外 API
└── config/
    └── config.go         # 扩展配置结构（可选）
```

### 3.5 加载流程（自动发现）

```
┌─────────────────────────────────────────────────────────────┐
│  Application Startup (main.go)                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  app.NewApp(config)                                         │
│  - 初始化基础服务                                            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  skills.DiscoverSkills() - 自动发现                         │
│  - 扫描 ~/.my-opencode/skills/                              │
│  - 检查每个子目录是否包含 SKILL.md                           │
│  - 目录不存在时静默跳过，不影响启动                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  skills.LoadAll(skillPaths) - 加载 Skills                    │
│  1. 遍历发现的 skill 路径                                      │
│  2. 对每个 skill：                                            │
│     - 解析 SKILL.md frontmatter                              │
│     - 提取 Metadata（name, description）                     │
│     - 记录可用 scripts/references/assets                     │
│  3. 构建内存索引 (SkillIndex)                                │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  索引就绪 - 可供查询使用                                      │
│  - 日志输出："[INFO] Loaded 23 skills"                       │
│  - 无 skills 时："[INFO] No skills directory found"          │
└─────────────────────────────────────────────────────────────┘
```

### 3.6 触发流程（自动判断）

```
┌─────────────────────────────────────────────────────────────┐
│  User Input: "帮我创建一个 React 数据可视化 Dashboard"           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  agent.ProcessMessage(input)                                │
│  - 调用 trigger.MatchSkills(input)                          │
│  - 无需任何配置，自动触发                                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  关键词提取                                                  │
│  - 输入分词：["创建", "React", "数据可视化", "Dashboard"]        │
│  - 过滤停用词                                                │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  遍历所有 Skills，计算匹配分数                               │
│  - frontend-design:                                         │
│    description 包含"React", "dashboard", "可视化"              │
│    score = 0.85                                             │
│  - xlsx: score = 0.05 (不匹配)                              │
│  - mcp-builder: score = 0.02 (不匹配)                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  过滤和排序                                                 │
│  - 过滤 score < 0.3 的 skills                                │
│  - 取 Top-K (默认 3 个)                                       │
│  结果：[frontend-design (0.85)]                             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  返回 MatchResult 列表                                       │
│  - 如果为空：不使用任何 skill，正常处理                       │
│  - 如果有匹配：注入 skill 内容到上下文                          │
└─────────────────────────────────────────────────────────────┘
```

### 3.7 上下文注入流程

```
┌─────────────────────────────────────────────────────────────┐
│  BuildPrompt(messages, matchedSkills)                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  对每个匹配的 Skill (按分数排序):                              │
│  1. 懒加载 Skill 内容（如未加载）                              │
│  2. 注入到 system message                                    │
│  3. 限制总长度，避免超出 token 限制                             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  最终 Prompt 结构：                                           │
│                                                             │
│  [System Message]                                           │
│  You are Claude Code, Anthropic's CLI for coding.           │
│                                                             │
│  ## Active Skills (自动加载)                                │
│  The following skills are available based on your request:  │
│                                                             │
│  ### frontend-design                                        │
│  Description: Create distinctive, production-grade          │
│  frontend interfaces with high design quality...            │
│                                                             │
│  [Skill body from SKILL.md...]                              │
│                                                             │
│  [User Messages...]                                         │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. 实现方案

### 4.1 Skills Manager（对外 API）

**文件**: `internal/skills/manager.go`

```go
package skills

// Manager 提供 skills 系统的公共 API
type Manager struct {
    index  *SkillIndex
    engine *TriggerEngine
}

// NewManager 创建 skills 管理器（自动发现）
func NewManager() (*Manager, error) {
    manager := &Manager{
        index:  NewSkillIndex(),
        engine: &TriggerEngine{},
    }

    // 自动发现和加载 skills（无需配置）
    if err := manager.AutoLoad(); err != nil {
        // 记录错误但不阻止程序启动
        logging.Warn("Failed to load skills", "error", err)
    }

    return manager, nil
}

// AutoLoad 自动发现并加载 skills
func (m *Manager) AutoLoad() error {
    skillPaths, err := DiscoverSkills()
    if err != nil {
        return err
    }

    for _, path := range skillPaths {
        skill, err := LoadSkill(path)
        if err != nil {
            logging.Warn("Failed to load skill", "path", path, "error", err)
            continue
        }
        m.index.Add(skill)
    }

    logging.Info("Skills loaded", "count", m.index.Count())
    return nil
}

// MatchSkills 根据用户输入自动匹配 skills
func (m *Manager) MatchSkills(input string) []MatchResult {
    return m.engine.MatchSkills(input)
}

// GetSkillContext 构建 context 字符串
func (m *Manager) GetSkillContext(matches []MatchResult) string {
    return BuildSkillContext(matches, m.index)
}

// ListSkills 列出所有已加载的 skills
func (m *Manager) ListSkills() []*Skill {
    return m.index.List()
}

// SearchSkills 搜索 skills
func (m *Manager) SearchSkills(query string) []*Skill {
    return m.index.Search(query)
}
```

### 4.2 App 层集成

**文件**: `internal/app/app.go`

```go
type App struct {
    config      *config.Config
    db          *sql.DB
    session     *session.Store
    skillsMgr   *skills.Manager  // 新增
}

func NewApp(cfg *config.Config) (*App, error) {
    // ... 现有初始化代码 ...

    // 初始化 skills 管理器（自动发现）
    skillsMgr, err := skills.NewManager()
    if err != nil {
        logging.Warn("Skills manager initialization failed", "error", err)
    }

    return &App{
        config:    cfg,
        db:        db,
        session:   sessionStore,
        skillsMgr: skillsMgr,
    }, nil
}
```

### 4.3 Agent 层集成

**文件**: `internal/llm/agent/agent.go`

```go
func (a *Agent) ProcessMessage(ctx context.Context, input string) error {
    // 自动匹配 skills（无需配置）
    var skillContext string
    if a.app.skillsMgr != nil {
        matches := a.app.skillsMgr.MatchSkills(input)
        if len(matches) > 0 {
            skillContext = a.app.skillsMgr.GetSkillContext(matches)
            logging.Debug("Matched skills", "skills", matches)
        }
    }

    // 构建消息，注入 skill 上下文
    messages := BuildMessages(input, skillContext)

    // 流式响应
    return a.StreamResponse(ctx, messages)
}
```

### 4.4 TUI 命令集成

**文件**: `internal/tui/components/chat/chat.go`

添加 Slash Commands：

```go
// /skills - 列出所有 skills
// /skills search <query> - 搜索 skills
// /skills reload - 重新加载 skills

func (c *Chat) handleCommand(input string) {
    parts := strings.Fields(input)
    cmd := parts[0]

    switch cmd {
    case "/skills":
        if len(parts) == 1 {
            c.showSkillList()
        } else if parts[1] == "search" {
            c.showSkillSearch(strings.Join(parts[2:], " "))
        } else if parts[1] == "reload" {
            c.reloadSkills()
        }
    }
}
```

---

## 5. 实施路线图

### 阶段 1: 核心基础设施（1-2 周）

| 任务 | 文件 | 优先级 |
|------|------|--------|
| 自动发现机制 | `internal/skills/discovery.go` | P0 |
| Skill 数据结构 | `internal/skills/skill.go` | P0 |
| 加载器实现 | `internal/skills/loader.go` | P0 |
| 索引管理 | `internal/skills/index.go` | P0 |

### 阶段 2: 触发和匹配（1 周）

| 任务 | 文件 | 优先级 |
|------|------|--------|
| 关键词提取 | `internal/skills/trigger.go` | P0 |
| 得分计算 | `internal/skills/trigger.go` | P0 |
| 上下文注入 | `internal/skills/context.go` | P0 |

### 阶段 3: 集成（1 周）

| 任务 | 文件 | 优先级 |
|------|------|--------|
| App 层集成 | `internal/app/app.go` | P0 |
| Agent 集成 | `internal/llm/agent/agent.go` | P0 |
| TUI 命令 | `internal/tui/components/chat/chat.go` | P1 |

---

## 6. 测试计划

### 6.1 单元测试

```go
// internal/skills/loader_test.go
func TestLoadSkill(t *testing.T) {
    skill, err := LoadSkill("testdata/test-skill")
    assert.NoError(t, err)
    assert.Equal(t, "test-skill", skill.Metadata.Name)
}

// internal/skills/trigger_test.go
func TestMatchSkills(t *testing.T) {
    idx := NewSkillIndex()
    idx.Add(&Skill{
        Metadata: SkillMetadata{
            Name: "frontend-design",
            Description: "Create frontend interfaces",
        },
    })

    engine := &TriggerEngine{index: idx}
    results := engine.MatchSkills("build a React dashboard")
    assert.Greater(t, len(results), 0)
    assert.Equal(t, "frontend-design", results[0].SkillName)
}

// internal/skills/discovery_test.go
func TestDiscoverSkills_NoDirectory(t *testing.T) {
    // 测试 skills 目录不存在时静默跳过
    skills, err := DiscoverSkills()
    assert.NoError(t, err)
    assert.Empty(t, skills)
}
```

### 6.2 集成测试

1. 启动应用（无 skills 目录），验证正常运行
2. 创建 skills 目录，添加测试 skill
3. 重启应用，验证自动加载
4. 输入各种查询，验证技能触发
5. 验证 `/skills` 命令功能

---

## 7. 依赖项

需要添加的 Go 模块：

```go
// go.mod
require (
    gopkg.in/yaml.v3 v3.0.1  // YAML frontmatter 解析
)
```

---

## 8. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| **Token 超限** | Skill 内容可能很大 | 懒加载 + 截断 + 只注入相关部分 |
| **匹配不准确** | 触发不相关的 skills | 可调阈值 + 用户反馈机制 |
| **启动变慢** | 加载大量 skills | 异步加载 + 缓存元数据 |
| **Skill 冲突** | 多个 skills 同时触发 | 限制最大数量 + 优先级排序 |

---

## 9. 总结

本方案为 MyOpenCode 项目提供了完整的 Skills 集成设计：

### 核心优势

1. **零配置** - 无需修改任何配置文件，自动发现
2. **通用性** - 适用于任何遵循 SKILL.md 格式的技能
3. **智能触发** - 根据输入内容自动判断使用哪些 skills
4. **无侵入** - skills 目录不存在时不影响程序运行

### 用户体验

- **自动**：用户只需将 skill 放入 `~/.my-opencode/skills/`
- **智能**：程序自动根据输入内容触发相应 skills
- **透明**：无需手动启用或配置，开箱即用
- **可控**：提供 `/skills` 命令供高级用户手动管理

### 实施后效果

```
用户：帮我创建一个 React 数据可视化 Dashboard

程序：[自动发现并注入 frontend-design skill]
      [根据 skill 指导，生成高质量前端代码]
```
