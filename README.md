# qrcd

两台电脑之间不能联网（比如涉密、物理隔离的环境），但又想把文件传过去，怎么办？这个工具用**二维码**来传：一台电脑把文件变成不断滚动的二维码显示在屏幕上，手机对着屏幕扫，再把扫到的内容交给另一台电脑，还原成原来的文件。**全程不联网。**

## 三个安装包

按角色分成三个，各装各的。包名和界面里都不会出现「发送 / 接收」这类字眼。

| 文件 | 装在哪 | 做什么 | 托盘图标 |
|------|--------|--------|----------|
| `qrcd-a.exe` | 电脑 A | 选文件，屏幕滚动放二维码 | **A** |
| `qrcd-b.exe` | 电脑 B | 接收数据，还原成文件保存 | **B** |
| `qrcd-relay.apk` | 手机 | 对着屏幕扫码，转发给电脑 B | — |

前两个是 Windows 桌面程序，双击就能用，不用装 Go 或任何依赖（靠系统自带的 Edge WebView2，Win10 / 11 都自带）。关掉窗口程序不会退出，只是缩到右下角托盘里；在托盘图标上点右键，可以「打开桌面端」或「退出」。

## 怎么用

**电脑 A 放码 → 手机扫码 → 电脑 B 收文件**

1. 电脑 A 上双击 `qrcd-a.exe`，选一个文件，点「开始」，屏幕就开始滚动放二维码。默认设置一般就能用；扫不动就把速度调慢一点。
2. 手机上装 `qrcd-relay.apk`，把「电脑地址」填成**电脑 B 的 IP:8080**，然后把镜头对准电脑 A 的屏幕扫，扫完点提交。
3. 电脑 B 上双击 `qrcd-b.exe`，会自动接收并还原，文件存在 `downloads/` 目录。

> 手机要和电脑 B 连同一个网络。地址填电脑 B 的局域网 IP（电脑自己开热点的话，填热点网关地址，比如 `192.168.253.1:8080`）。如果只在同一台电脑上同时跑 A 和 B 做测试，用 `--addr` 把端口错开（电脑 B 用 8080，电脑 A 用 `--addr :8081`）。

## 自己编译

```bash
git clone https://github.com/zhanghongyu12/qrCode-to-data.git
cd qrCode-to-data

# 电脑 A / 电脑 B：-H windowsgui 表示不弹黑色控制台窗口，role 决定是哪个角色
go build -ldflags "-X main.role=a -H windowsgui" -o qrcd-a.exe ./cmd/qrcd
go build -ldflags "-X main.role=b -H windowsgui" -o qrcd-b.exe ./cmd/qrcd
# 完整版（三个页面 + 命令行）
go build -o qrcd.exe ./cmd/qrcd

# 手机 App（产物在 app/build/outputs/apk/debug/app-debug.apk）
cd android && ./gradlew.bat assembleDebug
```

需要 Go 1.25+。Windows 上如果 360 等安全软件拦截编译产物，把项目 `.tmp/` 目录加进信任区。

## 命令行（可选）

图形界面已经覆盖主要流程，命令行留给脚本和自动化：

```bash
qrcd send ./file.zip            # 在终端里滚动放二维码
qrcd receive -o ./out/          # 用摄像头接收（需 -tags qrcd_camera 编译）
qrcd receive --source file ./帧图目录 -o ./out/   # 没摄像头时从图片目录读
```

- 退出码：`0` 成功 / `1` 传输失败 / `2` 参数错误 / `3` 环境错误
- 每帧 CRC32 校验，整体 SHA-256 校验，写完后才改文件名（不会写出半个文件）
- 用 Raptor 喷泉码加冗余，漏几帧、顺序乱都能还原

`qrcd send --help` / `qrcd receive --help` 看完整参数。

## 网络与防火墙

- 电脑自己打开的页面（`/sender`、`/receiver`、首页）用 `http://localhost:8080`，本机永远能通，不受网络切换影响（不要填网卡 IP，网络一变会报 `ERR_NETWORK_CHANGED`）。
- 手机（App 里的地址，或浏览器开 `/relay`）要填电脑的局域网 IP，手机和电脑要在同一个网络里。电脑开热点时，手机连上热点后填热点网关地址（如 `192.168.253.1:8080`）。
- 手机 App：仓库根目录有 `qrcd-relay.apk`（也可在 `qrcd web` 的 `/dl/app.apk` 下载，或 `adb install` 安装），用 ML Kit 解码。「电脑地址」在 App 里填，留空会提示你填；会自动补上 `http://` 和 `/api/ingest`。
- 防火墙：第一次从别的设备访问电脑的 8080 / 8443 会被 Windows 防火墙拦。运行仓库根目录的 `add-firewall.ps1`（右键「用 PowerShell 运行」，UAC 点「是」）一次性放行；用电脑自带的「移动热点」共享时通常会自己放行，不用跑这个脚本。

## 技术细节（写给开发者）

- **边扫边传**：扫到一帧就立刻入队上传，同时最多 6 个上传请求在飞，不用等全部扫完；个别帧失败就丢掉，靠喷泉码补回来。
- **requestAnimationFrame 驱动扫码**：去掉了原来 120ms 一次的定时器（那个把速度卡在约 8fps）；合成摄像头下现在能跑到约 32–38fps。
- **jsQR 放进 Web Worker**：解码不卡主线程；worker 出问题会自动回退到同步解码。
- **二维码用二进制模式（byte-mode）**：帧字节直接进二维码，不做 base64 膨胀。
- **纠错等级用 L（7%）**：二维码更稀疏、更好扫，漏掉的帧由喷泉码补。

接收端（`/api/ingest` → `receive.Processor`）和命令行 `qrcd receive` 都用 Raptor 喷泉码：顺序乱、漏帧都不影响正确性，只是多花点时间；每帧 CRC32 + 整体 SHA-256 校验，写完才改文件名。

## 已知限制

- 摄像头真实对拍（光照、角度、距离）要实机测；合成摄像头测出来的 fps 只是处理能力，真机还要看摄像头。
- 大文件会整个读进内存，适合 KB 到 MB 级别。
- relay 页面是单线程串行解码，帧率特别高时解码会成为瓶颈（可以改成多线程并行，还没做）。

---

本项目基于 AI Work Model 多角色协作开发，协作体系见 [`docs/AI_WORK_MODEL.md`](docs/AI_WORK_MODEL.md)。
