# AI Workflow (工作流程)

## 总览

```
产品想法 → 需求设计 → 原型 → 架构 → 数据库 → API → UI → 任务拆分 → 编码 → 测试 → Review → 发布
```

每个阶段都有明确的角色、输入文档和输出文档。
上一个阶段的输出是下一个阶段的输入，不允许跳过。

---

## 阶段详解

### 阶段 1: 想法记录

- **角色**: 人工 → PM
- **输入**: 用户的口头/文字想法
- **输出**: `docs/00_idea.md`
- **规则**: PM 将想法结构化记录，补充初步分析
- **完成信号**: `docs/00_idea.md` 状态变为"已确认"
- **门禁**: 硬门禁——需人工确认想法准确无误后才能流转

### 阶段 2: 需求设计 (PRD)

- **角色**: PM
- **输入**: `docs/00_idea.md`
- **输出**: `docs/01_prd.md`
- **规则**: PRD 状态必须变为"已确认"后才能进入下一阶段
- **完成信号**: `docs/01_prd.md` 状态变为"已确认"
- **门禁**: 软门禁——`docs/00_idea.md` 状态"已确认" + `docs/07_decisions.md` 无阻塞项

### 阶段 3: 架构设计

- **角色**: Architect
- **输入**: `docs/01_prd.md`
- **输出**: `docs/02_architecture.md`、`docs/03_database.md`、`docs/04_api.md`
- **规则**: 重要技术决策记录到 `docs/07_decisions.md`
- **完成信号**: `docs/02_architecture.md`、`docs/03_database.md`、`docs/04_api.md` 状态均变为"已确认"
- **门禁**: 软门禁——`docs/01_prd.md` 状态"已确认" + 无阻塞项

### 阶段 4: UI 设计

- **角色**: Designer
- **输入**: `docs/01_prd.md`
- **输出**: `docs/05_ui.md`
- **规则**: 可与阶段 3 并行
- **完成信号**: `docs/05_ui.md` 状态变为"已确认"
- **门禁**: 软门禁——`docs/01_prd.md` 状态"已确认" + 无阻塞项

### 阶段 5: 任务拆分

- **角色**: Architect（创建初始任务清单）→ Developer（执行并更新）
- **输入**: `docs/02_architecture.md`、`docs/04_api.md`、`docs/05_ui.md`
- **输出**: `docs/06_tasks.md`
- **规则**: 每个任务必须有明确的验收标准
- **完成信号**: `docs/06_tasks.md` 有待办任务
- **门禁**: 软门禁——`docs/02_architecture.md`、`docs/03_database.md`、`docs/04_api.md`、`docs/05_ui.md` 状态均"已确认"（`05_ui.md` 可标注"不适用"）+ 无阻塞项

### 阶段 6: 编码

- **角色**: Developer
- **输入**: `docs/02_architecture.md`、`docs/03_database.md`、`docs/04_api.md`、`docs/05_ui.md`、`docs/06_tasks.md`、`.ai/rules/coding_rules.md`
- **输出**: `src/`、`tests/`、更新 `docs/06_tasks.md`、`docs/CHANGELOG.md`
- **规则**: 严格遵循编码规范和架构设计
- **完成信号**: 代码可编译运行
- **门禁**: 软门禁——`docs/06_tasks.md` 有待办任务 + 无阻塞项

> 当 Developer 发现任务积压且有独立工作块可切时，可按 `.ai/rules/parallel_split_rules.md` 发起并行拆分请求，经使用者批准后执行，不作为阶段流转的固定环节。

### 阶段 7: 测试

- **角色**: Tester
- **输入**: `docs/01_prd.md`、`docs/02_architecture.md`、`docs/04_api.md`、`docs/06_tasks.md`、`src/`、`tests/unit/`
- **输出**: `tests/integration/`、`tests/e2e/`、`docs/09_test_report.md`
- **规则**: 可与阶段 6 交叉进行（流水线重叠：Developer 做任务 N+1 时 Tester 测任务 N，单任务时收益为零）
- **完成信号**: `docs/09_test_report.md` 状态变为"已确认"
- **门禁**: 软门禁——Developer 已打 `task-{编号}-handoff` tag + 代码可编译运行 + 无阻塞项

#### Developer↔Tester 交接协议

阶段 6（编码）与阶段 7（测试）可并行推进，通过以下协议协调：

**状态流转**：`待办 → 进行中 → 已完成 → 待测试 → 测试中 → 已测试`

1. Developer 完成实现 + `tests/unit/` 后，将任务标记为"待测试"，打 `task-{编号}-handoff` git tag 并通知 Tester
2. Tester 接手后 checkout 到该 tag，将任务标记为"测试中"，编写 `tests/integration/` + `tests/e2e/` 并跑全量测试
3. 测试通过：Tester 将任务标记为"已测试"，出报告（记录被测版本 tag + commit hash），通知 Reviewer
4. 测试失败：Tester 通过 `.session/{session_id}/board.md` 通知 Developer 修复（回环），Developer 修复后重新打 tag

- 常规交接通过 `docs/06_tasks.md` 状态流转（单向事务）
- 异常回环通过 `.session/{session_id}/board.md`（复用 coordination_rules.md）

### 阶段 8: 代码审查

- **角色**: Reviewer
- **输入**: `src/`、`tests/`、`docs/02_architecture.md`、`docs/04_api.md`、`docs/05_ui.md`、`docs/09_test_report.md`、`.ai/rules/coding_rules.md`
- **输出**: `docs/08_review.md`
- **规则**: 严重问题必须修复后才能进入发布
- **回环流程**（Reviewer 发现 Critical 问题后）：
  ```
- **回环流程**（Reviewer 发现 Critical 问题后）：
  双向沟通通过 `.session/{session_id}/board.md` 进行，详见 `.ai/rules/coordination_rules.md`。
  ```
  Review 发现 Critical 问题
    → Developer 修复（复用阶段 6 流程，在 06_tasks.md 中创建新任务）
        — 新任务的依赖关系字段同时引用原任务编号和 08_review.md 中的问题编号
    → Tester 回归测试（复用阶段 7 流程，更新 09_test_report.md）
    → Reviewer 重新审查（阶段 8，在 08_review.md 中追加"复审记录"区块，不覆盖原始报告）
    → 最多循环 2 次，仍不通过则升级为人工决策（记入 07_decisions.md）
  ```
- **回环出口条件**：`docs/06_tasks.md` 无"进行中"的修复任务 + `docs/08_review.md` 无 Critical 未修复 + 无阻塞项
- **完成信号**: `docs/08_review.md` 无 Critical 级别未修复问题
- **门禁**: 软门禁——`docs/09_test_report.md` 状态"已确认" + 无阻塞项

### 阶段 9: 发布

- **角色**: 人工确认 → Developer 执行
- **输入**: 所有文档 + 代码
- **输出**: Tag、更新 `docs/CHANGELOG.md`
- **规则**: 发布前所有测试必须通过
- **门禁**: 自动化门禁——测试全部通过 + `docs/CHANGELOG.md` 已更新 + `docs/08_review.md` 无 Critical 未修复

---

## 并行协调：Designer 与 Architect 对齐

阶段 3（架构设计）与阶段 4（UI 设计）可并行推进。为避免两边产出不一致，采用以下流程：

```
Architect 先填充 04_api.md 初稿（API 分组 + 路径前缀，状态"草稿"）
  → Designer 基于此设计 UI，在 05_ui.md 中标注"本设计假设以下接口存在"
  → Architect 完成详细 API 设计（填请求/响应结构），04_api.md 走"草稿→评审中→已确认"
  → Architect 对照 Designer 的假设做确认
  → 不一致处记入 07_decisions.md
  → 两边确认后，阶段 5 门禁才放行
```

说明：
- 04_api.md 初稿状态为"草稿"时，Designer 参考它属于平行协作行为，不触发门禁检查（Designer 的正式输入只有 01_prd.md）。
- 04_api.md 必须到"已确认"状态后才能通过阶段 5 门禁。
- 接口分组清单不单独建文件，直接作为 04_api.md 的阶段性内容。

---

## 冲突处理流程

当 AI 在工作中发现需求冲突或设计问题时：

```
1. 停止当前工作
2. 在 docs/07_decisions.md 记录问题（分配 DEC-XXX 编号）
3. 在 docs/06_tasks.md 状态区块的"阻塞项"字段列出决策编号（如 阻塞项：DEC-XXX（简述，待人工确认））
4. 标记对应任务状态为"已阻塞"
5. 等待人工确认
6. 人工确认决策后，接手 AI 清除 06_tasks.md 阻塞项中对应编号，并在 07_decisions.md 中将该决策标记为"已确认"
7. 继续工作
```

**绝对禁止：AI 自行修改需求或架构文档来解决冲突。**

---

## 质量门禁

阶段流转必须满足门禁条件。门禁分三级：

| 阶段流转 | 门禁类型 | 条件 |
|---------|---------|------|
| 想法 → PRD | 硬门禁 | `docs/00_idea.md` 状态"已确认" + 人工确认 |
| PRD → 架构 | 软门禁 | `docs/01_prd.md` 状态"已确认" + 无阻塞项 |
| 架构 → 任务 | 软门禁 | `docs/02_architecture.md`、`03_database.md`、`04_api.md` 状态"已确认" + 无阻塞项 |
| UI → 任务 | 软门禁 | `docs/05_ui.md` 状态"已确认"（或标注"不适用"） + 无阻塞项 |
| 任务 → 编码 | 软门禁 | `docs/06_tasks.md` 有待办任务 + 无阻塞项 |
| 编码 → 测试 | 软门禁 | Developer 已打 handoff tag + 代码可编译运行 + 无阻塞项 |
| 测试 → Review | 软门禁 | `docs/09_test_report.md` 状态"已确认" + 无阻塞项 |
| Review → 发布 | 自动化门禁 | 测试全部通过 + `docs/CHANGELOG.md` 已更新 + `docs/08_review.md` 无 Critical 未修复 |

- **硬门禁**：必须人工确认，AI 不得自行流转
- **软门禁**：AI 检查文档状态和阻塞项后自动流转
- **自动化门禁**：检查客观条件，全部满足才可发布

---

## AI 接手检查清单

任何 AI 开始工作前必须完成：

1. 阅读 `README.md` 了解项目背景与当前状态
2. 检查项目当前状态（`docs/06_tasks.md` 顶部"项目状态"区块）
   — 校验上游文档是否处于"已确认"状态、阻塞项是否为空；如不满足，必须停下来报错，不得继续
3. 阅读 `.ai/workflow.md` 了解工作流程
4. 确认自己的角色 (`.ai/roles/`)
5. 阅读角色对应的规则 (`.ai/rules/`)
6. 阅读 `docs/` 中相关文档
7. 阅读 `docs/06_tasks.md` 确认当前任务
8. 明确输入文档和输出文档
9. 开始工作
10. 工作完成后更新对应文档，并在 `docs/06_tasks.md` 项目状态区块推进当前阶段（更新"当前阶段""下一步行动""阻塞项"三个字段），再通知下个角色接手

**绝对禁止：跳过状态检查直接开工。上游文档未处于"已确认"状态时，不得开始本阶段工作。**

---

## 阶段流转图

```
[想法]
  │
  ▼
[PRD] ──────────────────→ [UI设计] (可并行)
  │                          │
  ▼                          │
[架构设计]                    │
  │                          │
  ├──→ [数据库设计]           │
  │                          │
  ├──→ [API设计]              │
  │                          │
  ▼                          ▼
[任务拆分] ←─────────────────┘
  │
  ▼
[编码] ←──→ [测试] (交叉进行)
  │
  ▼
[Review]
  │
  ▼
[发布]
```
