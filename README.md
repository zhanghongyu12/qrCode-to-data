# qrcd

> 通过二维码实现纯光学数据传输的命令行工具。

`qrcd` 把任意文件或文本编码成二维码帧流，在发送端终端逐帧播放，接收端用摄像头（或文件/标准输入兜底）扫描还原——**全程不依赖网络**。适合物理隔离环境下的数据搬运、离线设备投递、Air-gap 跨界传输等场景。

- **纯光学传输**：屏幕二维码 ↔ 摄像头，零网络依赖。
- **喷泉码容错**：Raptor 码（RFC 5053）+ 冗余符号，丢帧/乱序/元数据迟到均可还原。
- **完整性校验**：CRC32 帧校验 + SHA-256 整体校验，原子落盘（`.part` → rename）。
- **四档退出码**：`0` 成功 / `1` 传输失败 / `2` 参数错误 / `3` 环境错误，便于脚本集成。

> 当前版本 **v0.1.0（首发 MVP）**。Phase 2 计划支持同网 WiFi 直传加速与手机离线中转（`--net` 相关参数已预留占位，当前仅提示未实现）。

---

## 环境要求

| 项 | 要求 |
|----|------|
| 操作系统 | Windows / macOS / Linux |
| Go | 1.25+ |
| 摄像头接收（可选） | OpenCV 4 + Go 绑定（gocv），需用构建标签 `-tags qrcd_camera` 启用；**默认构建不含摄像头**，用 `--source file`/`stdin` 兜底 |

### Windows 环境注意

- 若 `GOROOT` 被误配（如指向空目录），go 命令前需 `unset GOROOT`。
- 部分国产安全软件（如 360）会锁定刚编译出的 `.exe`，报「process cannot access the file」。建议把项目 `.tmp/` 目录与 Go 临时构建目录加入信任区；或设 `GOTMPDIR=<项目>/.tmp` 并用 `go test -p 1` 顺序执行。

---

## 安装

```bash
# 方式一：直接安装到 $GOPATH/bin
go install qrcd@v0.1.0

# 方式二：从源码构建
git clone https://github.com/zhanghongyu12/qrCode-to-data.git
cd qrCode-to-data
unset GOROOT   # 仅 GOROOT 误配时需要
go build -o qrcd ./cmd/qrcd
```

启用摄像头接收（需先装好 OpenCV 4 + gocv 环境）：

```bash
go build -tags qrcd_camera -o qrcd ./cmd/qrcd
```

> 不带 `-tags qrcd_camera` 时，`receive --source camera` 会报环境错误（退出码 3）。

---

## 快速开始

```bash
# 1. 发送端：把文件编码成二维码流，在终端逐帧播放
qrcd send ./backup.tar.gz

# 2. 接收端：用摄像头扫描还原（需启用 qrcd_camera 构建）
qrcd receive --output ./received/

# 无摄像头？用文件兜底（把二维码图片逐帧喂给接收端）
qrcd receive --source file ./frames/ --output ./received/
```

发送文本：

```bash
qrcd send --text "Hello World"
```

---

## 子命令

### `qrcd send` — 发送

```
qrcd send <file>              # 发送文件
qrcd send --text <文本>        # 发送文本（与 <file> 二选一）
```

**参数：**

| 参数 | 默认 | 说明 |
|------|------|------|
| `<file>` | — | 要发送的文件路径（位置参数，与 `--text` 二选一） |
| `-t, --text` | — | 直接发送文本（与 `<file>` 二选一） |
| `--fps` | 10 | 二维码播放帧率（帧/秒） |
| `-v, --version` | 20 | QR 版本上限（1~40），载荷不足自动降级到更小版本 |
| `--ecc` | L | QR 纠错级别 L/M/Q/H |
| `-r, --redundancy` | 0.1 | 喷泉码冗余度（0~1，0.1 = 多 10% 编码符号，丢帧更多时调高） |
| `-b, --block-size` | 1024 | 源分块大小（字节） |
| `--terminal` | ansi | 渲染器：`ansi`（终端半块）/ `ascii`（降级） |
| `--invert` | false | 反色（浅色终端背景时启用） |
| `-q, --quiet` | false | 关闭进度，只输出最终结果 |
| `--net` | auto | 同网直传 `auto/on/off`（**Phase 2 未实现**，当前仅占位） |
| `--addr` | — | 指定监听 IP（**Phase 2**） |

**短文本直传**：文本 ≤ 200 字节时走静态标准文本二维码（不走帧流），任何第三方扫码 App 可读；超过 200 字节自动切换为二维码帧流。

**示例：**

```bash
qrcd send ./report.pdf --fps 15 --redundancy 0.2
qrcd send --text "一次性口令 ABC123" --quiet
qrcd send bigfile.bin -v 25 --ecc M -r 0.3
```

### `qrcd receive` — 接收

```
qrcd receive [路径]           # 默认从摄像头接收
qrcd receive --source file <路径>   # 从二维码图片文件或目录接收
qrcd receive --source stdin        # 从标准输入接收（帧字节流）
```

**参数：**

| 参数 | 默认 | 说明 |
|------|------|------|
| `[路径]` | — | 位置参数，仅 `--source file` 时用：二维码图片文件或含图片的目录 |
| `-o, --output` | . | 输出目录或完整文件名 |
| `--camera` | 0 | 摄像头设备号 |
| `--source` | camera | 输入源：`camera`/`file`/`stdin` |
| `--fps` | 30 | 采集/解码帧率上限 |
| `-v, --version` | 20 | 期望 QR 版本上限 |
| `--expect-size` | 0 | 预期总字节数（可选，元数据到达时提前校验） |
| `--hash` | — | 预期校验值，格式 `sha256:<64位十六进制>` |
| `--timeout` | 30s | 无新符号超时（如 `30s`，提示卡死并允许 Ctrl+C 终止） |
| `--overwrite` | false | 允许覆盖已存在文件 |
| `-q, --quiet` | false | 关闭进度，只输出最终结果 |

**示例：**

```bash
qrcd receive --output ./out/                          # 摄像头接收，落到 ./out/
qrcd receive --source file ./qr-frames/ -o ./out/     # 从图片目录接收
qrcd receive --hash sha256:9f86d...e3a0 --overwrite   # 校验哈希并允许覆盖
```

---

## 退出码

| 码 | 含义 | 典型场景 |
|----|------|----------|
| `0` | 成功 | 发送完成 / 接收还原且校验通过 |
| `1` | 传输失败 | 符号不足未能还原、SHA-256 校验失败、超时 |
| `2` | 参数错误 | 缺少必填参数、参数越界、`--hash` 格式错误 |
| `3` | 环境错误 | 摄像头不可用（未启用 `qrcd_camera` 构建）、输出路径不可写 |

脚本集成示例（bash）：

```bash
qrcd receive --output ./out/ --hash sha256:9f86d...e3a0
case $? in
  0) echo "接收成功";;
  1) echo "传输失败，请重发或调整扫描角度"; exit 1;;
  2) echo "参数错误"; exit 2;;
  3) echo "环境错误（摄像头/权限）"; exit 3;;
esac
```

---

## 工作原理

```
发送端                            接收端
payload 分块                      帧源取流（camera/file/stdin）
    ↓                                 ↓
SHA-256 整体哈希                  QR 图像解码（gozxing）
    ↓                                 ↓
Raptor 喷泉码编码                 frame 拆帧（CRC32 校验 / seq 去重）
    ↓                                 ↓
frame 组帧（32B 头 + 载荷 + 4B CRC32）   元数据到达 → 初始化会话/解码器
    ↓                                 ↓
QR 生成 + 终端逐帧播放  ──光学──▶  FEC 解码还原
                                        ↓
                                    SHA-256 校验 → .part 临时文件 → rename 原子落盘
```

- **帧协议**：32 字节帧头（magic `QRCD` / 版本 / 类型 / 标志 / 16B 传输 ID / seq / len）+ 载荷 + 4B CRC32（IEEE，大端）。
- **元数据帧**：携带文件名、大小、分块、哈希、FEC 方案等，每 20 个数据帧重播一次，保证接收端迟早收到。
- **容错**：丢帧/乱序/元数据迟到经喷泉码兜底；冗余度越高越抗丢，但总帧数越多。

---

## 故障排查

| 现象 | 可能原因 | 处理 |
|------|----------|------|
| `符号不足，未能还原数据` | 帧丢失过多超过冗余覆盖；扫描角度/距离问题 | 调高 `--redundancy`（如 0.2~0.3）、改善扫描条件、重发 |
| `SHA-256 校验失败` | 帧内容损坏但凑齐了符号 | 重发；检查屏幕/摄像头是否有反光、闪烁 |
| 退出码 3 `摄像头不可用` | 未用 `-tags qrcd_camera` 构建，或无 OpenCV | 改用 `--source file`/`stdin`，或装好 gocv 环境重新构建 |
| `process cannot access the file` | 安全软件锁定编译产物（Windows） | 把 `.tmp/` 与 Go 临时目录加入信任区；设 `GOTMPDIR` + `go test -p 1` |
| 发送文本时报「超过短文本上限」 | 文本 > 200 字节 | 正常，自动切帧流；或拆短文本 |

---

## 已知限制（v0.1.0）

- **Phase 2 未实现**：`--net` 同网 WiFi 直传、`--addr`、手机离线中转、`--terminal window` GUI 渲染——当前仅占位，调用会提示未实现。
- **摄像头真实闭环**：默认构建不含 gocv，实机摄像头采集未在 CI 验证，需装好环境并 `-tags qrcd_camera` 构建后在实机测试。
- **大文件内存**：当前载荷与图片源全量入内存，适合 KB~MB 量级；GB 级载荷需后续流式改造。

详见 `docs/08_review.md`（已知项 W-01~W-03）与 `docs/09_test_report.md`。

---

## 开发与协作

本项目基于「AI Work Model」多角色协作模板开发（PM/架构/设计/开发/审查/测试）。协作体系、角色职责、工作流程详见 **[`docs/AI_WORK_MODEL.md`](docs/AI_WORK_MODEL.md)**。

项目文档位于 `docs/`：

| 文档 | 内容 |
|------|------|
| `00_idea.md` ~ `05_ui.md` | 想法 / PRD / 架构 / 数据库 / API / UI |
| `06_tasks.md` | 任务清单与项目状态 |
| `07_decisions.md` | 重大决策记录（DEC-001~006） |
| `08_review.md` | 代码审查报告 |
| `09_test_report.md` | 测试报告 |
| `CHANGELOG.md` | 变更日志 |

---

## 许可证

见仓库根目录 LICENSE（如未声明，默认版权归项目维护者）。
