# 决策记录 (Architecture Decision Records)

> 状态：持续追加
> 最后更新：2026-08-17
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

## DEC-006: gozxing PURE_BARCODE 提示修复纯二维码大尺寸误检

- 日期：2026-08-17
- 决定：`qrcode.DecodeImageBytes` 的 gozxing 解码提示集新增 `DecodeHintType_PURE_BARCODE`（保留既有 `CHARACTER_SET=ISO-8859-1`、`TRY_HARDER`）。
- 原因：阶段 9 发布门禁复跑发现 `tests/integration`（`TestReceiveFromFileSource`、`TestTextAndBinaryPayload/binary`）随机失败，报「符号不足」。逐层注入诊断复现，确认根因不在 FEC/帧协议/渲染：
  - gozxing 的 HybridBinarizer 在**不含 PURE_BARCODE 提示**时，对**较大尺寸（≥~315px）的纯二维码**会误估模块数（抛 `NotFoundException: dimension = 75` 等幻影探测），丢弃约 8% 的帧。
  - 误检**内容相关**：每帧帧头携带 crypto 随机 `transfer_id`，不同传输会话产生不同模块图案 → 命中误检的帧随机 → 跨运行随机失败。
  - 像素尺寸矩阵：scale 1/2（≤210px）可解，scale≥3（≥315px）误检；`mine` 与 `skip2` 原生渲染行为一致 → 非本项目渲染缺陷，为 gozxing 解码侧局限。
  - 加入 `PURE_BARCODE` 后 scale 3/8/16 全部稳定可解；多轮复跑（集成/E2E 各 4+ 轮）全绿。
- 影响：`internal/qrcode/decode.go`（`DecodeImageBytes` 提示集 + 解释性注释）。`render.go` 不变。FEC/帧协议不变。本工具传输模型为「整帧即单枚二维码」，PURE_BARCODE 语义成立。
- 替代方案：
  - 测试夹具改用小渲染尺寸（scale≤2）：仅掩盖问题，不改善真实摄像头路径（实机采集为大图），不采用。
  - 二进制帧 Base64 包裹再入码：诊断证明误检在图像检测层而非字节层，同帧仍误检，不采用。
  - 切换/分叉 skip2 或 gozxing：成本高，超出 MVP 范围，不采用。
- 关联：DEC-005（gofountain 低 K 伪满秩，独立问题，本次未触发）；早期 `docs/09_test_report.md` 记录「全绿」系特定运行偶然命中，实际为随机失败，本次订正。
- 最终选择：`DecodeImageBytes` 启用 `PURE_BARCODE`。
- 状态：已确认

---

## DEC-007: 手机端从 jsQR 网页改为原生 Android App（ML Kit）

- 日期：2026-08-17
- 决定：手机中继端放弃 jsQR（getUserMedia 网页）方案，改为原生 Android App：CameraX（ImageAnalysis）+ Google ML Kit Barcode Scanning + OkHttp 转发。接收端 PC 不变（网络接收，无摄像头）。Web 服务（`qrcd web`）提供 `/sender`、`/receiver`、`/relay`（兼容旧网页）、`/`（首页含 APK 下载二维码）、`/dl/app.apk`（APK 下载）。
- 原因：jsQR 对 QR byte-mode 二进制帧（含高字节 ≥128）系统性解码失败，仅可靠解码可打印 ASCII；即使 base64 包裹，又导致单帧体积超出 QR 容量（v20 Q 482B）。网页 getUserMedia 还需 HTTPS 安全上下文，手机访问证书自签页面受限。ML Kit 为系统级离线扫码引擎，`barcode.rawBytes` 直出二进制帧字节，识别率与稳定性远超 jsQR，且免 HTTPS（App 本地权限）。原决策 DEC-002 的「手机端用移动网页 PWA」在二进制帧场景下不可行，本次推翻手机端载体选择。
- 影响：
  - 新增 `android/` 子项目（Gradle 8.14.3 / AGP 8.13.0 / Kotlin 2.0.21 / minSdk 24 / targetSdk 34），依赖 camera 1.3.4、mlkit barcode-scanning 17.3.0、okhttp 4.12.0、appcompat 1.7.0。
  - App 功能：手动「开始扫描」+「发送到 PC」+「清空」三按钮触发（非自动转发），即时反馈「已捕获 N 块」；dedupKey 整帧哈希去重（见 DEC-010）；帧类型区分（type=0x02 数据帧计入捕获计数，type=0x01 元数据帧仍上传但不计数，见 DEC-009）。
  - `.gitignore` 增加 `android/.gradle/`、`android/app/build/`、`/qrcd-app-debug.apk` 等。
  - DEC-002 中「手机端 PWA」标记为被本决策取代（保留 DEC-002 桌面端 Go CLI / FEC / QR 选型）。
- 替代方案：
  - 继续用 jsQR 网页 + base64 包裹：容量/可靠性两难，DEC-008 之前已验证失败。
  - 换更强 JS 扫码库（zxing-js）：仍受 HTTPS 安全上下文与网页解码性能限制。
  - 跨平台 App（Flutter/React Native）：分发与开发成本高，ML Kit 集成复杂度上升。
- 关联：DEC-002（手机端载体选型，本决策取代其手机端部分）、DEC-008（QR 密度/单帧载荷，配套 MaxSymbol 限制）、DEC-009（三端计数对齐）、DEC-010（dedupKey 全帧哈希）。
- 最终选择：原生 Android App + ML Kit。
- 状态：已确认

---

## DEC-008: 单帧符号字节上限 MaxSymbol 限制 QR 密度以提升手机扫码率

- 日期：2026-08-17
- 决定：`send.Options` 新增 `MaxSymbol int`（单符号字节上限，0=按 Version 容量自适应）。`BuildStream` 计算 `dataMax = min(maxPayload, opts.MaxSymbol)`，符号大小循环以 `dataMax` 为上限。Web 发送端 `handleEncode` 固定 `MaxSymbol: 151`（数据帧 → v12 Q，65×65 模块，手机摄像头可稳定识别）。元数据帧仍用 `Version: 40`（按文件名长度自动选最小版本，避免长文件名超出 v15 Q 容量 292B）。
- 原因：原 `Version: 15` 数据帧约 v20（97×97 模块）过密，手机摄像头实测大量失败，文件传输仅扫到少量块即卡死。降密度到 v12（MaxSymbol=151）后手机稳定扫到。DEC-002 设想的 v20 单帧 858B 在实机扫码率上不可行。
- 影响：
  - `internal/send/send.go` Options 增 `MaxSymbol` 字段；`stream.go` BuildStream 用 dataMax 限制符号大小。
  - 元数据帧因文件名长度可变，单独 `Version: 40`（auto-select min），此前用 v15 时长文件名超容报 500。
  - 单帧载荷变小 → 大文件块数激增，与 DEC-011（Raptor 单会话 K 上限，实现修订后取 K≤1024）冲突，需 DEC-012 多会话分片解决。
- 替代方案：
  - 保持高密度 v20：实机扫码失败，不可行。
  - 动态探测手机能力自适应版本：实现复杂且无可靠探测手段。
- 关联：DEC-007（App 扫码）、DEC-011/DEC-012（大文件块数上限）。
- 最终选择：MaxSymbol=151（数据帧 v12 Q）。
- 状态：已确认

---

## DEC-009: 三端计数对齐（进度基准改为 TotalSymbols，完成后继续累计）

- 日期：2026-08-17
- 决定：
  - `frame.MetaData` 新增 `TotalSymbols int`（发送端计划发送的编码符号总数，含冗余，不含元数据重播帧）。`send.BuildStream` 填充为 `totalData`。
  - 接收端进度基准从 `BlockCount`（K，源块数）改为 `TotalSymbols`（含冗余）。`Processor.Process` 中 `expected = meta.TotalSymbols`（回退 BlockCount）。
  - `Processor.Process` 去掉「done 后直接 return」的提前退出：还原完成后仍继续去重计数（unique 继续增长），使接收端「已收」追上手机实际扫描数。
  - 发送端 `/api/encode` 响应增 `symbols` 字段；发送端页面显示「块数据符号数」（与接收端 total 一致），内部播放仍用含元数据的 `frames`。
  - 接收端完成提示显示「已收 X/Y 块」。
  - 手机端只对数据帧（type=0x02）计入「已捕获 N 块」，元数据帧仍上传但不计数。
- 原因：喷泉码凑齐 K 个源块即解码完成并标记 done，之后到达的符号被 `if p.done { return nil }` 丢弃，unique 停在 K。导致三端数字差异巨大（发送端 95 帧 / 手机 91 块 / 接收端 72），用户误以为「扫描或传输不彻底」。这是喷泉码正常行为（冗余即为此设计），但显示口径不一致造成困惑。统一到「数据符号数（含冗余）」基准后，三端数字可比、完成后接收端继续累计追平手机扫描数。
- 影响：
  - `internal/frame/frame.go` MetaData 增 `TotalSymbols`；`internal/send/stream.go` 填充；`internal/receive/processor.go` 进度基准与 done 后计数逻辑；`internal/web/web.go` encode 响应 + `internal/web/pages.go` 三端显示。
  - 新增测试 `tests/unit/sendreceive/TestPostDecodeCounting`：验证 TotalSymbols>K、完成后继续投喂 unique 增长、进度基准为 TotalSymbols。
  - 文件完整性仍由 SHA-256 校验保证，计数对齐不改变还原正确性。
- 替代方案：
  - 接收端 total 仍用 K 并显示「还原完成即可」：用户困惑「为何只到 72」不消除。
  - 让接收端 total 含元数据重播帧（= 发送端 frames 95）：元数据帧是控制帧，混入数据计数语义不清；且手机已改为不计数元数据帧，反而又错位。
- 关联：DEC-010（dedupKey，配合去重正确性）、DEC-007（手机 App 计数）。
- 最终选择：基准统一为 TotalSymbols，完成后继续累计。
- 状态：已确认

---

## DEC-010: dedupKey 整帧哈希去重（修复数据帧误判为重复）

- 日期：2026-08-17
- 决定：手机端 `dedupKey` 从「前 16 字节 + 长度」改为「整帧哈希」（遍历全帧字节，`h = h*31 + byte`，含长度初值）。
- 原因：早期 dedupKey 只哈希前 16 字节 + 长度。帧头布局：magic[0:4] + version[4] + type[5] + flags[6:8] + transfer_id[8:24] + seq[24:28] + len[28:32]。同一会话所有数据帧的前 16 字节（magic+version+type+flags+transfer_id 前 8 字节）完全相同，且等长（同 symbolSize）→ 所有数据帧 dedupKey 相同 → 除第一个数据帧外全部被去重丢弃，手机只捕获到 2 块（meta + 1 data）。改为整帧哈希后，不同 seq 的数据帧哈希不同，去重正确。
- 影响：`android/.../MainActivity.kt` dedupKey 函数。Go 侧接收端去重本就用 seq（`Session.MarkReceived`），无误判。
- 替代方案：仅把 seq 字节纳入哈希（offset 24:28）：可行但脆弱（依赖帧头偏移常量）；整帧哈希更稳健且帧很小（~187B）无性能顾虑。
- 关联：DEC-007（手机 App 去重）、DEC-009（计数对齐）。
- 最终选择：整帧哈希。
- 状态：已确认

---

## DEC-011: Raptor 单会话 K 上限与 QR 单帧容量的内在约束（gofountain 实现 K≤1024）

- 日期：2026-08-17（修订：2026-08-20）
- 决定：记录（非新增实现）喷泉码单会话源块数上限约束。Raptor 码（RFC 5053）理论 K∈[4,8192]，但 gofountain 实现中 K 接近 8192 时内部矩阵求解会越界 panic（实测 K=4096 约 2s 编码，K=8192 不可用），**实际安全上限取 K≤1024**（K=1024 编码约 110ms，O(K²)）。在手机可扫的 QR 密度下（MaxSymbol=151，v12 Q，净 167B/帧），单会话最大可传 ≈ 1024×167 ≈ 167KB；即使密度提到 v20 L（净 822B），单会话最大 ≈ 1024×822 ≈ 822KB。8MB 文件在单会话内必然超限（8MB/151B≈55000 块），必须多会话分片。
- 原因：用户反馈 8MB 文件编码报 400（`send: 参数错误`），根因为 K 超 Raptor 上限。这是「手机可扫密度」与「喷泉码单会话容量」的内在矛盾，非 bug。
- 影响：大文件传输需 DEC-012 多会话分片解决；在此之前，单会话传输的合理上限约为 167KB（手机可扫密度）。建议大文件场景改用视频录像离线解析（DEC-013）+ 多会话分片。
- 替代方案：
  - 提高 QR 版本突破上限：手机扫码率随之崩溃，不可行。
  - 放弃喷泉码改可寻址分块（每帧带偏移，按序拼）：失去乱序/丢帧容错，与项目「光学传输可漏帧」定位冲突。
- 关联：DEC-008（MaxSymbol 密度）、DEC-012（分片方案）。
- 最终选择：接受 K≤1024 约束，以多会话分片突破。
- 状态：已确认

---

## DEC-012: 大文件多会话分片传输

- 日期：2026-08-17（修订：2026-08-20）
- 决定：发送端将大文件自动拆分为 N 个会话（每会话 ≤1024 块，各自独立 transfer_id 与元数据帧，元数据增 partIndex/partTotal），逐会话生成 QR 流；接收端按会话解码后按 partIndex 顺序拼接还原。配套 DEC-013 视频录像离线解析以提升大文件扫描可靠性。
- 原因：DEC-011 约束下单会话无法承载大文件，必须分片。各分片独立 transfer_id 复用现有 SessionManager 多会话能力。
- 影响：
  - `internal/frame/MetaData` 增分片字段：`partIndex`（int，0-based，omitempty）、`partTotal`（int，omitempty）、`overallName`（仅 part 0，omitempty）、`overallSize`（仅 part 0，omitempty）、`overallHash`（所有分片携带，作为整体关联键，omitempty）。`totalSymbols`（已有）。
  - `docs/04_api.md §2.3` 同步更新字段表、JSON 示例、分片语义。
  - `send.BuildStream` 改为多会话迭代器：自动计算分片数（`partSize = 1024 × symbolSize`，`partTotal = ceil(dataLen / partSize)`），逐会话编码。每会话独立 `transfer_id`、独立元数据帧（含分片级 name/size/hash + partIndex/partTotal + overallHash；仅 part 0 额外含 overallName/overallSize）。
  - `receive.Processor` 增分片缓存：按 `overallHash` 聚合各分片已还原字节，收齐 `0..partTotal-1` 后按序拼接 → 用 `overallHash` 校验整体 SHA-256 → 以 `overallName` 落盘（复用 `.part`→rename 原子落盘）。各分片可独立重发（独立 transfer_id，已有 SessionManager 多会话能力）。
  - 发送端 `handleEncode`/`/api/frame` 支持会话切换（web 页面的多会话进度）。
  - App 与网页适配多会话进度（显示「第 X/N 分片」+ 分片内块进度）。
  - `docs/07_decisions.md` DEC-011 同步订正 K 上限为 1024。
- 替代方案：见 DEC-011 替代方案。
- 关联：DEC-011（约束）、DEC-013（录像离线解析）。
- 最终选择：采用多会话分片方案；K≤1024；partIndex 0-based、omitempty；分片级 name/size/hash + overallHash 全分片关联 + part 0 额外 overallName/overallSize；接收端按 overallHash 聚合，收齐后拼接校验整体 SHA-256。
- 状态：已确认

### DEC-012 实现修订（2026-08-20 TASK-015）

实现期对上述契约做了两处细化，与原文略有出入，以本节为准：

1. **K 上限取 1024 而非 8192**：gofountain 在 K 接近 8192 时内部矩阵求解会越界 panic，且 O(K²) 编码代价在 K=4096 时已达约 2s。发送端以 `maxSourceK = 1024` 为单会话安全上限，`partSize = 1024 × symbolSize`（symbolSize 受 MaxSymbol 与 QR 版本容量约束），`partTotal = ceil(数据总长 / partSize)`。每分片分块大小自动取 `ceil(分片长 / 1024)`，满片时恰 K=1024。
2. **overallHash 为所有分片的关联键**：仅 part 0 携带 `overallName`/`overallSize` 的原始设计存在契约缺口——非 part 0 分片若先于 part 0 到达（网络乱序、分片重发），接收端无法把它们归入同一整体传输。实现修订为：**所有分片均携带 `overallHash`**（接收端按此键聚合），仅 part 0 额外携带 `overallName`/`overallSize`（整体文件信息，拼接落盘用）。
3. **接收端聚合键为 `overallHash` 而非 `transfer_id`**：各分片独立 transfer_id，接收端 `Processor.assemblies map[overallHash]*partAssembly` 按 `overallHash` 聚合，收集齐 0..partTotal-1 后按序拼接 → 整体 SHA-256 校验 → 以 `overallName` 落盘。
4. **Raptor 编码器预生成优化**：`NextSymbol` 首符号开销由逐符号重建 O(K³) 中间块改为一次性批量预生成基础冗余符号（O(K²)，K=1024 实测约 110ms），**但 NextSymbol 仍保持 04_api §3 契约的「可无限产出冗余符号」语义**：耗尽预生成符号后继续产出新冗余符号，不返回 nil。`TestNextSymbolFinite` 恢复为 `TestNextSymbolInfinite` 验证可无限产出。

---

## DEC-014: 桌面端原生窗口 + 系统托盘 + 三安装包拆分 + 中性命名

- 状态：已实现
- 决定：
  1. 桌面端改原生窗口承载（go-webview2，纯 Go 无 cgo），不再依赖浏览器；`-H windowsgui` 编译去除控制台黑窗。
  2. 右下角系统托盘常驻（getlantern/systray）：关窗不退，托盘右键「打开桌面端 / 退出」。
  3. 按角色拆三个独立安装包：`qrcd-a.exe`（播放端）、`qrcd-b.exe`（还原端）、`qrcd-relay.apk`（中继）；桌面 role 经 `-ldflags "-X main.role=a|b"` 编译，托盘图标中心大写字母 A/B/Q 区分。
  4. 中性命名（脱敏）：包名与界面不体现「发送/接收」——播放端/还原端/中继；发送→播放/输出、接收→还原、传输→交换、上传→提交、下载→保存。
- 原因：用户要求「拆成发送端、接收端、中继三个安装包，但包名与软件内部不体现发送/接收等敏感字样」；桌面端承载替代浏览器。
- 关键实现：
  - WebView2 窗口在 goroutine 中运行，`runtime.LockOSThread()` 绑定线程（go-webview2 `Run()` 不自锁线程，否则 Win32 消息循环线程漂移 → 窗口假死）。
  - 托盘（主 goroutine）与 WebView2 窗口（锁线程的 goroutine）各自独立消息循环。
  - 托盘图标为 32×32 32bpp ICO 内联生成：绿色圆角方块 + 白色大写字母（5×7 点阵 3× 放大）。
  - App 地址留空由默认 `localhost:8080` 改为提示填写（真机 localhost=手机自己）。
- 替代方案：托盘用原生 Shell_NotifyIcon syscall 自绘（省依赖但菜单/WM_COMMAND 处理繁琐）；桌面端用浏览器打开（依赖浏览器，非原生）。
- 关联：DEC-007（中继 App 基座）、DEC-008（QR 密度）、DEC-012（分片）。

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
