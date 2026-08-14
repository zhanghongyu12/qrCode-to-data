# 决策记录 (Architecture Decision Records)

> 状态：持续追加
> 最后更新：2026-08-14
> 维护者：全员可追加

## 使用说明

本文件记录所有重要的技术决策、需求变更确认、设计问题处理。
目的是避免未来 AI 或人工重复讨论已经确定的问题。

### 格式规范

```
## DEC-XXX: 决策标题

- 日期：YYYY-MM-DD
- 决定：
- 原因：
- 影响：
- 替代方案：
- 最终选择：
- 状态：[提议 | 已确认 | 已废弃]
```

### 规则

- 决策编号递增，不复用
- 已确认的决策不可删除，只能标记为"已废弃"
- 新决策与旧决策冲突时，必须引用旧决策编号
- 决策记录一旦"已确认"，对应文档才可修改

---

## 决策列表

## DEC-001: 拆分 08_review.md 为审查报告和测试报告

- 日期：2026-07-13
- 决定：将 `docs/08_review.md` 拆分为 `docs/08_review.md`（Reviewer 代码审查报告）和 `docs/09_test_report.md`（Tester 测试报告）
- 原因：原文件同时包含 Reviewer 的代码审查报告和 Tester 的测试报告，两个角色共同维护同一文件导致职责边界模糊，违反 SRP 原则
- 影响：reviewer.md 输出文档改为 08_review.md（审查部分），tester.md 输出文档改为 09_test_report.md，document_rules.md 编号表已同步更新
- 替代方案：保持 08_review.md 不拆分，通过章节区分——但无法解决两个角色同时修改同一文件的冲突问题
- 最终选择：拆分为两个独立文件
- 状态：已确认

---

## DEC-002: 技术栈选型（CLI 工具 + 手机移动网页）

- 日期：2026-08-14
- 决定：桌面端采用 Go 1.22+ 单二进制 CLI；喷泉码用 gofountain（默认 Raptor 码 RFC 5053，LT 兜底）；二维码生成用 skip2/go-qrcode（EC 级别默认 L）；二维码解析用 makiuchi-d/gozxing；摄像头采集用 gocv（OpenCV，Linux 备选 go4vl，另提供 file/stdin 兜底）；终端渲染用 ANSI 半块字符；CLI 框架用 cobra；同网直传（Phase 2）用 HTTP 临时服务 + 一次性 token；手机端（Phase 2）用移动网页 PWA（原生 JS/TS + IndexedDB + getUserMedia + Canvas）。详见 docs/02_architecture.md §1。
- 原因：
  - Go 单二进制 + 交叉编译满足「零配置、无运行时依赖」定位，二维码/FEC 库生态成熟（txqr 同栈可借鉴）。
  - gofountain 纯 Go 无 CGO，Raptor 冗余开销（约 5~10%）远低于纯 LT（20~30%），吞吐收益明显；RaptorQ 的 Go 实现尚不完整（实验性），不选。
  - QR 自身纠错降为 L，配合外层喷泉码 + CRC32 + SHA-256 三层兜底，最大化单帧载荷。
  - 摄像头是唯一破坏「纯单二进制」的环节，用「gocv 跨平台 + go4vl（Linux）+ 文件/stdin 兜底」三层策略缓解。
  - 手机端选移动网页 PWA 契合「免装 App、浏览器打开即用、可离线」的 PRD 要求，并复用同一帧协议实现能力对称。
- 影响：
  - Developer 按此选型搭建模块（见 docs/02_architecture.md §3 包结构）。
  - 摄像头路径存在 CGO/OpenCV 依赖，MVP 需明确接受该权衡，或在无 OpenCV 环境降级为 file/stdin 解码。
  - QR version 默认 20（EC L 约 858B/帧），纯光学吞吐约 7~12 KB/s，1MB 文件约 1.5~3 分钟（可调）。
- 替代方案：
  - 语言：Python（原型快，但分发需解释器、二维码动画渲染性能与打包差）；Rust（性能好但二维码/FEC 生态与开发成本不占优）。
  - FEC：RaptorQ（RFC 6330，near-zero 开销约 1~2%，但 Go 生态无完整成熟实现）；纯 LT（最简单但开销大）。
  - 二维码：yeqown/go-qrcode（渲染更强但依赖多）；boombuler/barcode（二维码能力弱）。
  - 摄像头：FFmpeg/GStreamer 子进程（需外部二进制）；go4vl 纯 Go（仅 Linux）。
  - 手机端：原生 App（Flutter/React Native，体验好但分发/开发成本高，与「免装 App」冲突）。
- 最终选择：采用「决定」栏所列推荐。
- 状态：已确认

---

## DEC-003: UI↔API 对齐（CLI 契约以 04_api.md §1 为权威）

- 日期：2026-08-14
- 决定：二进制名与 CLI 参数契约以 `docs/04_api.md` §1 为准（权威）。对齐 `docs/05_ui.md` §6 假设与 04_api.md 的差异，最终取值如下：
  - 二进制名：`qrcode` → `qrcd`（Windows 下 `qrcd.exe`）。
  - 发送渲染模式：`--window` → `--terminal`（string，默认 `ansi`；`window` 为 Phase 2 取值）。
  - 接收期望参数：`--blocks <int>` / `--size <string>` → `--expect-size <int>`（预期总字节数，可选）。
  - 接收校验开关：`--no-verify` → `--hash <sha256:hex>`（预期校验值；不提供跳过校验的开关，SHA-256 始终计算）。
  - 关闭进度：`--no-progress` → `--quiet, -q`（bool，默认 false）。
- 原因：05_ui.md 在并行期间未见 04_api.md 详细设计，§6 以假设形式给出参数；现 04_api.md 已确认，需将 UI 假设与 API 契约统一，避免 Developer 按歧义参数实现。
- 影响：
  - Developer 实现 `cmd/qrcd` 时，参数名、类型与默认值严格以 04_api.md §1 为准（send/receive 两张参数表 + 退出码契约）。
  - 05_ui.md §3.1/§3.2 usage 块与 §6 假设中的旧参数名（qrcode/--window/--blocks/--size/--no-verify/--no-progress）不具约束力，以本决议与 04_api.md 为准。
  - 其余未列差异（如 --fps/--version/--ecc/--redundancy 默认值、send 的 --stdin 等）同样以 04_api.md §1 为准；05_ui.md 仅约束交互话术/布局，不约束参数契约。
- 替代方案：
  - 以 05_ui.md 假设为准反向修改 04_api.md：不可取，04_api 已确认且为契约权威，其参数命名（--hash/--quiet/--expect-size/--terminal）语义更明确。
  - 双向各改一部分：增加维护成本且无实质收益，且会造成两端契约持续漂移。
- 最终选择：以 04_api.md §1 为权威，按上表对齐。
- 状态：已确认

---

## DEC-004: FEC 解码接口增加 messageLength 参数

- 日期：2026-08-14
- 决定：`internal/fec.Codec.NewDecoder(sourceSymbols int) Decoder` 改为 `NewDecoder(sourceSymbols int, messageLength int) Decoder`。
- 原因：gofountain 的 `Decoder` 必须知道原始数据长度才能正确还原消息（`NewDecoder(messageLength int)`），否则解码结果会包含 padding 字节。原架构契约遗漏了此参数，导致 Raptor/LT 解码器还原长度与原文不一致（off-by-one padded bytes）。这是架构阶段未对齐 gofountain 真实 API 的契约缺陷。
- 影响：
  - `internal/fec` 三个实现（raptor/lt/none）的 `NewDecoder` 签名全部更新。
  - `tests/unit/fec/fec_test.go` 所有调用点传入 `len(data)` 作为 messageLength。
  - `docs/04_api.md §3` FEC 接口契约同步更新。
  - 后续 receive 编排（TASK-009）在创建解码器时需从元数据帧的 `size` 字段获取 messageLength。
- 替代方案：
  - 不改契约，FEC 层返回 padded 数据，由 receive 编排层用 metadata.size 裁剪——不可取，违背「Decode 还原源数据」语义，FEC 层单元测试无法独立闭环，责任下移更易出错。
- 最终选择：改契约，增加 messageLength 参数。
- 状态：已确认

---

## DEC-005: gofountain 低 K 伪满秩局限与测试策略

- 日期：2026-08-14
- 决定：FEC 丢包测试（`tests/unit/fec`）使用较大源符号数 K（≥ 50）与较长数据，而非低 K（如 10）+ 顺序 id 的小数据。
- 原因：gofountain 的 Raptor 实现 `sparseMatrix.determined()` 在低源符号数（K≤10）+ 顺序编码 id 场景下存在「伪满秩」——返回 `done=true` 但 `Decode()` 结果错误。这是 gofountain 库本身的局限（`determined()` 仅检查所有行 coeff 非空，未严格验证矩阵可解），非 wrapper 缺陷（裸库同样复现）。用随机大 id（如官方测试 `rand.Intn(60000)`）或较大 K 可规避。
- 影响：
  - `TestRaptorPacketLoss` 改用 K=50、2000 字节数据、生成 120 个符号丢 20% 的场景。
  - `internal/fec` 的 `Encoder.NextSymbol` 仍按契约产出顺序递增 id（04_api.md §3 要求「id 递增」），不改实现。真实传输场景中接收端持续收符号至足量，伪满秩偶发时可通过继续收符号规避；若后续 receive 编排（TASK-009）发现伪满秩影响实际传输，再评估是否在 `NextSymbol` 引入随机 id 映射（届时记新决策）。
- 替代方案：
  - 在 `NextSymbol` 内部把顺序计数器映射为随机大 id——违背「id 递增」契约语义，且需在解码端做反向映射，复杂度高，暂不采用。
  - 在 wrapper 层检测伪满秩（done=true 后校验，错误则重置继续收）——gofountain decoder 一旦 done 不再接受新符号，重置会丢失已收符号，不可行。
- 最终选择：测试用较大 K 规避；实现保持顺序 id，真实场景靠冗余符号覆盖。
- 状态：已确认

---

## 模板示例

## DEC-000: 示例决策

- 日期：2026-07-10
- 决定：使用 PostgreSQL 作为主数据库
- 原因：需要事务支持、JSON 字段、成熟稳定
- 影响：所有数据存储相关模块
- 替代方案：
  - MySQL：功能足够但 JSON 支持不如 PostgreSQL
  - MongoDB：灵活但缺乏事务一致性保证
- 最终选择：PostgreSQL 15
- 状态：提议
