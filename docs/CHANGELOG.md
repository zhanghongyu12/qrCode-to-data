# Changelog

本项目所有重要变更均会记录在此文件中。
格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

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

### Changed

- DEC-004：`fec.Codec.NewDecoder` 契约增加 `messageLength` 参数（gofountain 必须）
- FEC 解码器复用单一 gofountain decoder 增量投喂（避免 AddBlocks 原地 XOR 污染重投数据）
- 依赖新增 `skip2/go-qrcode`、`makiuchi-d/gozxing`（`gocv` 不入 go.mod，camera 由 build tag 隔离，启用 `-tags qrcd_camera` 时需另行安装 gocv/OpenCV）

### Deprecated

### Removed

### Fixed

### Security

---

## 版本历史

（暂无正式发布版本）
