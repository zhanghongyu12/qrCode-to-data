# 任务清单

> 状态：持续更新
> 最后更新：2026-08-17
> 维护者：全员可更新

## 项目状态

- 运行模式：编排模式
- 当前阶段：阶段 6 - 编码
- 下一步行动：TASK-001~011 已编码完成，待进入阶段 7 测试
- 阻塞项：无
- 运行模式说明：`独立模式`（角色向用户确认后自提交）或 `编排模式`（角色暂存不提交，经编排者批准后自提交；编排者做集成 merge；push 由编排者申请、用户批准）。编排者激活/退出时翻转本字段。角色启动时读本字段判断提交权。
- 阻塞项格式说明：无阻塞时填"无"；有阻塞时列出决策编号及简述，如 `DEC-003（待人工确认 API 方案）、DEC-005（待人工确认 UI 与 API 对齐）`。AI 记录阻塞决策到 07_decisions.md 时必须同步更新本字段。

## 协调日志

> 编排模式下，编排者记录门禁放行、提交批准/拒绝等动作，留审计痕迹。独立模式下留空。

（暂无）

### 2026-08-14 阶段6 编码启动 - Dev-A

- 调度 Developer（Sonnet）实现 TASK-001/002/003，子代理连续多次因模型 API 故障失败，最终产出 frame/fec 代码骨架。
- 编排者审查发现 FEC 解码器缺陷：gofountain 的 `Decoder.AddBlocks` 会原地 XOR 修改 block.Data，wrapper 每次 AddSymbol 重建 decoder 重投喂已损坏数据，导致解码错误。修复为复用单一 decoder 增量投喂。
- 发现 04_api.md §3 的 `NewDecoder(sourceSymbols)` 契约缺 messageLength 参数（gofountain 必须）。记 DEC-004 改契约为 `NewDecoder(sourceSymbols, messageLength)`，同步更新 04_api.md §3 与三个实现。
- 发现 gofountain 在低 K + 顺序 id 下存在伪满秩局限（done=true 但解码错误）。记 DEC-005，丢包测试改用较大 K（50）+ 较长数据。
- `go build ./...` 与 `go test ./...` 全绿。TASK-001/002/003 状态→待测试。
- 提交：本轮代码以 Developer 身份提交（编排者代行，因子代理连续故障无法自提交）。

### 2026-08-17 阶段6 收尾 - Dev-C

- 调度 Developer（Dev-C）完成 TASK-008/009/010/011：发送编排（短文本 F-05 直传 + QR 帧流播放 + 元数据周期重播）、接收编排（CRC 拆帧/seq 去重/元数据迟到缓冲/FEC 解码/SHA-256 校验/.part 落盘）、CLI 入口（退出码四档 0/1/2/3）、端到端纯光学闭环联调。
- 复核 CLI flag 绑定：send/receive 全部使用 Var* 变体（StringVarP/IntVarP/Float64VarP/Int64VarP/DurationVar/BoolVarP）绑定 opts，无 StringP/IntP 等非绑定变体遗漏。
- 发现并修复 `--timeout` 默认值契约偏差：04_api §1.4 规定默认 30s，原代码绑定 0（等效禁用超时提示），已改为 30s。
- 启动项核查：gofountain 解码器单一实例增量投喂（无重建重投喂）、K≥50 规避低 K 伪满秩、短文本 ≤200B 走 F-05 直传，均无回归。
- 删除临时探针 probe_e2e_test.go；正式单元测试落位 tests/unit/sendreceive/（闭环/丢帧/元数据迟到/none 短路/短文本/重播/去重/QR 图片/expect-size/hash 共 11 例）。
- `unset GOROOT && go build ./...`、`go vet ./...`、`go test ./...` 全绿；CLI 实测：`qrcd send` 缺参输出 usage 退出码 2，`qrcd receive --camera 0`（无 gocv 环境）报环境错误退出码 3。
- TASK-008/009/010 状态→待测试，TASK-011（纯光学闭环）→已完成；看板更新为 待办2/进行中10/已完成1。代码已 git add 暂存（未 commit），待编排者批准提交。

---

## 看板总览

| 待办 | 进行中 | 已完成 | 已阻塞 |
|------|--------|--------|--------|
| 2    | 10     | 1      | 0      |

---

## 待办 (TODO)

## TASK-001: 项目骨架与 Go 模块初始化

- 状态：待测试
- 优先级：P0
- 负责角色：Developer
- 关联需求：PRD F-01；02_architecture §1/§3/§5.2；04_api §1
- 估算：0.5 天
- 依赖关系：无
- 描述：初始化 Go 1.22+ 模块，创建 cmd/qrcd 与 internal/{payload,fec,frame,qrcode,capture,send,receive,progress} 目录；引入并锁定依赖 cobra、gofountain、skip2/go-qrcode、makiuchi-d/gozxing、gocv；建立 cobra 入口占位（`qrcd --help` 输出中文 usage）。
- 验收标准：
  - go.mod 存在且 Go 版本 ≥ 1.22，依赖锁定（go.sum 已提交）。
  - 目录结构与 02_architecture §3 一致，各 internal 包可 import。
  - `go build ./...` 与 `go test ./...` 均通过。
  - `go run ./cmd/qrcd --help` 输出中文 usage 且退出码 0。

## TASK-002: 二维码帧协议 internal/frame

- 状态：待测试
- 优先级：P0
- 负责角色：Developer
- 关联需求：PRD F-02/F-03/F-06；02_architecture §3/§4.1/§4.2；04_api §2
- 估算：1.5 天
- 依赖关系：TASK-001
- 描述：实现 04_api §2 帧协议：固定 32 字节头（magic "QRCD"/version/type/flags/transfer_id/seq/len）+ payload + 4 字节 CRC32（IEEE，大端，对头+payload 计算）。提供组帧（元数据帧/数据帧）、拆帧（校验 magic/version/CRC32，失败丢弃）、元数据帧 JSON 序列化/解析、数据帧按 seq 去重、transfer_id 会话区分等纯函数接口。
- 验收标准：
  - 单元测试：合法帧「组帧→拆帧」字节一致；CRC32 篡改 1 字节→拆帧报错并丢弃；magic/version 非法→丢弃；seq 重复帧→去重忽略。
  - 元数据帧 JSON 字段与 04_api §2.3 完全一致（name/size/blockSize/blockCount/hashAlgo/hash/fec/redundancy/payloadType/mimeType）。
  - 整数大端、文本 UTF-8、CRC32 大端，与 04_api §2.2 一致。
  - 空 payload 与 len 与实际长度不符时明确处理，不 panic。

## TASK-003: 喷泉码编解码 internal/fec

- 状态：待测试
- 优先级：P0
- 负责角色：Developer
- 关联需求：PRD F-04；02_architecture §1/§3；04_api §3
- 估算：2 天
- 依赖关系：TASK-001
- 描述：实现 04_api §3 的 FEC 接口契约（Codec/Encoder/Decoder），适配 gofountain。默认 Raptor 码（RFC 5053），LT 码兜底；blockCount=1 时 `fec="none"` 短路不经过 FEC。保证任意足量互不相同符号可无损还原、顺序无关。
- 验收标准：
  - 接口签名与 04_api §3 完全一致。
  - 单元测试：乱序 + 去重投喂 ≥ 阈值符号后 Decode 结果与原文逐字节一致；确定性种子下丢块 10~30% 场景可还原。
  - 空载荷（size=0）返回空字节不 panic；单块载荷走 `none` 短路。
  - `Scheme()` 返回 raptor/lt/none；`NextSymbol()` 可无限产出且 id 递增；`Received()` 正确统计不重复符号数。

## TASK-004: 二维码生成与解析 internal/qrcode

- 状态：待测试
- 优先级：P0
- 负责角色：Developer
- 关联需求：PRD F-02/F-03/F-05；02_architecture §1/§3/§7；04_api §2.5/§2.6
- 估算：2 天
- 依赖关系：TASK-001
- 描述：封装 skip2/go-qrcode（生成）与 makiuchi-d/gozxing（解析）。生成：按 version 上限（默认 20，1~40）与 EC 级别（默认 L）编码字节为 QR 位图，超容量自动降级/报错；提供 ANSI 半块字符（▀▄█）渲染 + ASCII 降级 + ≥4 模块静区 + 光标复位帧刷新。解析：字节/位图→解码内容。短文本（≤200 字节）直接编码标准文本二维码。
- 验收标准：
  - 生成的 QR 位图可被 gozxing 自行解码回原文（闭环）；短文本二维码可被通用解码器读取。
  - ANSI 渲染仅含半块字符/空格、含静区、无彩色；ASCII 降级可用。
  - 载荷超出 version 容量时返回明确错误或自动降级，不产生截断。
  - version（1~40）/ecc（L/M/Q/H）参数范围校验，非法值报参数错误。

## TASK-005: 载荷读写与校验 internal/payload

- 状态：待测试
- 优先级：P0
- 负责角色：Developer
- 关联需求：PRD F-01/F-05/F-06；02_architecture §3/§4.1/§4.2；04_api §2.3
- 估算：1.5 天
- 依赖关系：TASK-001
- 描述：载荷读取（文件/文本/可选 stdin）与写入（输出目录/文件名）；分块（blockSize 默认 1024）；SHA-256 与 CRC32 计算；`.part` 临时文件 + 校验通过后 rename 原子落盘；`--overwrite` 覆盖控制；文本摘要名生成。仅读写用户显式指定路径，不做越界路径访问。
- 验收标准：
  - 文本/文件两类载荷读取字节一致；分块边界正确（含尾部不满一块）。
  - SHA-256/CRC32 与标准工具（sha256sum）结果一致。
  - 写入走 `.part`→rename；校验失败不产生最终文件并清理 `.part`；`--overwrite=false` 时已存在文件报错。
  - 文件不存在/无读取权限→中文报错；输出目录不可写→报错。

## TASK-006: 摄像头采集 internal/capture

- 状态：待测试
- 优先级：P0
- 负责角色：Developer
- 关联需求：PRD F-03；02_architecture §1/§3/§5.1；04_api §1.4
- 估算：2 天
- 依赖关系：TASK-001
- 描述：摄像头帧采集（gocv，跨平台），支持 `--camera` 设备号；提供 `--source file|stdin` 兜底（从图片/视频文件或 stdin 读取帧）。通过接口隔离屏蔽 CGO/OpenCV 依赖，使解码层只依赖抽象帧源。无摄像头环境不 panic、报明确中文错误。
- 验收标准：
  - 抽象帧源接口清晰，camera/file/stdin 三种实现可插拔。
  - `--source file` 逐帧读取图片/视频供解码；`--source stdin` 可读入字节流。
  - `--camera N` 打开指定设备；设备不存在/权限拒绝→中文报错 + 退出码 3。
  - 无 gocv/OpenCV 构建环境下，`--source file|stdin` 路径仍可编译运行（build tag 隔离）。

## TASK-007: 进度与统计 internal/progress

- 状态：待测试
- 优先级：P0
- 负责角色：Developer
- 关联需求：PRD F-06；02_architecture §3/§5.3；04_api §1.3/§1.4
- 估算：1 天
- 依赖关系：TASK-001
- 描述：传输进度/速率/校验结果统计与终端展示。发送端：已播块数/总块数、百分比、fps、预计剩余；接收端：唯一块数/期望块数、百分比、去重数、速率。统一进度事件接口供 send/receive 调用。`--quiet` 关闭进度只输出最终结果；`--timeout` 无新块超时提示卡死。进度输出到 stdout，错误到 stderr。
- 验收标准：
  - 进度条实时刷新（ANSI 光标复位，不滚动累积），字段含百分比/已收（发）块/速率。
  - 最终 100% 与校验结果一致；`--quiet` 下无进度输出仅保留最终结果。
  - 无新块超 `--timeout`→提示并可 Ctrl+C 终止。
  - 去重数、唯一块数统计与 frame/fec 实际行为一致。

## TASK-008: 发送编排 internal/send

- 状态：待测试
- 优先级：P0
- 负责角色：Developer
- 关联需求：PRD F-02/F-05；02_architecture §3/§4.1；04_api §1.3
- 估算：2 天
- 依赖关系：TASK-002、TASK-003、TASK-004、TASK-005、TASK-007
- 描述：发送编排：payload 读取分块→计算整体 SHA-256（写入元数据帧）→fec 编码→frame 组帧（元数据帧每 20 个数据帧重播一次）→qrcode 生成 + ANSI 渲染逐帧播放。短文本（≤200 字节）走 F-05 直传路径（静态单张/少量标准文本二维码，不启动动画）。支持 --fps/--version/--ecc/--redundancy/--block-size/--terminal/--invert；Ctrl+C 停止释放资源；载荷过大提示预估时长。
- 验收标准：
  - 任意二进制文件可稳定生成 QR 流，无 panic；QR 可被自研解析器解码。
  - 元数据帧周期性重播（每 20 数据帧一次）可被观察验证。
  - 短文本 ≤200 字节静态显示标准文本二维码（第三方 App 可读）；超长自动切换 QR 流并提示。
  - 播放速率可调（--fps）；Ctrl+C 立即停止释放资源；载荷过大打印预估时长 + 建议同网加速。

## TASK-009: 接收编排 internal/receive

- 状态：待测试
- 优先级：P0
- 负责角色：Developer
- 关联需求：PRD F-03/F-05/F-06；02_architecture §3/§4.2；04_api §1.4
- 估算：2 天
- 依赖关系：TASK-002、TASK-003、TASK-004、TASK-005、TASK-006、TASK-007
- 描述：接收编排：capture 取帧→qrcode 解码→frame 拆帧（CRC 校验/去重/元数据帧初始化会话）→fec 解码（收够足量还原）→payload 校验（SHA-256）→`.part` 落盘→rename。支持 --output/--camera/--source/--fps/--version/--expect-size/--hash/--timeout/--overwrite；`--hash` 预校验；校验失败丢弃损坏数据并提示重传。
- 验收标准：
  - 能从 QR 流（含漏帧/乱序，配合 FEC）稳定还原数据，逐字节一致。
  - 元数据帧迟到/中途加入仍能初始化会话并还原（周期重播）。
  - 校验通过输出文件路径/大小/SHA-256 +「校验通过」，退出码 0；失败丢弃损坏数据、提示重传、退出码 1。
  - `--expect-size`/`--hash` 不匹配时报错；重复帧去重不重复计数。

## TASK-010: CLI 入口与参数契约 cmd/qrcd

- 状态：待测试
- 优先级：P0
- 负责角色：Developer
- 关联需求：PRD F-01；02_architecture §3/§5.1；04_api §1
- 估算：1.5 天
- 依赖关系：TASK-008、TASK-009
- 描述：用 cobra 实现 `qrcd send`/`qrcd receive` 子命令，参数名与默认值与 04_api §1 完全对齐（send：--text/--fps/--version/--ecc/--redundancy/--block-size/--terminal/--invert/--quiet/--net/--addr；receive：--output/--camera/--source/--fps/--version/--expect-size/--hash/--timeout/--overwrite/--quiet）。退出码 0 成功/1 传输或解码失败/2 参数错误/3 环境错误；帮助与错误提示中文；错误到 stderr、进度与结果到 stdout。
- 验收标准：
  - `qrcd send <file>`、`qrcd send --text <文本>`、`qrcd receive` 可按 04_api §1 运行；缺参输出 usage + 退出码 2。
  - 退出码四档与 04_api §1.2 一致；错误输出到 stderr、结果到 stdout。
  - usage/帮助中文，参数名与默认值与 04_api §1 表完全一致。
  - 无 GUI 依赖，Linux/macOS/Windows 可交叉编译（命令见 02_architecture §5.1）。

## TASK-011: 端到端联调与 MVP 验收（纯光学闭环）

- 状态：已完成
- 优先级：P0
- 负责角色：Developer
- 关联需求：PRD F-01~F-06；02_architecture §5.2；04_api §1/§2
- 估算：1.5 天
- 依赖关系：TASK-010
- 描述：端到端联调纯光学闭环：`send`（file 与 text）→`receive --source file|stdin`（无摄像头环境）走通「分块→FEC→组帧→QR→拆帧→FEC→校验→落盘」，还原字节一致；补充集成/确定性用例。验证退出码、进度、校验输出。
- 验收标准：
  - 无摄像头环境下用 file/stdin 兜底完整走通闭环，还原结果 SHA-256 与发送端一致。
  - 文本直传（≤200 字节）走标准文本二维码，第三方通用扫码 App 可读。
  - 人为丢弃帧的丢块/乱序场景仍可还原（喷泉码兜底）。
  - `go test ./...` 全绿，`go vet ./...` 无告警。

## TASK-012: 同网检测与 WiFi 加速 internal/net（Phase 2）

- 状态：待办
- 优先级：P1
- 负责角色：Developer
- 关联需求：PRD F-07；02_architecture §3/§4.3；04_api §4
- 估算：2 天
- 依赖关系：TASK-010
- 描述：同网检测 + 临时 HTTP 服务 + 一次性 token（128bit crypto/rand，5 分钟过期）。握手码 `qrcd://transfer?t=<token>&addr=<ip:port>&sha256=<hex>`；`GET /v1/transfer/{token}` 下载载荷（响应头 X-QRCD-Name/Size/SHA256）；token 单次使用/失效/过期错误码 403/410/404；不可达自动回退纯光学。支持 --net auto/on/off、--addr。
- 验收标准：
  - 同网时握手码扫通后走 HTTP 直传，SHA-256 校验通过；吞吐显著高于纯光学。
  - token 单次使用、失效、过期错误码正确；跨网/直连失败自动回退纯光学。
  - `--net off` 强制纯光学；端口冲突自动换端口。

## TASK-013: 手机离线中转网页 web/（Phase 2）

- 状态：待办
- 优先级：P1
- 负责角色：Developer
- 关联需求：PRD F-08；02_architecture §3/§4.4；04_api §5；03_database §4
- 估算：3 天
- 依赖关系：TASK-010
- 描述：移动网页 PWA：getUserMedia 摄像头扫码接收→IndexedDB 暂存→重新编码 QR 流播放（Canvas），实现「收→暂存→发」离线摆渡；Service Worker 离线；空间配额（navigator.storage.estimate()）+ 手动清除。复用桌面端帧协议（能力对称）。
- 验收标准：
  - 离线可用（Service Worker 缓存 shell）。
  - 「收→暂存→发」完整闭环；暂存数据刷新/断电后仍可取。
  - 支持手动删除暂存；空间不足阻止接收并提示。
  - 收发能力对称，帧协议与桌面端一致。

## 进行中 (IN PROGRESS)

（暂无）

## 已完成 (DONE)

（暂无）

## 已阻塞 (BLOCKED)

（暂无）

---

## 任务编号规则

- 格式：TASK-XXX
- 编号递增，不复用
- 已完成任务保留记录，不删除
- 阻塞任务需关联 docs/07_decisions.md 中的决策记录
