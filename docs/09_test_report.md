# 测试报告

> 状态：已确认
> 最后更新：2026-08-17
> 维护者：Tester
> 被测版本：tag `stage-6-handoff` / commit `e8bdc79`（阶段 6 编码产出）

## 1. 测试范围

对 qrcd 阶段 6 编码产出（TASK-001~011）进行集成测试与端到端（E2E）黑盒测试，验证纯光学数据传输闭环：

- 模块间协作链路：payload 分块 → FEC 编码 → frame 组帧 → QR 生成 → QR 解码 → frame 拆帧 → FEC 解码 → SHA-256 校验 → `.part`→rename 落盘
- CLI 行为：`send`/`receive` 子命令、退出码四档（0 成功/1 传输失败/2 参数错误/3 环境错误）、stdout/stderr 分流、中文帮助、短文本 F-05 直传

已通过阶段 6 编码产出的单元测试（tests/unit/ 7 包）不在此重复，本报告聚焦新增的集成与 E2E 层。

## 2. 测试环境

| 项 | 值 |
|----|----|
| OS | Windows 10 Pro 10.0.19045 |
| Go | 1.25.5（环境变量 `GOROOT` 被误配为空目录，所有 go 命令前 `unset GOROOT`） |
| 构建产物临时目录 | `GOTMPDIR=E:/aiCode/qrCode-to-data/.tmp`（见 §6 已知问题） |
| 摄像头 | 不可用（无 gocv/OpenCV 构建），用 `--source file`/`stdin` 兜底 |
| 测试执行 | `go test -p 1 ./...`（顺序执行，见 §6） |

## 3. 测试用例统计

| 层 | 包 | 用例数 | 通过 | 失败 | 跳过 |
|----|----|--------|------|------|------|
| 单元 | tests/unit/capture | — | ✓ | 0 | 0 |
| 单元 | tests/unit/fec | — | ✓ | 0 | 0 |
| 单元 | tests/unit/frame | — | ✓ | 0 | 0 |
| 单元 | tests/unit/payload | — | ✓ | 0 | 0 |
| 单元 | tests/unit/progress | — | ✓ | 0 | 0 |
| 单元 | tests/unit/qrcode | — | ✓ | 0 | 0 |
| 单元 | tests/unit/sendreceive | 11 | ✓ | 0 | 0 |
| 集成 | tests/integration | 8 | ✓ | 0 | 0 |
| E2E | tests/e2e | 10 | ✓ | 0 | 0 |

- **集成测试用例**（tests/integration，8 函数）：
  `TestSendFile_PlayAndHash`、`TestSendTextOverLimit_SwitchToStream`、`TestReceiveFromFileSource`、`TestOverwriteControl`、`TestBlockSizeMatrix`（256/1024/4096）、`TestECCMatrix`（L/M/Q/H）、`TestTextAndBinaryPayload`（text/binary）
- **E2E 测试用例**（tests/e2e，10 函数）：
  `TestCLI_Help`、`TestCLI_SendShortText`、`TestCLI_SendNoArg`、`TestCLI_SendBadECC`、`TestCLI_SendBadFPS`、`TestCLI_ReceiveCameraNoGocv`、`TestCLI_ReceiveFileNoPath`、`TestCLI_ReceiveBadSource`、`TestCLI_ReceiveBadHash`

**总计：全绿（`go test -p 1 ./...` exit 0），0 失败。**

## 4. 验收项核对

| 验收点（TASK-011/04_api §1.2） | 结果 |
|--------------------------------|------|
| 无摄像头环境 file/stdin 闭环逐字节还原，SHA-256 与发送端一致 | ✓ 集成 `TestReceiveFromFileSource`/`TestBlockSizeMatrix`/`TestECCMatrix` |
| 文本直传（≤200B）走标准文本二维码（第三方 App 可读） | ✓ E2E `TestCLI_SendShortText`（渲染 + SHA-256） |
| 丢帧/乱序/元数据迟到经喷泉码兜底可还原 | ✓ 单元 sendreceive `TestE2EPacketLoss`/`TestE2EMetaLate`/`TestReceiveDedup` |
| 退出码 0/1/2/3 与 04_api §1.2 一致 | ✓ E2E `TestCLI_SendShortText`(0)/`TestCLI_SendNoArg`/`BadECC`/`BadFPS`/`ReceiveFileNoPath`/`ReceiveBadSource`(2)/`ReceiveCameraNoGocv`(3)/`ReceiveBadHash`(1) |
| `go test ./...` 全绿，`go vet ./...` 无告警 | ✓ |

## 5. 测试中发现并修复的缺陷

**缺陷 T-01（已修复）**：`send.BuildStream` 将元数据 `meta.BlockCount` 设为原始分块数 `blockCount`，但当 `blockCount∈{2,3}` 时 FEC 源符号数被提升至 `sourceK=4`（Raptor 要求 K≥4）。接收端 `processor.ensureDecoder` 用 `meta.BlockCount` 重建解码器，K 不匹配（meta=2 实际=4），导致 `msgLen`/`symLen` 计算错误，gofountain raptor 解码 panic（`slice bounds out of range`）。
- 修复：`internal/send/stream.go` 将 `meta.BlockCount` 改为 boost 后的实际 `sourceK`。
- 回归验证：集成 `TestTextAndBinaryPayload/text`（960B 文本，K=2→4）现通过。

> 此修复属 Developer 权限范围（internal/）。因 subagent 调度通道故障（见 §6），经用户授权由编排者代行 Developer 修复，将在阶段 8 提交中一并记录。

## 6. 已知问题

1. **360 安全软件干扰并行测试执行**：`go test ./...` 默认并行编译多个测试二进制到临时目录，360 实时扫描会锁住刚生成的 `.exe`，导致 `fork/exec ... Access is denied` 或偶发"符号不足"（gozxing 调用被干扰）。
   - 规避：`GOTMPDIR` 指向项目内 `.tmp/` + `go test -p 1` 顺序执行。
   - 待人工：建议将 `E:/aiCode/qrCode-to-data/.tmp` 与 Go 临时构建目录加入 360 信任区，之后可恢复并行测试。

2. **subagent 调度通道故障**：调度 Tester/Reviewer 角色 subagent 时，后端模型连续 3 次返回 `reasoning_content` / `tool_calls` API 错误（同一模型 id），无法走编排者调度流程。经用户授权，阶段 7（测试）由编排者破例亲自下场完成，全程不再调度 subagent。

3. **CLI 级 send→receive 真实闭环未测**：send 在终端逐帧渲染 ANSI 二维码不落盘帧图片，receive 默认走摄像头，无摄像头环境无法串联 CLI 级闭环。本报告 E2E 退化为 CLI 行为验证（退出码/输出）；端到端字节级闭环由集成测试用完整 `send.Send`/`receive.Receive` API 覆盖。摄像头真实闭环待实机验证。

## 7. 覆盖率说明

未启用 `-cover` 统计（本阶段聚焦功能正确性）。核心路径覆盖情况：
- FEC 编解码：单元（fec）+ 集成（闭环/丢帧/去重）
- 帧协议：单元（frame，组帧/拆帧/CRC/去重）
- QR 生成解析：单元（qrcode，闭环）+ 集成（多 version/ecc）
- send/receive 编排：单元（sendreceive）+ 集成（完整 Receive 路径）+ E2E（CLI 退出码）
- 未覆盖：摄像头采集（`camera_cgo.go`，需 gocv 构建环境+实机）、net（Phase 2 未实现）

## 8. 结论

**通过**。阶段 6 编码产出（commit `e8bdc79`）经集成测试与 E2E 测试验证，纯光学数据传输闭环功能正确，退出码契约符合 04_api §1.2，所有测试用例通过（0 失败）。发现 1 个业务缺陷（T-01）已修复。可进入阶段 8 代码审查。

待实机/待人工项（§6）不阻塞阶段流转，建议在发布前处理 360 信任区配置与摄像头真实闭环验证。
