# 接口设计（CLI 命令 + 传输协议）

> 状态：已确认
> 最后更新：2026-08-20
> 维护者：Architect

> 说明：本项目无 REST API、无服务端。本文档定义三类契约：
> 1. **CLI 命令接口**（send / receive 子命令）；
> 2. **二维码帧协议**（纯光学传输的数据格式，头 + 载荷 + 校验）；
> 3. **FEC 编解码接口契约**（Go 接口签名）；
> 4. **同网直传协议** 与 **手机网页 ↔ 桌面端交互接口**（Phase 2 初稿）。

---

## 1. CLI 命令接口契约

### 1.1 总体规范

- 程序名：`qrcd`（可执行文件，Windows 下 `qrcd.exe`）
- 命令格式：`qrcd <子命令> [参数] [flags]`
- 子命令：`send`、`receive`
- 语言：帮助与错误提示统一中文
- 数据格式：二进制（文件/字节流）；短文本直传允许标准文本二维码（兼容第三方 App）

### 1.2 退出码

| 退出码 | 含义 |
|--------|------|
| 0 | 成功（传输完成且校验通过） |
| 1 | 传输/解码失败（校验失败、符号不足、超时等） |
| 2 | 参数错误（非法参数、缺参） |
| 3 | 环境错误（摄像头不可用、文件不可读/写、端口冲突等） |

- 错误信息输出到 stderr；进度与结果输出到 stdout。

### 1.3 `send` 子命令

```
qrcd send <file> [flags]
qrcd send --text <文本> [flags]
```

- 位置参数 `<file>`：要发送的文件路径。与 `--text` 二选一（未指定则报错并输出 usage）。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `<file>` | string | 条件 | - | 待发送文件路径（与 `--text` 二选一） |
| `--text, -t` | string | 条件 | - | 直接发送文本（与 `<file>` 二选一） |
| `--fps` | int | 否 | 10 | 二维码播放帧率（帧/秒） |
| `--version, -v` | int | 否 | 20 | QR 版本上限（1~40），载荷不足自动降级 |
| `--ecc` | string | 否 | L | QR 纠错级别 L/M/Q/H（配合喷泉码默认 L） |
| `--redundancy, -r` | float | 否 | 0.1 | 喷泉码冗余度（0~1，0.1=多 10% 编码符号） |
| `--block-size, -b` | int | 否 | 1024 | 源分块大小（字节） |
| `--terminal` | string | 否 | ansi | 渲染器：`ansi`（终端半块）/ `window`（Phase 2） |
| `--invert` | bool | 否 | false | 反色（浅色终端背景） |
| `--quiet, -q` | bool | 否 | false | 关闭进度，只输出最终结果 |
| `--net` | string | 否 | auto | 同网直传（Phase 2）：`auto`/`on`/`off` |
| `--addr` | string | 否 | - | 指定监听 IP（同网直传，Phase 2） |

**返回**：
- 成功：退出码 0，打印整体 SHA-256。
- 载荷过大：打印预估时长 + 建议走同网加速，仍继续纯光学。

### 1.4 `receive` 子命令

```
qrcd receive [flags]
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `--output, -o` | string | 否 | 当前目录 | 输出目录或完整文件名 |
| `--camera` | int | 否 | 0 | 摄像头设备号 |
| `--source` | string | 否 | camera | 输入源：`camera`/`file`/`stdin`（无摄像头兜底） |
| `--fps` | int | 否 | 30 | 采集/解码帧率上限 |
| `--version, -v` | int | 否 | 20 | 期望 QR 版本上限 |
| `--expect-size` | int | 否 | - | 预期总字节数（可选，提前校验） |
| `--hash` | string | 否 | - | 预期校验值，格式 `sha256:<hex>` |
| `--timeout` | duration | 否 | 30s | 无新符号超时（提示卡死并允许终止） |
| `--overwrite` | bool | 否 | false | 允许覆盖已存在文件 |
| `--quiet, -q` | bool | 否 | false | 关闭进度，只输出最终结果 |

**返回**：
- 成功：退出码 0，输出文件路径 + 大小 + SHA-256 +「校验通过」。
- 失败：退出码 1/3，输出中文原因与重试引导（如「符号不足，请调整角度重扫」）。

---

## 2. 二维码帧协议（纯光学传输）

> 参考 txqr（LT 码 + 简单块封装）与 RaptorQR（帧头携带元数据 + 系统码）的做法，本协议采用「固定 32 字节头 + 载荷 + 4 字节 CRC32」结构。

### 2.1 帧总体格式

```
+----------------+--------+--------+--------+----------------+--------------+----------+------------+------------+
| magic (4B)     | ver(1) | type(1)| flags  | transfer_id    | seq (4B)     | len (4B) | payload    | crc32 (4B) |
| "QRCD"         | 0x01   |        | (2B)   | (16B)          | (u32, 大端)  | (u32,大端)| (len 字节) | 大端        |
+----------------+--------+--------+--------+----------------+--------------+----------+------------+------------+
头 = 前 32 字节；CRC32 = 对「头(0:32) + payload」整体计算的 IEEE CRC32
```

### 2.2 字段定义

| 偏移 | 长度 | 字段 | 说明 |
|------|------|------|------|
| 0 | 4 | magic | 固定 `0x51 0x52 0x43 0x44` = ASCII `"QRCD"` |
| 4 | 1 | version | 协议版本，当前 `0x01` |
| 5 | 1 | type | 帧类型：`0x01` 元数据帧，`0x02` 数据帧 |
| 6 | 2 | flags | bit0 载荷类型（0 文本 / 1 文件）；bit1 是否 FEC（0 无 / 1 喷泉）；其余保留为 0 |
| 8 | 16 | transfer_id | 本次传输会话 ID（随机 128bit），用于区分并发/残留帧 |
| 24 | 4 | seq | 帧序号：数据帧 = 喷泉编码符号 id；元数据帧 = 固定 0 |
| 28 | 4 | len | payload 字节长度（u32 大端） |
| 32 | len | payload | 载荷（见 2.3 / 2.4） |
| 32+len | 4 | crc32 | 对「头 + payload」的 CRC32（大端），校验失败丢帧 |

- 整数一律**大端**；文本载荷一律 **UTF-8**。

### 2.3 元数据帧（type=0x01）

- payload 为 JSON（UTF-8，单行，无 BOM）。
- 单会话（不分片）的 JSON 示例：

```json
{
  "name": "backup.tar.gz",
  "size": 1048576,
  "blockSize": 1024,
  "blockCount": 1024,
  "hashAlgo": "sha256",
  "hash": "<hex>",
  "fec": "raptor",
  "redundancy": 0.1,
  "payloadType": "file",
  "mimeType": "application/octet-stream",
  "totalSymbols": 1127
}
```

- 多会话分片（大文件拆为 N 个独立传输会话，见 DEC-012）时，第 0 分片（partIndex=0）额外携带整体信息：

```json
{
  "name": "part_0.tar.gz.part",
  "size": 131072,
  "blockSize": 1024,
  "blockCount": 128,
  "hashAlgo": "sha256",
  "hash": "<part0_sha256>",
  "fec": "raptor",
  "redundancy": 0.1,
  "payloadType": "file",
  "mimeType": "application/octet-stream",
  "totalSymbols": 141,
  "partIndex": 0,
  "partTotal": 4,
  "overallName": "backup.tar.gz",
  "overallSize": 8388608,
  "overallHash": "<overall_sha256>"
}
```

| 字段 | 必需 | 说明 |
|------|------|------|
| `name` | 是 | 原始文件名（分片时 = 整体文件名 + `.part` 后缀，如 `backup.tar.gz.part`）；文本载荷为 `<文本摘要>.txt` 或用户指定 |
| `size` | 是 | 本分片载荷字节数（分片时 = 该分片实际字节数，非整体大小） |
| `blockSize` / `blockCount` | 是 | 源分块大小与源块数 |
| `hashAlgo` / `hash` | 是 | 本分片完整性校验算法与值（分片时 = 该分片 SHA-256，非整体） |
| `fec` | 是 | FEC 方案：`none` / `lt` / `raptor` |
| `redundancy` | 是 | 冗余度 |
| `payloadType` / `mimeType` | 是 | `text`/`file` 与内容类型 |
| `totalSymbols` | 否 | 发送端计划发送的编码符号总数（含冗余），用于接收端进度基准。缺省则回退到 `blockCount`。详见 DEC-009 |
| `partIndex` | 否（分片时必需） | 分片序号，0-based（0..partTotal-1）。单会话时缺省 |
| `partTotal` | 否（分片时必需） | 总分片数。单会话时缺省 |
| `overallName` | 否（仅 part 0） | 整体原始文件名（仅 partIndex=0 携带，拼接后落盘的文件名） |
| `overallSize` | 否（仅 part 0） | 整体总字节数（仅 partIndex=0 携带） |
| `overallHash` | 否（仅 part 0） | 整体 SHA-256（仅 partIndex=0 携带，接收端拼接后按此校验） |

**分片语义**（DEC-012）：
- 大文件（超过单会话 Raptor 容量上限 K≤8192）自动拆为 N 个分片会话，各分片独立 `transfer_id`、独立 QR 流。
- 每个分片有自己的 `name`/`size`/`hash`（分片级），可独立校验、断点重发分片。
- `partIndex=0` 的分片额外携带 `overallName`/`overallSize`/`overallHash`，接收端收齐 N 个分片后按序（partIndex 0..N-1）拼接字节，对整体计算 SHA-256 与 `overallHash` 比对，通过后以 `overallName` 落盘。
- 分片大小自动计算：`partSize = 8192 × symbolSize`（symbolSize 受 MaxSymbol 与 QR 版本容量约束），`partTotal = ceil(原始数据总长 / partSize)`。发送端保证每会话源块数 ≤8192。

- 元数据帧在发送过程中**周期性重播**（默认每 20 个数据帧重播一次），保证接收端中途加入或漏收元数据也能恢复。

### 2.4 数据帧（type=0x02）

- payload = 一个喷泉编码符号的原始字节（Raptor/LT 输出）。`seq` = 符号 id。
- 接收端按 `seq` 去重（已收过忽略），交 FEC 解码。
- 单块载荷（size ≤ 单帧载荷上限，如短文本超长或极小文件）：`fec="none"`，`seq` = 分块序号，`blockCount=1`。

### 2.5 短文本直传（F-05 特殊路径）

- 短文本（≤ 约 200 字节）**不组帧**，直接编码为**标准文本二维码**（内容即原文），第三方扫码 App 可读。
- 超过单帧容量：自动切换为「元数据帧 + 数据帧」的 QR 流（`payloadType="text"`），并提示已切换。

### 2.6 容量与版本

- 帧总长须 ≤ 所选 QR version 的 8-bit 字节容量（EC L）。默认 version 20 ≈ 858B，扣 32B 头 + 4B CRC 后载荷 ≈ 822B。
- 单帧超出容量时：降低 version 或减小 blockSize（实现层自动处理，遵循「载荷不足自动降级」）。

---

## 3. FEC 编解码接口契约（Go）

> 包路径 `internal/fec`，接口面向编排层（send/receive），实现层适配 gofountain。以下为**契约签名**，非实现。

```go
// Codec 创建编码器/解码器
type Codec interface {
    // Scheme 返回方案名：none / lt / raptor
    Scheme() string

    // NewEncoder 将源块 data 切为 blockSize 大小的源符号，返回编码器
    NewEncoder(data []byte, blockSize int, redundancy float64) (Encoder, error)

    // NewDecoder 创建解码器，sourceSymbols 为源符号数，messageLength 为原始数据长度
    NewDecoder(sourceSymbols int, messageLength int) Decoder
}

type Encoder interface {
    // SourceSymbols 返回源符号数量
    SourceSymbols() int
    // NextSymbol 返回下一个编码符号（含符号 id），可无限产出冗余符号
    NextSymbol() (id uint32, symbol []byte)
}

type Decoder interface {
    // AddSymbol 送入一个编码符号（可乱序、可重复）；done=true 表示已收够可还原
    AddSymbol(id uint32, symbol []byte) (done bool, err error)
    // Decode 还原源数据（须在 done=true 后调用）
    Decode() ([]byte, error)
    // Received 返回已收到的不重复符号数
    Received() int
}
```

- 空载荷：编码器/解码器对 size=0 直接返回空字节，不产生符号（正确处理，不 panic）。
- 单块载荷（blockCount=1）：`fec="none"` 短路，不经过 FEC，直接单数据帧。
- 语义保证：任意足量（≥ 源符号数 + 少量冗余）互不相同的符号即可无损还原，顺序无关。

---

## 4. 同网直传协议（Phase 2 初稿）

> 目标：两端同网时二维码只做握手，数据走局域网 HTTP。不可达自动回退纯光学。

### 4.1 握手码（二维码内容）

```
qrcd://transfer?t=<token>&addr=<ip>:<port>&sha256=<hex>
```

| 字段 | 说明 |
|------|------|
| `t` | 一次性随机 token（128bit，`crypto/rand`，单次使用，默认 5 分钟过期） |
| `addr` | 发送端局域网 IP:port |
| `sha256` | 载荷整体哈希（可选，接收端校验） |

### 4.2 HTTP 端点（发送端临时服务）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/transfer/{token}` | 下载载荷：响应头携带元数据，响应体为文件流 |
| GET | `/v1/health` | 探测服务存活（可选） |

- 响应头（下载）：`X-QRCD-Name`（原始文件名）、`X-QRCD-Size`（字节数）、`X-QRCD-SHA256`、`Content-Type`。
- token 校验失败 → `403`；token 已用 → `410 Gone`；过期 → `404`。
- 服务监听本机局域网 IP 的随机高位端口，传输完成或超时即关闭。

### 4.3 接收端流程

1. 扫描握手码 → 解析 `t` / `addr` / `sha256`。
2. `GET /v1/transfer/{t}` 拉取数据流 → 落 `.part` → SHA-256 校验 → rename。
3. 连接失败 / token 失效 / 超时 → 提示并自动回退纯光学（或提示重试）。

---

## 5. 手机网页 ↔ 桌面端交互接口（Phase 2 初稿）

> 手机网页为**自包含 PWA**，与桌面端无网络连接、无服务端交互。二者交互的唯一媒介是**二维码**，复用 §2 帧协议。此处约定网页内部的存储与能力接口。

### 5.1 能力对称性

| 能力 | 桌面端 CLI | 手机网页 |
|------|-----------|---------|
| 收（扫码解码） | `receive --source camera` | getUserMedia + jsQR / gozxing(wasm) |
| 存 | 落盘文件 | IndexedDB（见 03_database.md §4） |
| 发（编码播放） | `send`（ANSI 半块渲染） | Canvas 逐帧渲染 + 屏幕播放 |

### 5.2 网页内部操作（非网络 API，为页面动作）

| 动作 | 说明 |
|------|------|
| 接收 | 摄像头连续解码 §2 帧 → 还原 → 写入 IndexedDB（`qrcd/transfers`） |
| 列表 | 读取 IndexedDB 显示暂存项（名称/大小/时间） |
| 发送 | 选暂存项 → 按 §2 重新组帧 → Canvas 播放 QR 流 |
| 删除 | 删除暂存项（彻底清除，不可恢复） |
| 离线 | Service Worker 缓存静态资源，首次访问后离线可用 |

### 5.3 与桌面端一致性

- 帧协议、元数据 JSON 字段、CRC32/SHA-256 算法与桌面端**完全一致**（能力对称，PRD F-08）。
- 手机端编解码可用同一帧协议定义（以 JS 重实现或 wasm 复用 Go 核心，实现阶段再定，见 DEC 待补）。

---

## 6. 契约分组总览

| 分组 | 标识 | 说明 |
|------|------|------|
| CLI 命令 | `qrcd send` / `qrcd receive` | 桌面端唯一入口 |
| 二维码帧协议 | §2 | 纯光学 / 握手 / 中转的共同语言 |
| FEC 接口 | `internal/fec` | 喷泉码编解码契约 |
| 同网直传 | `qrcd://transfer` + `/v1/transfer/{token}` | Phase 2 加速路径 |
| 手机网页交互 | IndexedDB + 帧协议 | Phase 2 离线中转 |
