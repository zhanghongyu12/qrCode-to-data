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

### Changed

- DEC-004：`fec.Codec.NewDecoder` 契约增加 `messageLength` 参数（gofountain 必须）
- FEC 解码器复用单一 gofountain decoder 增量投喂（避免 AddBlocks 原地 XOR 污染重投数据）

### Deprecated

### Removed

### Fixed

### Security

---

## 版本历史

（暂无正式发布版本）
