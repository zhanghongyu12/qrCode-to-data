# Session 背景

> Session ID: {session_id}
> 类型: 自动辩论
> 状态: 进行中
> 创建时间: YYYY-MM-DDTHH:MM+08:00
> 创建者: 人工
> 关联阶段: （如 阶段 3-4 并行设计）

## 讨论背景
（描述本次辩论要解决的问题，必填）

## 参与角色
- Expert A（工具名，如 Claude Code，窗口 1）
- Expert B（工具名，如 Codex，窗口 2）

## 输出目录
.session/{session_id}/

## 轮数预算
- 计划轮数: 
- MAX_ROUNDS: 
- 已延期次数: 0

## 墙钟
- 截止时间: YYYY-MM-DDTHH:MM+08:00
- 墙钟缓冲: 30秒
- 停止时间: 
- 已延长次数: 0

## 延期审批
- 审批结果: 待否决 | 已否决 | 无（未触发延期）

## 硬上限速查
| 约束 | 上限 |
|------|------|
| 初始预估轮数 | ≤ 20 |
| 单次轮数延期 | ≤ 6 轮 |
| 总延期次数 | ≤ 2 |
| 单次墙钟延长 | ≤ 60 分钟 |
| 总延长次数 | ≤ 2 |
| 初始墙钟时长 | ≤ 120 分钟 |
| 封顶轮数 | 32（20 + 6 + 6） |

## 文件角色（自动辩论模式）
- board.md → 对话记录 + 轮询信号（替代 discussion.md）
- current_round.txt → 当前已完成轮数（纯数字单行）
- max_rounds.txt → MAX_ROUNDS 封顶值（纯数字单行）
- turn.txt → 当前该谁发言（单字符 A 或 B）
- expert_a/b.md → Phase 1 独立输出，Phase 2 起只读
- context.md → 状态、轮数预算、截止时间、审批结果
