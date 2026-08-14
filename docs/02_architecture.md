# 系统架构设计

> 状态：已确认
> 最后更新：2026-08-14
> 维护者：Architect

> 说明：本项目形态为 **CLI 工具 + 手机移动网页（Phase 2）**，非传统 Web 服务。本架构文档据此组织，不套用服务端/微服务模板。技术栈选型为「提议」状态（见 `docs/07_decisions.md` DEC-002），待人工确认后生效。

---

## 0. 架构原则

1. **单二进制、零运行时依赖优先**：桌面端以 Go 编译为单一可执行文件分发，除摄像头采集外不要求目标机预装运行时。
2. **核心算法可替换**：喷泉码、二维码编解码、摄像头采集各自成独立模块，通过接口隔离，便于替换库而不改动编排层。
3. **一条链路多形态复用**：二维码帧协议（头 + 载荷 + 校验）是纯光学、同网握手、手机中转三条链路的共同语言，收发两端能力对称。
4. **降级优先于失败**：同网直传不可用时回退纯光学；摄像头不可用时支持从图片/视频文件或 stdin 解码。
5. **不依赖网络即可工作**：纯光学路径为零网络依赖，这是产品核心承诺；网络仅作为 Phase 2 加速路径。
6. **数据不出用户控制范围**：不引入任何第三方服务器/云端，手机中转数据仅存本机浏览器存储。

---

## 1. 技术栈选型（提议，待人工确认）

> 每条选型给出明确推荐 + 理由 + 备选。最终确认前，实现一律以本推荐为准。

| 类别 | 推荐 | 理由 | 备选 |
|------|------|------|------|
| 语言 | **Go 1.22+** | 单二进制、交叉编译 Linux/macOS/Windows；二维码与喷泉码库成熟；参考项目 txqr 同栈，社区可借鉴 | Python（原型快，但分发需解释器、二维码动画渲染性能与打包成本差）；Rust（性能好但二维码/FEC 生态不如 Go 成熟，开发成本高） |
| 喷泉码（FEC） | **gofountain（github.com/google/gofountain），默认 Raptor 码（RFC 5053），LT 码兜底** | 纯 Go 无 CGO；txqr 同源生态已验证；Raptor 冗余开销（约 5~10%）远低于纯 LT（20~30%），吞吐收益明显 | RaptorQ（RFC 6330，near-zero 开销约 1~2%，但 Go 生态无完整成熟实现，gofountain 的 RaptorQ 为实验性，极致吞吐才值得）；纯 LT（实现最简但开销大） |
| 二维码生成 | **skip2/go-qrcode** | 纯 Go、成熟稳定、支持 version 1~40 与 EC L/M/Q/H；配合喷泉码可将 QR 自身纠错降到 L，最大化单帧载荷 | yeqown/go-qrcode（支持渐变/边框/更多渲染，但依赖更多）；boombuler/barcode（通用条码库，二维码能力弱） |
| 二维码解析 | **makiuchi-d/gozxing** | ZXing 的 Go 移植，解码鲁棒性与 API 成熟，可处理摄像头帧与图片 | 直接调用 ZXing C++/Java（重、需外部运行时）；tuotoo/qrcode（维护停滞） |
| 摄像头采集 | **gocv（OpenCV Go 绑定）跨平台采集 + gozxing 解码** | Go 生态唯一成熟的跨平台摄像头方案，能稳定取帧喂给解码器 | go4vl（Linux V4L2，纯 Go 无 CGO，可作为 Linux 单二进制方案）；FFmpeg/GStreamer 子进程（需外部二进制）。**另提供「从图片/视频文件或 stdin 解码」兜底，无摄像头环境可用** |
| 终端渲染 | **ANSI 半块字符（▀▄█）渲染** | 纯文本终端即可显示二维码（服务器/嵌入式无 GUI 可用），txqr 已验证；零额外依赖 | termbox-go（终端 TUI 更完整、可控颜色，但 Windows 平台兼容有代价）；独立窗口/SDL（Phase 2/P2 再引入 GUI 渲染） |
| CLI 框架 | **cobra** | 子命令/usage 帮助规范，满足 PRD F-01「清晰中文提示与 usage」；扩展 Phase 2 子命令方便 | 标准库 flag + 手写子命令分发（零依赖，但帮助/校验需手写，扩展性差） |
| 同网直传（Phase 2） | **HTTP 临时服务 + 一次性 token（参考 qrcp）** | 实现简单、跨平台、二维码只需承载一段短 URL；Go 标准库即可 | WebRTC（P2P 穿透强但复杂度高、非零配置）；原始 TCP socket（需自定义协议） |
| 手机端（Phase 2） | **移动网页 PWA（原生 JS/TS，无框架或轻量框架）** | 浏览器打开即用、免安装；复用浏览器摄像头（getUserMedia）+ IndexedDB 暂存；Service Worker 支持离线 | 原生 App（Flutter/React Native，体验好但分发/开发成本高，与「免装 App」定位冲突） |

### 关键权衡提示（已写入 DEC-002 影响）

- **摄像头采集是唯一破坏「纯单二进制」的点**：gocv 依赖 OpenCV 运行时（CGO）。缓解策略：`receive` 命令提供 `--source file|stdin` 兜底；Linux 可选 go4vl 保持纯 Go。MVP 需明确「摄像头路径允许带 OpenCV 依赖」，或在无 OpenCV 环境降级为屏幕对屏幕 + 外部采集。
- **吞吐与可扫描性的平衡**：二维码 version 越大单帧载荷越多，但对摄像头分辨率/距离要求越高。默认 version 20（EC L 约 858 字节/帧），可调上限到 40。

---

## 2. 系统架构图

```
┌─────────────────────────────── 发送端（桌面 CLI） ───────────────────────────────┐
│                                                                                  │
│  cmd/qrcd (cobra 入口)                                                           │
│    │ send                                                                         │
│    ▼                                                                              │
│  internal/payload ── 读文件/文本 → 分块 → 计算 SHA-256/CRC32                      │
│    │                                                                              │
│    ▼                                                                              │
│  internal/fec ── 喷泉码编码（gofountain Raptor/LT）→ 编码符号流                    │
│    │                                                                              │
│    ▼                                                                              │
│  internal/frame ── 组帧（32B 头 + 载荷 + CRC32）                                   │
│    │                                                                              │
│    ▼                                                                              │
│  internal/qrcode ── go-qrcode 生成 QR 位图 → ANSI 半块渲染 → 屏幕逐帧播放          │
│    │                                                                              │
│    ▼                                                                              │
│  internal/progress ── 进度/速率统计（终端输出）                                    │
│                                                                                  │
│  [Phase 2] internal/net ── 同网直传：临时 HTTP 服务 + 一次性 token，QR 只做握手    │
└──────────────────────────────────────────────────────────────────────────────────┘
        │ 光学通道（屏幕 → 摄像头）        │ 网络通道（Phase 2 局域网）
        ▼                                 ▼
┌─────────────────────────────── 接收端（桌面 CLI） ───────────────────────────────┐
│  cmd/qrcd (cobra 入口)                                                            │
│    │ receive                                                                      │
│    ▼                                                                              │
│  internal/capture ── gocv 采集摄像头帧 / 图片文件 / stdin                          │
│    │                                                                              │
│    ▼                                                                              │
│  internal/qrcode ── gozxing 解码二维码 → 字节                                      │
│    │                                                                              │
│    ▼                                                                              │
│  internal/frame ── 拆帧校验（CRC32）→ 元数据帧 / 数据帧 → 去重                     │
│    │                                                                              │
│    ▼                                                                              │
│  internal/fec ── 喷泉码解码（收够足量符号 → 还原）                                 │
│    │                                                                              │
│    ▼                                                                              │
│  internal/payload ── SHA-256 完整性校验 → 写 .part 临时文件 → 校验通过后 rename     │
│    │                                                                              │
│    ▼                                                                              │
│  internal/progress ── 进度/校验结果输出                                            │
└──────────────────────────────────────────────────────────────────────────────────┘

┌──────────────────── 手机中转（Phase 2，移动网页 PWA） ────────────────────┐
│  浏览器页面                                                               │
│    ├─ 收：getUserMedia 摄像头 → jsQR/gozxing(wasm) 解码 → IndexedDB 暂存   │
│    ├─ 存：IndexedDB（qrcd / transfers）                                   │
│    └─ 发：IndexedDB 读取 → JS 编码 QR 流 → 屏幕播放（Canvas）              │
│  复用与桌面端完全相同的二维码帧协议，能力对称                               │
└──────────────────────────────────────────────────────────────────────────┘
```

### 模块依赖方向（单向，无循环）

```
cmd/qrcd
  → internal/send     → internal/payload → internal/fec → internal/frame → internal/qrcode
  → internal/receive  → internal/capture → internal/qrcode → internal/frame → internal/fec → internal/payload
  → internal/progress
  → internal/net (Phase 2)
```

- `internal/payload`、`internal/fec`、`internal/frame`、`internal/qrcode` 为纯算法/纯 Go，无平台依赖，可被桌面端与（未来）手机端复用同一套帧协议定义。
- `internal/capture` 是唯一含 CGO/平台依赖的模块，接口隔离。
- 手机网页（`web/`）为独立前端，仅共享帧协议格式与存储 schema，不与 Go 代码交叉编译。

---

## 3. 模块划分与职责

| 模块 | 职责 | 技术实现 |
|------|------|---------|
| `cmd/qrcd` | CLI 入口：解析 send/receive 子命令与参数，错误提示与 usage | cobra |
| `internal/payload` | 载荷读取/写入：文本与文件两类；分块；SHA-256 与 CRC32 校验；`.part` 临时文件 + rename 原子落盘 | 标准库 + crypto/sha256, hash/crc32 |
| `internal/fec` | 喷泉码编解码接口封装：把字节流编码为符号流 / 从足量符号还原 | gofountain（Raptor 主 / LT 兜底） |
| `internal/frame` | 二维码帧协议：组帧（头+载荷+CRC32）、拆帧、去重、元数据帧解析 | 自研，纯 Go |
| `internal/qrcode` | 二维码生成（go-qrcode）与解析（gozxing）；终端 ANSI 半块渲染 | skip2/go-qrcode, makiuchi-d/gozxing |
| `internal/capture` | 摄像头帧采集（跨平台）；文件/图片/stdin 兜底 | gocv（+ go4vl 备选） |
| `internal/send` | 发送编排：payload → fec → frame → qrcode 逐帧播放；进度输出 | 组合上述模块 |
| `internal/receive` | 接收编排：capture → qrcode → frame → fec → payload 还原；进度/校验 | 组合上述模块 |
| `internal/progress` | 进度、速率、校验结果统计与终端展示 | 标准库 |
| `internal/net`（Phase 2） | 同网检测 + 临时 HTTP 服务 + 一次性 token | 标准库 net/http |
| `web/`（Phase 2） | 手机移动网页 PWA：收/暂存/发 | 原生 JS/TS + IndexedDB + getUserMedia + Canvas |

---

## 4. 数据流

### 4.1 发送端编码链路（纯光学）

```
文件/文本
  → payload 分块（blockSize，默认 1024B）
  → 计算整体 SHA-256（写入元数据帧）
  → fec 编码（每源块 → K 个源符号；Raptor 生成带冗余编码符号，编号 id 递增）
  → frame 组帧：
      ① 元数据帧（type=0x01：文件名/大小/源块数/哈希/FEC 参数）
      ② 数据帧（type=0x02：符号 id + 符号字节）
      每 N 帧重播一次元数据帧（保证接收端中途加入也能解析）
  → qrcode 生成 QR 位图（version ≤ 上限，EC L）
  → ANSI 半块渲染到终端，按 fps 逐帧播放（发送端循环取符号，可无限补发冗余）
```

### 4.2 接收端解码链路（纯光学）

```
摄像头帧（或图片/文件/stdin）
  → qrcode 解码出字节
  → frame 拆帧：校验 CRC32，失败丢弃；解析 type
     元数据帧 → 初始化会话（预期总大小/块数/哈希）
     数据帧 → 按符号 id 去重（已收过忽略）
  → fec 解码：AddSymbol(id, data)；收够足量 → Decode 还原
  → payload：SHA-256 校验还原结果
     通过 → 写 .part 临时文件 → rename 为最终文件名，提示「校验通过」
     失败 → 丢弃/标记损坏，提示重传
  → progress 全程显示已收块数/百分比/速率
```

### 4.3 同网加速链路（Phase 2）

```
发送端 send --net auto
  → 检测局域网可达（或用户指定 --addr）
  → 起临时 HTTP 服务（随机端口）+ 一次性 token
  → 二维码只承载握手信息（qrcd://transfer?t=<token>&addr=<ip:port>）
接收端 receive
  → 扫握手码 → GET /v1/transfer/{token} 拉取数据流
  → SHA-256 校验
  → 不可达/token 失效 → 自动回退纯光学
```

### 4.4 手机中转链路（Phase 2）

```
A 机 send →（光学）→ 手机网页「收」：摄像头解码 → IndexedDB 暂存
手机网页「发」：IndexedDB 读取 → 编码 QR 流 → 屏幕播放 →（光学）→ C 机 receive
整条链路复用同一帧协议，手机只做「收 → 暂存 → 重放」，不改变数据内容。
```

---

## 5. 部署方案

### 5.1 单二进制分发（桌面端）

- Go 交叉编译产出单一可执行文件：`qrcd`（Linux/macOS）/ `qrcd.exe`（Windows）。
  - `GOOS=linux GOARCH=amd64 go build ./cmd/qrcd`
  - `GOOS=darwin GOARCH=arm64 go build ./cmd/qrcd`
  - `GOOS=windows GOARCH=amd64 go build ./cmd/qrcd`
- 目标平台：Linux / macOS / Windows 终端（对应 PRD 兼容性要求）。
- 摄像头依赖说明（诚实标注）：
  - 纯光学 `send`（屏幕发）为纯 Go 单二进制，无外部依赖。
  - `receive` 的摄像头路径依赖 OpenCV（gocv）；Linux 可用 go4vl 实现纯 Go；否则提供 `--source file|stdin` 兜底。
- 发布物：每平台一个可执行文件 + 可选 SHA256 校验清单；Phase 2 附加 `web/` 静态资源（手机网页，托管到任意静态源或本地）。

### 5.2 环境划分

| 环境 | 用途 | 配置 |
|------|------|------|
| 开发 | 本地编译调试 | `go run ./cmd/qrcd`，调试日志开启 |
| 测试 | CI/单元测试 | `go test ./...`；帧协议/FEC 用确定性用例 |
| 生产 | 分发的单二进制 | 默认日志只输出进度与错误，静默调试信息 |

### 5.3 健康检查 / 监控告警

- **不适用**（CLI 无常驻服务）。对应能力由 CLI 的「进度输出 + 错误码 + 超时检测」承担：接收端超过 `--timeout` 无新块则提示卡死并允许手动终止。

---

## 6. 安全设计

### 6.1 认证与授权

- 纯光学路径：无网络、无账号体系，天然物理隔离；不引入认证。
- 同网直传（Phase 2）：一次性随机 token（高熵，如 128bit），握手码被第三方误扫后 token 单次使用即失效；token 过期时间短（如 5 分钟）。

### 6.2 数据安全

- 数据不经过任何第三方服务器/云端。
- 工具以本机用户权限运行，仅读写用户显式指定的文件路径（send 的输入、receive 的 `--output`）。
- 同网直传会话传输（Phase 2）：HTTP 仅局域网可达，绑定监听地址为本机局域网 IP；可选会话加密（P2，TLS 自签 + 指纹校验，待后续决策）。
- 手机中转（Phase 2）：暂存数据仅存本机浏览器 IndexedDB，支持手动清除，不自动上传；可选 XChaCha20 加密暂存（P2）。

### 6.3 数据完整性

- 单帧：CRC32 校验（防单帧误读/损坏）。
- 整体：SHA-256 校验还原结果（F-06），校验失败不静默写出损坏数据。
- 二维码自身 EC 降为 L 的前提是外层有喷泉码 + CRC + SHA-256 三层兜底。

### 6.4 SDL / 安全审计

- 编码阶段：所有外部库锁定版本并评审（gofountain / go-qrcode / gozxing / gocv）；禁用 `os/exec` 调用外部解码器（避免注入）。
- 发布前：`go vet` + 静态检查；临时文件清理（`.part`）；token 用 `crypto/rand` 生成，禁止可预测随机源。
- 本 CLI 不处理用户凭据，安全审计重点是「文件路径越界读写」与「临时文件残留」。

---

## 7. 性能考量

### 7.1 性能目标（MVP 建议值，实测后细化）

| 指标 | 目标值 |
|------|--------|
| 短文本直传 | 单帧/少量帧，秒级完成（≤ ~200 字节直接单张文本二维码） |
| 纯光学吞吐 | 默认 version 20（858B/帧，EC L）@ 10~15 fps ≈ 7~12 KB/s（Raptor 冗余后） |
| 文件传输 | 若干 MB 级：1MB ≈ 1.5~3 分钟（随 version/fps 可调） |
| 丢帧容错 | 随机丢块 10~30% + 乱序仍可无损还原 |
| 并发 | 单会话单用户，无并发要求 |

### 7.2 优化策略

- QR 纠错降为 L + 喷泉码兜底，最大化单帧载荷（比默认 M 多约 20~30% 载荷）。
- 喷泉码冗余度可调（`--redundancy`）：冗余越小吞吐越高、越依赖扫码稳定；默认约 10%。
- 二维码 version 自适应/可调：摄像头分辨率允许时提升 version（25 约 1273B/帧，40 约 2953B/帧）。
- 接收端解码用多线程：摄像头采集、二维码解码、FEC 解码流水线并行，避免互相阻塞。
- 去重 + 已收符号计数，避免重复帧浪费解码时间。

---

## 8. 扩展性设计

- **协议可演进**：帧头含 `version` 字段，未来帧格式变更可向前兼容（旧版本工具识别后提示升级）。
- **核心可替换**：FEC、二维码编解码、摄像头采集均以接口隔离，可替换库不重构编排层。
- **能力对称复用**：手机网页复用同一帧协议，新增「中转」几乎不增加核心复杂度，仅新增前端 + 本地暂存。
- **水平扩展**：不适用（单机 CLI）。同网直传可扩展为多接收者同时下载同一 token（Phase 2 可选）。
- **渲染器可插拔**：终端 ANSI 渲染之外，未来可插入窗口渲染 / 图片导出，不影响帧协议。

---

## 9. 第三方服务依赖

> 无任何远程第三方服务/云端。以下为代码库级第三方依赖（Go module / 前端资源）。

| 依赖 | 用途 | 替代方案 |
|------|------|---------|
| github.com/spf13/cobra | CLI 子命令与 usage | 标准库 flag |
| github.com/google/gofountain | 喷泉码（Raptor/LT） | 自研 LT / RaptorQ 实现 |
| github.com/skip2/go-qrcode | 二维码生成 | yeqown/go-qrcode |
| github.com/makiuchi-d/gozxing | 二维码解析 | ZXing C++ 绑定 |
| gocv.io/x/gocv | 摄像头采集（OpenCV） | go4vl（Linux 纯 Go） |
| （前端 Phase 2）jsQR 或 gozxing/wasm | 手机网页扫码 | 浏览器 BarcodeDetector API |
