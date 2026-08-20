# Changelog

本项目所有重要变更均会记录在此文件中。
格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Added

- DEC-007 手机端原生 Android App：CameraX + ML Kit Barcode（rawBytes 直出二进制帧）+ OkHttp 转发，三按钮手动触发（扫描/发送/清空），即时反馈「已捕获 N 块」。取代 jsQR 网页方案（DEC-002 手机端部分废弃）。Web 服务增 `/dl/app.apk`（APK 下载）、`/`（首页含 APK 下载二维码）、`/api/qrcode`（生成下载链接二维码）、`/scantest`（jsQR 自检）。
- DEC-008 `send.Options.MaxSymbol`：单符号字节上限（0=自适应），Web 端固定 151（数据帧 v12 Q，65×65，手机可稳定扫描）；元数据帧用 Version:40 auto-select min 解决长文件名超容。
- DEC-009 `frame.MetaData.TotalSymbols`：发送端填充含冗余的编码符号总数；接收端进度基准改用之，还原完成后继续累计已收使三端计数对齐；手机端只对数据帧（type=0x02）计数；发送端显示数据符号数；`/api/encode` 响应增 `symbols`。新增单测 `TestPostDecodeCounting`。
- DEC-013 App 录像模式：CameraX VideoCapture 录制 + MediaMetadataRetriever 抽帧离线解析，将扫描与解码解耦。MainActivity.kt 新增录像模式（recordBtn 绑定 ↔ 切换录像/停止；VideoCapture<Recorder> + Quality.SD + 不录音；多段录像累积序号 recordSeq）。停止后 parseExecutor 后台线程 MediaMetadataRetriever 按 ~66ms 步进抽帧 → ML Kit 解码 → 共享 ingestBarcodeBytes 去重+缓存+计数。实时扫描与录像解析共用同一去重/缓存路径。onDestroy 补 parseExecutor.shutdown()。sendBuffer 上传期间禁用 recordBtn。

### Fixed

- T-03（DEC-010）：手机端 `dedupKey` 改整帧哈希。早期仅哈希前 16 字节+长度，而所有数据帧前 16 字节相同且等长 → 除首帧外全被去重丢弃，手机只捕获 2 块。修复后文件传输跑通，SHA-256 校验通过。
- T-04（DEC-009）：三端计数对不上（发送端 95/手机 91/接收端 72）。根因为喷泉码凑齐 K 即 done、之后符号被丢弃。改为 TotalSymbols 基准 + done 后继续累计，三端数字可比。
- T-05（DEC-008）：发送端二维码不动（play 链路改 img.onload 链式调度，等当前帧加载绘制完再调度下一帧）。
- T-06（DEC-008）：帧 5/帧 0 加载失败 500（元数据帧超 v15 Q 容量，改 Version:40）。
- T-02（DEC-006）：`qrcode.DecodeImageBytes` 启用 gozxing `PURE_BARCODE` 提示，修复纯二维码大尺寸（≥~315px）下 HybridBinarizer 误估模块数导致约 8% 随机丢帧（报「符号不足」）。此前跨运行随机失败，修复后集成/E2E 多轮复跑全绿。
- T-01：`send.BuildStream` 元数据 `BlockCount` 改存 boost 后的实际 sourceK（原存原始 blockCount，当 blockCount∈{2,3} 被提升至 K=4 时不匹配，致接收端 `ensureDecoder` 用错误 K 重建解码器，gofountain raptor 解码 panic）

### Added

- 初始化项目模板结构
  - 创建 .ai/ 角色定义、规则、工作流程
  - 创建 docs/ 文档模板体系
  - 创建 src/、tests/ 目录骨架
- TASK-001 项目骨架：`go mod init qrcd`，cmd/qrcd（cobra 中文 usage），10 个 internal 包
- TASK-002 帧协议 `internal/frame`：32B 头 + payload + CRC32，元数据/数据帧，seq 去重
- TASK-003 喷泉码 `internal/fec`：Raptor/LT/none 适配 gofountain，单元测试覆盖闭环/丢包/空载荷
- TASK-004 二维码 `internal/qrcode`：go-qrcode 生成（version 上限 1~40 + ECC L/M/Q/H 校验）+ gozxing 解析闭环，ANSI 半块/ASCII 渲染（≥4 模块静区、无彩色），短文本（≤200B）直传
- TASK-005 载荷 `internal/payload`：文件/文本/stdin 读取、分块（默认 1024B）、SHA-256/CRC32、`.part`→rename 原子落盘、`--overwrite` 控制、文本摘要名
- TASK-006 采集 `internal/capture`：帧源抽象接口（camera/file/stdin 可插拔），camera 经 `-tags qrcd_camera` build tag 隔离 gocv（默认构建不依赖 OpenCV）
- TASK-007 进度 `internal/progress`：统一进度事件接口（百分比/速率/ETA/去重统计）、ANSI 原地刷新、超时监控、`--quiet`
- TASK-008 发送编排 `internal/send`：payload 分块→SHA-256→FEC 编码→frame 组帧→QR 生成 + ANSI/ASCII 逐帧播放；短文本 ≤200B 走 F-05 直传（静态标准文本二维码）；元数据帧每 20 个数据帧重播一次；自适应分块适配单帧 QR 容量；FPS/版本/ECC/冗余度可控，Ctrl+C 干净停止
- TASK-009 接收编排 `internal/receive`：帧源取流→qrcode 解码→frame 拆帧（CRC 校验/seq 去重/元数据迟到缓冲初始化会话）→FEC 解码→SHA-256 校验→`.part` 临时文件→rename 原子落盘；`--expect-size`/`--hash` 预校验，失败丢弃损坏数据并提示重传
- TASK-010 CLI 入口 `cmd/qrcd`：cobra `send`/`receive` 子命令，参数名/默认值与 04_api §1 对齐，flag 统一 Var* 变体绑定 opts；退出码四档（0 成功/1 传输失败/2 参数错误/3 环境错误），错误到 stderr、进度与结果到 stdout，帮助与错误提示中文
- TASK-011 端到端联调：send→receive 纯光学闭环（file/stdin 兜底，无摄像头环境）逐字节还原一致，SHA-256 与发送端一致；丢帧/乱序/元数据迟到场景经喷泉码兜底可还原；单元测试落位 tests/unit/sendreceive/ 共 11 例
- 阶段7测试：tests/integration/（8 例，完整 Send/Receive API 闭环 + BlockSize/ECC 矩阵 + Overwrite 控制 + 文本/二进制载荷）、tests/e2e/（10 例，CLI 黑盒退出码 0/1/2/3 + QR 渲染 + help）、docs/09_test_report.md（被测版本 stage-6-handoff/e8bdc79）

### Changed

- DEC-004：`fec.Codec.NewDecoder` 契约增加 `messageLength` 参数（gofountain 必须）
- FEC 解码器复用单一 gofountain decoder 增量投喂（避免 AddBlocks 原地 XOR 污染重投数据）
- 依赖新增 `skip2/go-qrcode`、`makiuchi-d/gozxing`（`gocv` 不入 go.mod，camera 由 build tag 隔离，启用 `-tags qrcd_camera` 时需另行安装 gocv/OpenCV）

### Deprecated

- DEC-002 手机端「移动网页 PWA」方案被 DEC-007 取代（jsQR 对二进制 byte-mode QR 解码不可靠）。桌面端 Go CLI / FEC / QR 选型不变。

### Removed

### Security

---

## 版本历史

（暂无正式发布版本）
