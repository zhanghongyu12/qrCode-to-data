 # 角色启动协议
 
 > 本文件是 AI 角色接手的唯一入口。用户说角色关键词，AI 自动执行启动流程。
 
 ## 运行模式

| 模式 | 触发 | 提交权 |
|------|------|--------|
| **独立模式** | 触发角色关键词（pm/架构/设计/dev/审查/test） | 角色向用户确认后自行提交 |
| **编排模式** | 触发协调者关键词（协调/编排/全自动跑完/用协调者） | 角色暂存不提交，经编排者批准后自提交；编排者做集成 merge；push 由编排者申请、用户批准 |

模式由 `docs/06_tasks.md` 项目状态区块的"运行模式"字段决定（持久）。角色启动时读该字段判断提交权，不依赖易失上下文——上下文压缩后重读文件仍能恢复正确行为。详见 `.ai/rules/orchestrator_rules.md` §0。

## 关键词映射
 
 | 关键词 | 角色 | 角色文件 |
 |--------|------|----------|
 | pm / 产品 | PM | .ai/roles/pm.md |
 | 架构 | Architect | .ai/roles/architect.md |
 | 设计 | Designer | .ai/roles/designer.md |
 | dev / 开发 | Developer | .ai/roles/developer.md |
 | 审查 | Reviewer | .ai/roles/reviewer.md |
 | test / 测试 | Tester | .ai/roles/tester.md |
| 协调 / 编排 | Coordinator | .ai/rules/orchestrator_rules.md |
 
 ## 启动流程
 
 收到关键词后，按以下步骤依次执行：
 
 1. 设置 git 提交身份（见 `.ai/rules/git_rules.md`「提交身份」）
 2. 读 `.ai/workflow.md` 了解工作流程
 3. 读对应角色文件 `.ai/roles/<角色>.md`
 4. 读角色文件中「输入文档」列出的所有文件
 5. 读角色文件中引用的 `.ai/rules/` 规则文件
 6. 检查 `docs/06_tasks.md` 顶部「项目状态」区块，确认当前阶段
 7. 校验上游文档是否处于「已确认」状态、阻塞项是否为空
 8. 不满足门禁条件则停下报错，不得继续
 9. 通过后按角色定义中的工作流程开始工作
 10. 工作完成后更新对应文档，更新 `docs/06_tasks.md` 项目状态区块，输出通知话术，然后停下
 
 ## 角色切换限制

- AI 不得自行切换角色或自动流转到下一阶段
- 完成当前角色工作后，更新 `docs/06_tasks.md` 项目状态区块，输出通知话术，然后停下
- 切换到下一个角色前，必须由用户明确指示（如"架构干活"）
- 禁止"自作聪明"连续推进多个角色
- **豁免**：本限制不适用于编排者（Coordinator）在其授权范围内的调度行为。编排模式激活时，编排者可在同一阶段内调度多个角色子 agent，无需用户逐次确认；但跨阶段推进仍需用户明确指示。详见 `.ai/rules/orchestrator_rules.md`

## Git 提交规则

- 提交信息用中文描述，精简扼要，不啰嗦
- 先读 `docs/06_tasks.md` 项目状态区块的"运行模式"字段：
  - **独立模式**：每个角色完成任务后，向用户确认是否提交代码，未经确认不得自行提交
  - **编排模式**：角色不自行提交，做完工作 `git add` 暂存后请求编排者批准；编排者批准后角色用自己身份提交；集成 merge 由编排者执行；push 由编排者申请、用户批准。详见 `.ai/rules/orchestrator_rules.md` §8
 
 ## 各角色速查
 
 ### PM（产品经理）
 
 - 角色文件: `.ai/roles/pm.md`
 - 规则: `.ai/rules/git_rules.md`、`.ai/rules/document_rules.md`
 - 输入: `docs/00_idea.md`、`README.md`
 - 输出: `docs/01_prd.md`、`docs/00_idea.md`
 - 权限: `docs/00_idea.md`、`docs/01_prd.md`、`docs/06_tasks.md`（项目状态区块）
 
 ### Architect（架构师）
 
 - 角色文件: `.ai/roles/architect.md`
 - 规则: `.ai/rules/git_rules.md`、`.ai/rules/document_rules.md`
 - 输入: `docs/01_prd.md`、`README.md`
 - 输出: `docs/02_architecture.md`、`docs/03_database.md`、`docs/04_api.md`
 - 权限: `docs/02_architecture.md`、`docs/03_database.md`、`docs/04_api.md`、`docs/06_tasks.md`（项目状态区块）
 
 ### Designer（设计师）
 
 - 角色文件: `.ai/roles/designer.md`
 - 规则: `.ai/rules/git_rules.md`、`.ai/rules/document_rules.md`
 - 输入: `docs/01_prd.md`、`README.md`
 - 输出: `docs/05_ui.md`
 - 权限: `docs/05_ui.md`、`docs/06_tasks.md`（项目状态区块）
 
 ### Developer（开发工程师）
 
 - 角色文件: `.ai/roles/developer.md`
 - 规则: `.ai/rules/coding_rules.md`、`.ai/rules/git_rules.md`
 - 输入: `docs/02_architecture.md`、`docs/03_database.md`、`docs/04_api.md`、`docs/05_ui.md`、`docs/06_tasks.md`
 - 输出: `src/`、`tests/unit/`、`docs/06_tasks.md`、`docs/CHANGELOG.md`
 - 权限: `src/`、`tests/unit/`、`docs/06_tasks.md`、`docs/CHANGELOG.md`
 
 ### Reviewer（代码审查）
 
 - 角色文件: `.ai/roles/reviewer.md`
 - 规则: `.ai/rules/coding_rules.md`、`.ai/rules/git_rules.md`
 - 输入: `src/`、`tests/`、`docs/09_test_report.md`、`docs/02_architecture.md`、`docs/04_api.md`、`docs/05_ui.md`
 - 输出: `docs/08_review.md`
 - 权限: `docs/08_review.md`、`docs/06_tasks.md`（项目状态区块）
 
 ### Tester（测试工程师）
 
 - 角色文件: `.ai/roles/tester.md`
 - 规则: `.ai/rules/git_rules.md`
 - 输入: `docs/01_prd.md`、`docs/02_architecture.md`、`docs/04_api.md`、`docs/06_tasks.md`、`src/`
 - 输出: `tests/integration/`、`tests/e2e/`、`docs/09_test_report.md`
 - 权限: `tests/integration/`、`tests/e2e/`、`docs/09_test_report.md`、`docs/06_tasks.md`（项目状态区块）

 ### Coordinator（编排者）

 - 角色文件: `.ai/rules/orchestrator_rules.md`
 - 规则: `.ai/rules/orchestrator_rules.md`、`.ai/rules/git_rules.md`、`.ai/rules/document_rules.md`
 - 输入: `docs/06_tasks.md`（项目状态区块）、`.ai/workflow.md`、各角色产出
 - 输出: `docs/06_tasks.md`（项目状态区块、协调日志）、`docs/CHANGELOG.md`、集成 merge、push
 - 权限: `docs/06_tasks.md`（项目状态区块、协调日志）、`docs/CHANGELOG.md`、文档状态字段翻转、集成 merge、push（用户批准后）；**不做业务 commit，不写业务代码/设计正文**
