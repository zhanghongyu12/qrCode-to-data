# qrcd

通过二维码在设备间传文件，**全程不联网**。屏幕显示二维码，摄像头扫描还原。适合气隙环境（无网络/物理隔离）下搬数据。

## 三个产物（双击即用）

Windows 用户：下载 `qrcd.exe` 和 `qrcd-web.bat`（放同一目录），**双击 `qrcd-web.bat`**——自动起服务并打开浏览器到三端首页：

| 产物 | 网址 | 作用 |
|------|------|------|
| ① 发送端 | http://localhost:8080/sender | 选文件，屏幕逐帧播放二维码 |
| ② 手机中继 | http://localhost:8080/relay | 手机扫码存下，再重放给接收端 |
| ③ 接收端 | http://localhost:8080/receiver | 扫码还原并下载文件 |

手机和电脑需在同一局域网；手机访问时把 `localhost` 换成电脑 IP（双击后窗口里会打印）。关闭那个黑色窗口即停止服务。

> 不装 Go、不装任何依赖，`qrcd.exe` 是独立可执行文件。

### 用法演示

**直接传（发送端 → 接收端）**
1. 电脑 A：`qrcd web` → 打开 `/sender` → 选文件
2. 电脑 B：`qrcd web` → 打开 `/receiver` → 摄像头对准 A 的屏幕 → 自动还原下载

**气隙中转（发送端 → 手机 → 接收端）**
1. 电脑 A：`qrcd web` → `/sender` 选文件
2. 手机：`qrcd web` → `/relay` → 扫 A 屏幕的二维码（自动逐帧捕获）→ 点「切换到重放」
3. 电脑 B：`qrcd web` → `/receiver` → 摄像头对准手机屏幕 → 自动还原下载

## 安装

```bash
git clone https://github.com/zhanghongyu12/qrCode-to-data.git
cd qrCode-to-data
go build -o qrcd ./cmd/qrcd
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

## 已知限制

- 接收端 Web 页面按系统源符号顺序重组（收齐 0..N-1 即还原），丢帧过多时需重扫；命令行 `qrcd receive` 支持完整 Raptor 纠错。
- 摄像头真实闭环需实机测试（光照/角度/距离）。
- 大文件全量入内存，适合 KB~MB 量级。

---

本项目基于 AI Work Model 多角色协作开发，协作体系见 [`docs/AI_WORK_MODEL.md`](docs/AI_WORK_MODEL.md)。
