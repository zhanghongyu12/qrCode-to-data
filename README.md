# AI Work Model

> AI 原生个人开发模板

一种不依赖聊天记录、通过项目文件驱动的多 AI 协作开发模式。

## 这是什么？

这是一个项目模板，定义了多个 AI 角色（产品经理、架构师、设计师、开发、审查、测试）的职责边界、协作规则和文档体系。
任何 AI 工具（ChatGPT、Claude Code、Codex、Qoder、DeepSeek 等）都可以基于这套文件体系无缝接手工作。

## 快速开始

> 项目当前状态详见 `docs/06_tasks.md` 顶部"项目状态"区块。

### 作为新项目使用

1. 复制本模板目录作为新项目根目录
2. 在 `docs/00_idea.md` 中写下你的产品想法
3. 让 AI 以 PM 角色接手，阅读 `.ai/roles/pm.md` 后开始工作
4. AI 会按工作流程自动推进，每步输出对应文档

### 作为 AI 接手现有项目

1. 阅读 `README.md`（本文件）了解项目背景与状态
2. 检查项目当前状态（`docs/06_tasks.md` 顶部"项目状态"区块）
3. 阅读 `.ai/workflow.md` 了解工作流程
4. 确认你的角色，阅读 `.ai/roles/<角色>.md`
5. 阅读相关 `.ai/rules/` 规则文件
6. 阅读 `docs/06_tasks.md` 确认当前任务
7. 开始工作，完成后更新对应文档
### 角色快速启动
说角色关键词（pm/架构/设计/dev/审查/test），AI 读 `.ai/startup.md` 自动执行启动流程。

## 目录结构

```
.
├── README.md              ← 项目说明（本文件）
├── .ai/                   ← AI 协作配置
│   ├── roles/             ← 角色定义
│   │   ├── pm.md
│   │   ├── architect.md
│   │   ├── designer.md
│   │   ├── developer.md
│   │   ├── reviewer.md
│   │   └── tester.md
│   ├── rules/             ← 工作规则
│   │   ├── coding_rules.md
│   │   ├── git_rules.md
│   │   ├── coordination_rules.md
│   │   └── document_rules.md
│   └── workflow.md        ← 工作流程
├── docs/                  ← 项目文档
│   ├── 00_idea.md         ← 产品想法
│   ├── 01_prd.md          ← 产品需求文档
│   ├── 02_architecture.md ← 系统架构设计
│   ├── 03_database.md     ← 数据库设计
│   ├── 04_api.md          ← API 设计
│   ├── 05_ui.md           ← UI 设计
│   ├── 06_tasks.md        ← 任务清单
│   ├── 07_decisions.md    ← 决策记录
│   ├── 08_review.md       ← 审查报告
│   ├── 09_test_report.md  ← 测试报告
│   └── CHANGELOG.md       ← 变更日志
├── src/                   ← 源代码
├── tests/                 ← 测试代码
│   ├── unit/              ← 单元测试（Developer）
│   ├── integration/       ← 集成测试（Tester）
│   └── e2e/               ← 端到端测试（Tester）
└── .gitignore
```

## 角色体系

| 角色 | 职责 | 主要输出 |
|------|------|---------|
| PM (产品经理) | 需求分析、PRD | 00_idea.md, 01_prd.md |
| Architect (架构师) | 技术方案、系统设计 | 02_architecture.md, 03_database.md, 04_api.md |
| Designer (设计师) | 界面设计、交互设计 | 05_ui.md |
| Developer (开发) | 编码、单元测试、修 Bug | src/, tests/unit/, 06_tasks.md |
| Reviewer (审查) | 代码质量审查 | 08_review.md |
| Tester (测试) | 集成/端到端测试、用例 | tests/integration/, tests/e2e/, 09_test_report.md |

## 工作流程

```
想法 → PRD → 架构设计 → 数据库设计 → API设计 → UI设计 → 任务拆分 → 编码 → 测试 → Review → 发布
```
详见 `.ai/workflow.md`。

## 支持的 AI 工具

> 以下为 2026 年 7 月国产模型最新版本推荐，按角色列出前三名。
> 工具不限，支持多模型切换的编程工具（如 Qoder、Trae、Claude Code + 国产 API）均可按阶段切模型。

### PM（产品经理）

| 排名 | 模型 | 推荐理由 |
|------|------|---------|
| 1 | Kimi K2.6 | 超长上下文，一次吃进想法 + 竞品资料，文档结构化能力最强 |
| 2 | GLM-5 | 全模态旗舰，语言表达和文档质量稳定 |
| 3 | Qwen3.7-Max | Agentic 旗舰，通用表达强，需求场景推演能力突出 |

### Architect（架构师）

| 排名 | 模型 | 推荐理由 |
|------|------|---------|
| 1 | Kimi K2.6 | 长上下文对照 PRD + 架构 + 数据库 + API 多份文档，综合决策最佳 |
| 2 | DeepSeek-V4 | V4 长上下文效率大幅提升，技术选型推理链路深 |
| 3 | Qwen3.7-Max | "从说得好到做得到"的 Agentic 基座，系统设计落地性强 |

### Designer（设计师）

| 排名 | 模型 | 推荐理由 |
|------|------|---------|
| 1 | GLM-5 | 全模态旗舰，可处理视觉参考素材，文档结构化输出稳定 |
| 2 | Kimi K2.6 | 超长上下文，一次吃进 PRD + 竞品界面，页面规划能力强 |
| 3 | Qwen3.7-Max | Agentic 旗舰，用户场景推演和交互流程设计能力强 |

### Developer（开发）

| 排名 | 模型 | 推荐理由 |
|------|------|---------|
| 1 | Qwen3.6-Plus | 国产编码新王，编程能力接近 Claude Sonnet，日常主力 |
| 2 | GLM-5.1 | 代码能力大增，支持长程自主工作 8 小时，适合复杂长任务 |
| 3 | DeepSeek-V4 | 编码 + 长上下文兼顾，性价比最高，适合批量实现 |

### Reviewer（审查）

| 排名 | 模型 | 推荐理由 |
|------|------|---------|
| 1 | GLM-5.1 | 代码审查 + 长程分析，能持续自主工作，安全/性能隐患最敏锐 |
| 2 | DeepSeek-V4 | 深度推理审查，架构一致性检查强 |
| 3 | Kimi K2.6 | 适合做交叉审查第二意见，对冲单一模型盲区 |

### Tester（测试）

| 排名 | 模型 | 推荐理由 |
|------|------|---------|
| 1 | DeepSeek-V4 | 测试用例覆盖全面，性价比高，适合批量铺用例 |
| 2 | Qwen3.6-Plus | 编码能力强，测试代码质量高 |
| 3 | 豆包 2.0-Code | 字节编码模型，批量生成测试用例效率高 |

### 模型速查总表

| 角色 | 首选 | 第二 | 第三 |
|------|------|------|------|
| PM | Kimi K2.6 | GLM-5 | Qwen3.7-Max |
| Architect | Kimi K2.6 | DeepSeek-V4 | Qwen3.7-Max |
| Designer | GLM-5 | Kimi K2.6 | Qwen3.7-Max |
| Developer | Qwen3.6-Plus | GLM-5.1 | DeepSeek-V4 |
| Reviewer | GLM-5.1 | DeepSeek-V4 | Kimi K2.6 |
| Tester | DeepSeek-V4 | Qwen3.6-Plus | 豆包 2.0-Code |

> **选型建议**：如果全程只用一个模型，DeepSeek-V4 编码 + 推理 + 长上下文兼顾，可跑完全流程；Review 阶段临时调 GLM-5.1 做关键审查即可。
> **双模型交叉审查**：配合 `.ai/rules/ExpertDebate` 机制，用 GLM-5.1 + DeepSeek-V4 分别审查，取交集问题必修，单模型独有作为参考项。

## 双窗口分工：Developer + Tester 并行

本模板支持开两个窗口，一个扮 Developer、一个扮 Tester，并行推进编码与测试。

### 使用方式

1. 窗口 A 扮 Developer，窗口 B 扮 Tester，共享同一仓库
2. 默认 Developer 先行：A 实现 + 写 `tests/unit/` → 交付打 `task-{编号}-handoff` tag → B 接手写 `tests/integration/` + `tests/e2e/` → 出报告
3. 并行 = 流水线重叠：A 做任务 N+1 时 B 测任务 N，单任务时收益为零
4. 测试目录分层：`tests/unit/` 归 Developer，`tests/integration/` + `tests/e2e/` 归 Tester（详见 `.ai/rules/coding_rules.md`）

### 共享 git 工作区风险提示

两窗口共享同一仓库时，git 操作可能互相阻塞（如同时 checkout 不同 tag）。建议：
- 错开提交时间，或使用 `git worktree` 为 Tester 创建独立工作目录
- Developer 交付打 tag 后通知 Tester，Tester 在自己的 worktree 中 checkout 该 tag 测试

### 提升测试互补的可选机制

分工结构本身保证三层测试都有人写（Developer 写 unit，Tester 写 integration + e2e）。
如果项目觉得双方视角仍有盲区，可按需从以下机制中挑选几条加上去，不强制：

1. **实现边界说明**：Developer 交付时附 3-5 行"已处理/已知不覆盖/风险点"，Tester 据此精准补测。投入产出比最高，建议优先加这条。
2. **交叉阅读**：双方写完各自测试后互读对方测试，在 `docs/06_tasks.md` 追加"补充测试建议"。
3. **覆盖率软阈值**：报告含行+分支覆盖率，低于 80% 需列具体未覆盖文件，下降触发警告，禁硬门槛。
4. **契约派生**（有 API 且 `04_api.md` 结构化时）：Developer 和 Tester 各自对照同一份结构化契约工作，漏了契约点测试自动红。
5. **验收点清单 + 变异测试**：高可靠性项目 opt-in，Tester 前置产清单 + 变异测试发现弱断言。

## 自发起并行拆分

除了上述 Developer + Tester 的预设分工，模板支持任一角色在工作中自发起并行拆分——当发现自己被瓶颈卡住、有独立工作块可切出时，拉一个辅助窗口并行。

与预设分工的区别：预设分工是模板内建的（Developer 写 unit、Tester 写 integration/e2e），自发起并行分工是角色临时决定的，按文件隔离切出一路工作给辅助窗口。

机制要点：
- **硬门禁**：可切出独立任务 ≥ 4 个、文件级隔离可达成、契约面 ≤ 8 个签名、发起方保留侧有独立任务可推进。四条全满足才产出请求
- **人工门禁**：发起方产出拆分请求说明，由使用者决定是否执行，不擅自启动
- **契约锁定**：接口签名钉死在 context.md，变更必须先留痕再改代码，辅助方编译报错先查记录不改接口
- **交付状态机**：辅助方交付状态流转（未开始→编码中→测试中→已交付），发起方只认"已交付"为收尾信号

详见 `.ai/rules/parallel_split_rules.md`。

## 核心原则

- **文档驱动**：所有工作基于文档，不基于聊天记录
- **角色分明**：每个 AI 明确自己的角色和权限边界
- **不可跳过**：设计阶段不可跳过直接编码
- **决策留痕**：重要决策记录在 07_decisions.md，避免重复讨论
- **冲突上报**：AI 发现需求冲突不上自作主张，记录后等待人工确认
- **角色间协作**：需要双向对话时通过 session board.md 沟通，详见 `.ai/rules/coordination_rules.md`
