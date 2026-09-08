# qrcd

通过二维码在设备间传数据，**全程不联网**。屏幕显示二维码，摄像头扫描还原。适合气隙环境（无网络/物理隔离）下搬数据。

## 三个安装包（双击即用）

按角色拆成三个独立安装包，**包名与界面均使用中性命名**（不体现「发送/接收」）：

| 安装包 | 角色 | 托盘字母 | 用途 |
|--------|------|----------|------|
| `qrcd-a.exe` | 播放端 | **A** | 选文件 → 屏幕播放二维码 |
| `qrcd-b.exe` | 还原端 | **B** | 等手机提交 → 进度 → 保存文件 |
| `qrcd-relay.apk` | 中继端 | — | 手机扫码 → 提交到电脑 |

其中 `qrcd-a.exe` / `qrcd-b.exe` 为**桌面原生窗口（WebView2）+ 右下角托盘常驻**：双击即用、无控制台黑窗、不装 Go/任何依赖（需系统 Edge WebView2 运行时，Win10/11 默认自带）。关窗进程不退，托盘右键可「打开桌面端」或「退出」；`qrcd-relay.apk` 为 Android 手机 App。

### 用法

**气隙流程（播放端 → 手机 → 还原端）**
1. 电脑 A：双击 `qrcd-a.exe`（托盘 A）→ 选文件 → 点「开始」→ 屏幕放码（建议 fps 8–12、网格 2×2、密度 350、实时扫描）
2. 手机：装 `qrcd-relay.apk` → 「电脑地址」填 **还原端电脑 IP:8080** → 扫码对准 A 屏幕 → 提交
3. 电脑 B：双击 `qrcd-b.exe`（托盘 B）→ 自动还原并落盘到 `downloads/`

> 手机与还原端需在同一局域网；手机填还原端电脑的局域网 IP（开「移动热点」时填热点网关地址，如 `192.168.253.1:8080`）。同一台电脑上同时测试两端时，用 `--addr` 错开端口（如还原端 8080、播放端 `--addr :8081`）。

## 构建

```bash
git clone https://github.com/zhanghongyu12/qrCode-to-data.git
cd qrCode-to-data

# 播放端（A）/ 还原端（B）：-H windowsgui 去掉控制台窗口，role 编译进独立安装包
go build -ldflags "-X main.role=a -H windowsgui" -o qrcd-a.exe ./cmd/qrcd
go build -ldflags "-X main.role=b -H windowsgui" -o qrcd-b.exe ./cmd/qrcd
# 完整开发版（三端页签 + CLI send/receive）
go build -o qrcd.exe ./cmd/qrcd

# Android 中继 App（产物 app/build/outputs/apk/debug/app-debug.apk）
cd android && ./gradlew.bat assembleDebug
```

需要 Go 1.25+。Windows 上若 360 等安全软件拦截编译产物，把项目 `.tmp/` 目录加入信任区。

## 命令行（可选）

Web 三端已覆盖主流程，命令行用于脚本/自动化：

```bash
qrcd send ./file.zip            # 终端播放二维码
qrcd receive -o ./out/          # 摄像头接收还原（需 -tags qrcd_camera 构建启用摄像头）
qrcd receive --source file ./帧图目录 -o ./out/   # 无摄像头时从图片目录接收
```

- 退出码：`0` 成功 / `1` 传输失败 / `2` 参数错误 / `3` 环境错误
- 传输完整性：每帧 CRC32 + 整体 SHA-256 校验，原子落盘
- 容错：Raptor 喷泉码冗余符号，丢帧/乱序可还原

`qrcd send --help` / `qrcd receive --help` 查看完整参数。

## 实现细节

**接收/中继管线（`internal/web/pages.go` 的 relay 页）**
- **边扫边发**：扫到一帧立即入队上传，`pumpUpload` 保持并发度 6 的 in-flight，无需「扫完再发」；失败帧丢弃由喷泉码兜底，不重试不阻塞。
- **rAF 驱动扫描**：`requestAnimationFrame` + 防重入，移除了旧的 `setTimeout 120ms`（≈8fps 硬上限）；合成摄像头实测管线吞吐 ~32–38fps（旧基线 ~8fps，约 4×）。
- **jsQR 移入 Web Worker**：Blob-URL worker 经 `importScripts(location.origin+'/jsQR.js')` 加载 jsQR，主线程不被同步解码阻塞；worker 出错自动置空回退同步 jsQR。
- **二进制 byte-mode QR**：帧字节直接入 QR（无 base64 膨胀），relay 经 jsQR 的 `binaryData` 可靠还原二进制字节；BarcodeDetector 的 `rawValue` 对二进制不可靠，仅作回退。端到端实测：合成摄像头喂真实二进制帧 → relay → `/api/ingest` → Raptor 还原，SHA-256 精确匹配。
- **ECC 用 L**：`handleFrame` 用 QR 纠错等级 L（7%），帧内 ECC 与喷泉码分工（corruption vs erasure），L 最大化单帧稀疏度（更易扫），丢帧由喷泉码兜底。

**纠错**：接收端 Web 页面（`/api/ingest` → `receive.Processor`）与命令行 `qrcd receive` 均使用 Raptor 喷泉码——任意顺序到达、丢帧只损失时间不损失正确性；每帧 CRC32 + 整体 SHA-256 校验，原子落盘。

## 部署 / 网络

- **电脑自己用的页面**（`/sender`、`/receiver`、首页）一律用 `http://localhost:8080`——本机永远通，不受网络切换影响（别用网卡 IP，网络一变会 `ERR_NETWORK_CHANGED`）。
- **手机**（App 里的服务器地址，或浏览器开 `/relay`）用电脑的局域网 IP；手机和电脑需在同一网络。电脑开「移动热点」时，手机连热点后填热点网关地址（如 `192.168.253.1:8080`）即可。
- **手机中继 App**（Android）：仓库根目录有 `qrcd-relay.apk`（由 `qrcd web` 的 `/dl/app.apk` 提供下载，或 `adb install` 安装），用 ML Kit 解码。「电脑地址」在 App 内填写（留空会提示填写，自动补 `http://` 与 `/api/ingest`），真机上填还原端电脑 IP:8080。
- **防火墙**：首次从外部设备访问电脑 8080/8443，Windows 防火墙会拦入站。运行仓库根目录的 `add-firewall.ps1`（右键「用 PowerShell 运行」，UAC 点「是」）一次性放行 8080+8443；经电脑自带「移动热点」共享时通常自动放行，无需此脚本。

## 已知限制

- 摄像头真实闭环（光照/角度/距离）需实机测试；合成摄像头下的 fps 是管线吞吐，真机还取决于摄像头。
- 大文件全量入内存，适合 KB~MB 量级。
- relay 页单 worker 串行解码，极高帧率下解码是瓶颈（可改多 worker 池进一步并行，未实现）。

---

本项目基于 AI Work Model 多角色协作开发，协作体系见 [`docs/AI_WORK_MODEL.md`](docs/AI_WORK_MODEL.md)。
