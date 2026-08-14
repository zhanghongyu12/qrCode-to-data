# Git Rules (Git 协作规则)

---

## 提交身份

每个角色开工前必须设置自己的 git author，使 `git log` 中每个 commit 都能追溯到角色和操作者。

### 格式

- `user.name = {角色}-{git用户名}`
- `user.email = {角色}-{git用户名}@{项目域名}`
- 项目域名取项目名小写 + `.local`（如 `ai-work-model.local`），可在 `docs/07_decisions.md` 中覆盖
- git 用户名从本地 git 配置读取（`git config user.name` 的原始值），如未配置则提示人工先设置基础 git 身份

### 角色对照表

| 角色 | user.name | user.email |
|------|-----------|------------|
| PM | PM-{git用户名} | pm-{git用户名}@{项目域名} |
| Architect | Architect-{git用户名} | architect-{git用户名}@{项目域名} |
| Designer | Designer-{git用户名} | designer-{git用户名}@{项目域名} |
| Developer | Developer-{git用户名} | developer-{git用户名}@{项目域名} |
| Reviewer | Reviewer-{git用户名} | reviewer-{git用户名}@{项目域名} |
| Tester | Tester-{git用户名} | tester-{git用户名}@{项目域名} |
| Coordinator | Coordinator-{git用户名} | coordinator-{git用户名}@{项目域名} |

### 并行分工时的实例标识

并行分工中，同角色多实例在 `user.name` 和 `user.email` 中加实例后缀：

- 发起方：`Developer-{git用户名}-A` / `developer-{git用户名}-a@{项目域名}`
- 辅助方：`Developer-{git用户名}-B` / `developer-{git用户名}-b@{项目域名}`

详见 `.ai/rules/parallel_split_rules.md`。

### 设置时机

每个角色开工前（工作流程第一步）确认 `git config user.name` 和 `git config user.email` 已设置为本角色身份。如已设置且正确则跳过。

### Coordinator 身份用途

Coordinator 身份**仅用于集成 merge / tag commit**（编排层动作）。业务 commit（src/tests 的实现提交）由各角色用自己的身份完成，不用 Coordinator 身份。详见 `.ai/rules/orchestrator_rules.md` §8。

## 分支策略

### 分支模型

```
main          ← 稳定发布分支，始终可运行
develop       ← 开发集成分支
feature/*     ← 功能分支，如 feature/user-auth
fix/*         ← 修复分支，如 fix/login-redirect
hotfix/*      ← 紧急修复分支，从 main 拉出
```

### 规则

- `main` 分支禁止直接提交，必须通过合并
- 功能开发在 `feature/*` 分支进行
- 合并到 `main` 前必须通过 Review
- 合并后删除功能分支

---

## 提交规范

### Commit Message

遵循 coding_rules.md 中的 Commit Message 格式。

### 提交粒度

- 一个提交只做一件事
- 提交前确保代码可编译/可运行
- 不要提交调试代码（console.log, print 调试等）
- 不要提交敏感信息（密钥、密码、配置中的真实凭据）

### 编排模式下的提交

- 角色做完工作 `git add` 暂存，**不自行 commit**，返回请求编排者批准
- 编排者审 `git diff --cached`（全文），批准后角色用自己身份 `git commit`
- 集成 merge 由编排者用 Coordinator 身份执行（`--no-ff`）
- push 是重大决策，编排者申请、用户批准后执行；阶段末批量 push
- 详见 `.ai/rules/orchestrator_rules.md` §8

---

## 禁止操作

以下操作对未提交改动是**不可逆**的，一旦执行无法恢复，严禁使用：

| 禁止操作 | 替代做法 |
|---------|---------|
| `git checkout HEAD -- <file>` 清理工作区 | `git stash`（可恢复） |
| `git reset --hard` 丢弃未提交改动 | `git stash` 或 `git reset --soft`（撤 commit 保留暂存，编排者回滚越界 commit 时用） |
| `git clean -fd` 清理未跟踪文件 | 先 `git status` 确认无他人改动，再谨慎执行 |
| `git branch -D` 强删分支 | 正常合并后 `git branch -d` |

通用原则：对非自己归属的文件执行任何 git 写操作前，先 `git status` 确认该文件无未提交改动。

---

## Worktree 用法

并行分工中使用 `git worktree` 创建独立工作空间，实现物理隔离：

```powershell
# 创建并行工作空间（在项目根目录执行）
git worktree add ../{项目名}-b feature/{协作名}-b

# 收尾后清理
git worktree remove ../{项目名}-b

# 查看所有 worktree
git worktree list
```

特点：
- 不同物理目录 checkout 不同分支，工作区完全隔离
- 共享同一 `.git` 仓库，两路 commit 互相即时可见，无需 push/pull
- 未提交改动物理隔离，不会被对方操作误伤

详见 `.ai/rules/parallel_split_rules.md`。

---

## 合并规则

- 合并前确保分支与目标分支同步
- 合并前必须通过测试
- 使用 `--no-ff` 合并，保留分支历史
- 合并信息包含关联的 Task ID

---

## Tag / Release

### 版本号

遵循语义化版本 (Semantic Versioning)：

```
MAJOR.MINOR.PATCH
```

- MAJOR：不兼容的 API 修改
- MINOR：向下兼容的功能新增
- PATCH：向下兼容的问题修复

### Release 流程

1. 确认所有测试通过
2. 更新 docs/CHANGELOG.md
3. 打 Tag：`v1.0.0`
4. 合并 develop → main

---

## .gitignore 原则

- 忽略所有依赖目录（node_modules/, venv/, __pycache__/ 等）
- 忽略构建产物（dist/, build/, *.pyc 等）
- 忽略环境配置（.env, .env.local 等）
- 忽略编辑器配置（.vscode/, .idea/ 等，除非团队共享配置）
- 不忽略文档文件
