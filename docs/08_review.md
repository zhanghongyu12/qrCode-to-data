# 代码审查报告

> 状态：已确认
> 最后更新：2026-08-17
> 维护者：Reviewer
> 审查范围：阶段 6 编码产出 + 阶段 7 测试产出（commit `0be4bee` 及此前 `e8bdc79`）

## 1. 审查范围

| 模块 | 文件 | 行数(估) |
|------|------|----------|
| 帧协议 | internal/frame/{frame,build,session}.go | ~330 |
| 喷泉码 | internal/fec/{fec,raptor,lt,none}.go | ~280 |
| 二维码 | internal/qrcode/{qrcode,render,decode}.go | ~300 |
| 载荷 | internal/payload/{load,write,hash,chunk,summary}.go | ~220 |
| 采集 | internal/capture/{capture,camera_stub,camera_cgo}.go | ~200 |
| 发送 | internal/send/{send,stream}.go | ~520 |
| 接收 | internal/receive/{receive,processor}.go | ~530 |
| 进度 | internal/progress/{progress,format,monitor}.go | ~250 |
| CLI | cmd/qrcd/main.go | ~230 |
| 测试 | tests/unit/* + tests/integration/* + tests/e2e/* | ~1100 |

## 2. 问题列表

### 严重 (Critical)
无。

### 缺陷修复（阶段 9 发布门禁复跑发现，已修复）

**T-02（已修复）：gozxing 纯二维码大尺寸误检导致随机丢帧**
- 位置：`internal/qrcode/decode.go` `DecodeImageBytes`
- 问题：缺 `PURE_BARCODE` 提示时，gozxing HybridBinarizer 对 ≥~315px 的纯二维码误估模块数（`NotFoundException: dimension = 75`），丢弃约 8% 的帧；每帧帧头含 crypto 随机 `transfer_id` → 命中帧随机 → `tests/integration` 跨运行随机失败（报「符号不足」）。
- 根因定位：逐层注入诊断（渲染对照 skip2 原生、像素尺寸矩阵、PURE_BARCODE 对照），排除 FEC/帧协议/渲染，确认为 gozxing 解码侧局限。
- 修复：提示集新增 `DecodeHintType_PURE_BARCODE`（DEC-006）。修复后 scale 3/8/16 全解，集成/E2E 多轮复跑全绿。
- 订正：早期 `docs/09_test_report.md` 记录「全绿」系特定运行偶然命中（恰好该会话未命中误检帧），实际为随机失败；本次订正为「存在随机失败并已修复」。

### 警告 (Warning)

**W-01：`qrcodeMaxCapacity` 包级 `capacityCache` map 无并发保护**
- 位置：`internal/send/stream.go` `capacityCache` 变量
- 问题：包级 `map[[2]int]int` 无 mutex 保护，`qrcodeMaxCapacity` 并发调用会 data race。
- 影响：当前 `send.Send` 单线程调用，无实际触发；但若未来并发发送或多 goroutine 复用，存在 race。
- 建议：加 `sync.RWMutex` 或改 `sync.Map`。MVP 不阻塞。

**W-02：`Processor.Process` 无锁，`Stats`/`Result` 有锁**
- 位置：`internal/receive/processor.go`
- 问题：`Process` 写 `unique`/`dedupTotal`/`expected` 等字段未持 `mu`，而 `Done`/`Result`/`Stats` 持锁。若外部并发调用 `Stats` 与 `Process` 会 race。
- 影响：`receive.Receive` 循环单线程串行调用 `Process`，同循环内检查 `Done`，无实际并发；测试亦在循环内调用。MVP 安全。
- 建议：统一加锁，或在文档注明 `Processor` 非并发安全。

**W-03：载荷与帧源全量入内存**
- 位置：`internal/payload/write.go` `WriteFile`（`data []byte` 整体写）、`internal/capture/capture.go` `fileSource`（一次性加载全部图片到 `[]Frame`）
- 问题：大文件/大目录全部读入内存，内存占用 = 载荷大小。
- 影响：MVP 传输量级（KB~MB）无压力；GB 级载荷会 OOM。
- 建议：Phase 2 或大文件场景改流式落盘/逐帧读取。MVP 不阻塞。

### 建议 (Suggestion)

**S-01：`--hash` 仅校验长度未校验 hex 字符**
- 位置：`cmd/qrcd/main.go` `validateReceiveOpts`
- 问题：`len(hex) != 64` 判定，未校验是否全为十六进制字符（如 `sha256:zzz...` 64 个非 hex 字符也通过）。
- 影响：非法 hex 进入接收端后由实际 hash 比较拦截（不匹配报错），功能正确但报错时机晚、提示不精确。
- 建议：补 `encoding/hex.DecodeString` 校验。低优先。

**S-02：`receive.CheckTimeout` 警告格式**
- 位置：`internal/receive/receive.go`
- 问题：`fmt.Fprintln(os.Stderr, "警告:", terr)` 输出 `警告: <err>`，格式可用但略糙。
- 影响：无功能影响。
- 建议：统一错误输出格式。

**S-03：CRC 校验失败静默丢帧**
- 位置：`internal/receive/processor.go` `Process`（`UnmarshalFrame` 失败 `return nil`）
- 问题：CRC/magic 校验失败的帧静默丢弃，无日志。
- 影响：符合设计（坏帧丢弃，不中断传输），但调试时难定位丢帧原因。
- 建议：`--verbose` 模式下 log 丢帧。MVP 可接受。

## 3. 安全发现

| 项 | 结果 |
|----|------|
| 路径穿越 | ✓ `payload.OutputPath` 用 `filepath.Base(name)` 清洗文件名，输出路径受 `--output` 显式控制 |
| 完整性校验 | ✓ CRC32(IEEE,大端) 校验头+payload，SHA-256 校验整体载荷；校验失败不落盘最终文件 |
| 随机数 | ✓ `transfer_id` 用 `crypto/rand` 生成 128bit 会话 ID |
| 注入面 | ✓ 无 SQL/无 shell exec（除 e2e 测试 `go build`）、无反序列化不可信数据 |
| 凭据泄露 | ✓ 无硬编码密钥，token 机制为 Phase 2 未实现 |
| 临时文件 | ✓ `.part`→rename 原子落盘，失败清理 `.part` |

## 4. 性能发现

| 项 | 结果 |
|----|------|
| QR 容量计算 | ✓ `qrcodeMaxCapacity` 二分实测 + `capacityCache` 缓存（见 W-01 并发隐患） |
| FEC 解码 | ✓ 复用单一 gofountain decoder 增量 `AddBlocks`（DEC-005 修复，无重建重投喂污染） |
| 帧播放节流 | ✓ `send.play` 用 `time.Ticker` 按 `--fps` 节流 |
| 内存 | ⚠ 全量入内存（见 W-03），大文件场景需优化 |
| goroutine | ✓ 无泄漏；`signal.NotifyContext` + ticker 均正确释放 |

## 5. API 契约一致性

| 契约（04_api） | 实现一致性 |
|----------------|-----------|
| §1.1 CLI 规范 | ✓ `qrcd send`/`receive`，中文帮助/错误 |
| §1.2 退出码 0/1/2/3 | ✓ `exitCodeFor` 映射正确，E2E 测试覆盖 |
| §1.3 send flags | ✓ 名称/默认值对齐（`--timeout` 30s 已修） |
| §1.4 receive flags | ✓ 名称/默认值对齐 |
| §2 帧协议 32B 头+payload+4B CRC32 | ✓ `MarshalFrame`/`UnmarshalFrame` 一致，大端 IEEE |
| §2.3 元数据 JSON 字段 | ✓ `MetaData` 结构体字段完整 |
| §2.5 短文本 ≤200B 直传 | ✓ `ShortTextLimit=200`，F-05 路径 |
| §3 FEC 接口签名 | ✓ `Codec`/`Encoder`/`Decoder`，`NewDecoder(k, msgLen)`（DEC-004） |

## 6. 编码规范

| 项 | 结果 |
|----|------|
| 命名清晰 | ✓ 中英结合，角色/包职责清晰 |
| 注释 | ✓ 复杂逻辑（FEC 解码/CRC/ISO-8859-1 二进制还原）有解释性注释 |
| 错误处理 | ✓ 哨兵错误分类（ErrUsage/ErrEnv/ErrTransfer），不吞致命错误（丢帧 return nil 符合设计） |
| 文件行数 | ✓ 多数 <300 行；send.go/stream.go/processor.go 略超但职责单一 |
| 单函数行数 | ✓ 均 <50 行 |
| 依赖单向 | ✓ internal 包无循环依赖（frame←fec←send/receive←cmd） |

## 7. 总体评价

代码实现**符合架构设计（02_architecture）与接口契约（04_api）**，质量良好：
- 帧协议、FEC、QR 编解码、载荷读写各层职责清晰，接口隔离到位（capture 抽象 Source 屏蔽 gocv）。
- 安全性达标：路径清洗、CRC/SHA-256 双重校验、crypto/rand 会话 ID、原子落盘。
- 测试覆盖完整：单元 7 包 + 集成 8 例 + E2E 10 例，全绿。
- 无 Critical 问题，3 个 Warning 均为并发/内存优化项，MVP 范围内不阻塞发布。

## 8. 下一步建议

1. **可进入阶段 9 发布**：无 Critical 未修复，满足"测试→Review"与"Review→发布"门禁。
2. Warning 项建议在 Phase 2 或大文件支持时处理（W-01 并发锁、W-03 流式落盘）。
3. Suggestion 项（S-01 hex 校验等）可纳入后续小版本打磨。
4. 发布前需处理：360 信任区配置（测试环境）、摄像头真实闭环验证（待实机）。
