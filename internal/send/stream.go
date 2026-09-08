package send

import (
	"fmt"
	"math"

	"qrcd/internal/fec"
	"qrcd/internal/frame"
	"qrcd/internal/payload"
	"qrcd/internal/qrcode"
)

// metaReplayEvery 每播放 N 个数据帧重播一次元数据帧，
// 保证还原端中途加入或漏收元数据也能初始化会话。
const metaReplayEvery = 20

// maxSourceK 单会话 Raptor 源块数上限（DEC-012）。
// 取 1024 而非 RFC 5053 的理论上限 8192：
//   - gofountain 在 K=8192 时内部矩阵求解越界 panic（实测）；
//   - 批量编码总代价 O(K²)，K=1024 编码约 110ms / 解码约 100ms，实时播放不卡顿；
//   - 超过此上限的载荷由 BuildSessionStreams 自动拆分为多个独立交换会话。
const maxSourceK = 1024

// StreamItem 一帧播放单元（元数据帧或数据帧），Bytes 为已序列化帧字节。
type StreamItem struct {
	IsMeta bool
	Seq    uint32
	Bytes  []byte
}

// Stream 播放帧流，逐帧产出（惰性生成，避免一次性生成全部帧占用内存）。
type Stream struct {
	load        *payload.Load
	transferID  [16]byte
	meta        *frame.MetaData
	codec       fec.Codec
	enc         fec.Encoder
	sourceK     int // FEC 源符号数（= blockCount）
	totalData   int // 计划播放的数据帧总数（含冗余）
	dataSent    int // 已播放的数据帧数
	pendingMeta bool
	done        bool
}

// Meta 返回本次交换的元数据。
func (s *Stream) Meta() *frame.MetaData { return s.meta }

// TransferID 返回本次交换会话 ID。
func (s *Stream) TransferID() [16]byte { return s.transferID }

// TotalData 返回计划播放的数据帧总数（含冗余）。
func (s *Stream) TotalData() int { return s.totalData }

// Next 返回下一帧；ok=false 表示流结束。
func (s *Stream) Next() (item StreamItem, ok bool) {
	if s.done {
		return StreamItem{}, false
	}

	// 首帧与周期性重播元数据帧
	if s.pendingMeta {
		s.pendingMeta = false
		mf, err := frame.BuildMetaFrame(s.transferID, s.meta)
		if err != nil {
			s.done = true
			return StreamItem{}, false
		}
		return StreamItem{IsMeta: true, Seq: 0, Bytes: frame.MarshalFrame(mf)}, true
	}

	if s.dataSent >= s.totalData {
		s.done = true
		return StreamItem{}, false
	}

	id, sym := s.enc.NextSymbol()
	if sym == nil {
		// none 编码器首次之后返回 nil；防御性结束
		s.dataSent = s.totalData
		s.done = true
		return StreamItem{}, false
	}

	fecEnabled := s.codec.Scheme() != "none"
	df := frame.BuildDataFrame(s.transferID, id, sym, s.meta.PayloadType, fecEnabled)
	s.dataSent++

	// 每 metaReplayEvery 个数据帧后重播一次元数据帧（不足一周期则不再重播）
	if s.dataSent%metaReplayEvery == 0 && s.dataSent < s.totalData {
		s.pendingMeta = true
	}

	return StreamItem{IsMeta: false, Seq: id, Bytes: frame.MarshalFrame(df)}, true
}

// BuildStream 构建播放帧流（不渲染 QR）。
// 自动适配块大小：保证单个编码符号 ≤ 单帧 QR 容量（默认 version 20 扣除帧头/CRC）。
func BuildStream(load *payload.Load, opts Options) (*Stream, error) {
	if load == nil || len(load.Data) == 0 {
		return nil, fmt.Errorf("%w: 载荷为空，无需输出", ErrUsage)
	}
	if opts.BlockSize < 1 {
		return nil, fmt.Errorf("%w: --block-size 必须 ≥ 1", ErrUsage)
	}

	maxPayload, err := maxFramePayload(opts.Version, opts.ECC)
	if err != nil {
		return nil, err
	}
	if maxPayload < 1 {
		return nil, fmt.Errorf("%w: QR version %d（EC %s）容量过小，无法承载帧头", ErrUsage, opts.Version, opts.ECC)
	}

	// 数据符号字节上限：默认按 Version 容量自适应；MaxSymbol>0 时取较小者，
	// 使单帧 QR 更稀疏（更低版本）以提升手机摄像头扫码识别率（见 DEC-008）。
	dataMax := maxPayload
	if opts.MaxSymbol > 0 && opts.MaxSymbol < dataMax {
		dataMax = opts.MaxSymbol
	}

	// 自适应分块：符号大小 = ceil(数据长度 / 块数)，须 ≤ 单帧载荷上限
	blockSize := opts.BlockSize
	blockCount := payload.BlockCount(load.Data, blockSize)
	if blockCount < 1 {
		blockCount = 1
	}
	symbolSize := (len(load.Data) + blockCount - 1) / blockCount
	for symbolSize > dataMax && blockCount < len(load.Data) {
		blockCount++
		symbolSize = (len(load.Data) + blockCount - 1) / blockCount
	}
	if symbolSize > dataMax {
		return nil, fmt.Errorf("%w: 数据 %d 字节无法适配 QR version %d（EC %s）单帧容量 %d 字节（MaxSymbol 限制 %d），请增大 --version 或减小 --max-symbol",
			ErrUsage, len(load.Data), opts.Version, opts.ECC, maxPayload, dataMax)
	}

	// FEC 方案选择：
	//  - 单块（≤ 单帧容量）→ none 直通，不组 FEC
	//  - 多块 → Raptor（K 提升至 ≥4），规避 LT 在部分 K 值上的解码缺陷
	scheme := "none"
	sourceK := blockCount
	if blockCount > 1 {
		scheme = "raptor"
		if sourceK < 4 {
			// Raptor 要求 K∈[4,8192]；K<4 时提升块数（符号更小，仍满足容量）
			sourceK = 4
			symbolSize = (len(load.Data) + sourceK - 1) / sourceK
			if symbolSize < 1 {
				symbolSize = 1
			}
		}
		if sourceK > maxSourceK {
			return nil, fmt.Errorf("%w: 数据 %d 字节超过 Raptor 单会话安全上限（K≤%d，约 %d 字节），将自动多会话分片；请直接调用 BuildSessionStreams 或 Send",
				ErrUsage, len(load.Data), maxSourceK, maxSourceK*maxPayload)
		}
		if symbolSize > maxPayload {
			return nil, fmt.Errorf("%w: 数据 %d 字节无法适配 QR version %d（EC %s）单帧容量 %d 字节，请增大 --version 或减小 --block-size",
				ErrUsage, len(load.Data), opts.Version, opts.ECC, maxPayload)
		}
	}

	// Raptor 数据补齐到 K*symbolSize：gofountain 对源码块不等长（padding）解码结果错误，
	// 补齐为整数倍后所有符号等长，可稳定还原；none 直通保持原文不变。
	encodeData := load.Data
	if scheme != "none" {
		paddedLen := sourceK * symbolSize
		if paddedLen != len(load.Data) {
			encodeData = make([]byte, paddedLen)
			copy(encodeData, load.Data)
		}
	}

	codec, err := fec.NewCodec(scheme, sourceK)
	if err != nil {
		return nil, fmt.Errorf("%w: 创建 FEC 编解码器失败: %v", ErrUsage, err)
	}
	enc, err := codec.NewEncoder(encodeData, blockSize, opts.Redundancy)
	if err != nil {
		return nil, fmt.Errorf("send: 创建编码器失败: %w", err)
	}

	totalData := enc.SourceSymbols()
	if totalData < 1 {
		totalData = 1
	}
	if scheme != "none" {
		extra := int(math.Ceil(opts.Redundancy * float64(enc.SourceSymbols())))
		totalData = enc.SourceSymbols() + extra
	}

	meta := &frame.MetaData{
		Name:         load.Name,
		Size:         int64(len(load.Data)),
		BlockSize:    blockSize,
		BlockCount:   sourceK,
		HashAlgo:     "sha256",
		Hash:         payload.SHA256Hex(load.Data),
		FEC:          scheme,
		Redundancy:   opts.Redundancy,
		PayloadType:  load.PayloadType,
		MimeType:     load.MimeType,
		TotalSymbols: totalData,
	}

	return &Stream{
		load:        load,
		transferID:  frame.MustNewTransferID(),
		meta:        meta,
		codec:       codec,
		enc:         enc,
		sourceK:     sourceK,
		totalData:   totalData,
		pendingMeta: true,
	}, nil
}

// BuildSessionStreams 构建多会话分片帧流列表（DEC-012）。
// 大文件（超过单会话 Raptor 安全上限 maxSourceK）自动拆为 N 个独立交换会话：
//   - 各分片独立 transfer_id、独立 QR 流、独立 name/size/hash（分片级校验）
//   - 分片大小 partSize = maxSourceK × symbolSize，partTotal = ceil(原始总长 / partSize)
//   - 每分片分块大小自动取 ceil(分片长 / maxSourceK)，保证单会话源块数 ≤ maxSourceK
//   - 所有分片均携带 overallHash 作为整体关联键；仅 partIndex=0 额外携带
//     overallName/overallSize（整体文件信息，还原端拼接落盘用）
//
// 数据不足单会话容量时返回单流（行为与 BuildStream 完全一致，向后兼容）。
func BuildSessionStreams(load *payload.Load, opts Options) ([]*Stream, error) {
	if load == nil || len(load.Data) == 0 {
		return nil, fmt.Errorf("%w: 载荷为空，无需输出", ErrUsage)
	}

	maxPayload, err := maxFramePayload(opts.Version, opts.ECC)
	if err != nil {
		return nil, err
	}
	// 单符号字节上限：与 BuildStream 保持一致（MaxSymbol 优先，提升手机扫码识别率）
	dataMax := maxPayload
	if opts.MaxSymbol > 0 && opts.MaxSymbol < dataMax {
		dataMax = opts.MaxSymbol
	}
	if dataMax < 1 {
		return nil, fmt.Errorf("%w: QR version %d（EC %s）容量过小，无法承载帧头", ErrUsage, opts.Version, opts.ECC)
	}

	partSize := maxSourceK * dataMax
	partTotal := (len(load.Data) + partSize - 1) / partSize
	if partTotal <= 1 {
		st, err := BuildStream(load, opts)
		if err != nil {
			return nil, err
		}
		return []*Stream{st}, nil
	}

	overallHash := payload.SHA256Hex(load.Data)
	streams := make([]*Stream, 0, partTotal)
	for i := 0; i < partTotal; i++ {
		start := i * partSize
		end := start + partSize
		if end > len(load.Data) {
			end = len(load.Data)
		}
		partData := load.Data[start:end]

		po := opts
		// 每分片保证源块数 ≤ maxSourceK：分块大小 = ceil(分片长 / maxSourceK)。
		// 满片时 K=1024 恰在上限；余量分片 K 更小（≤1 时走 none 直通）。
		po.BlockSize = (len(partData) + maxSourceK - 1) / maxSourceK
		if po.BlockSize < 1 {
			po.BlockSize = 1
		}
		if po.BlockSize > dataMax {
			po.BlockSize = dataMax
		}

		partLoad := &payload.Load{
			Data:        partData,
			Name:        fmt.Sprintf("%s.part%d", load.Name, i),
			PayloadType: load.PayloadType,
			MimeType:    load.MimeType,
		}
		st, err := BuildStream(partLoad, po)
		if err != nil {
			return nil, fmt.Errorf("send: 构建分片 %d/%d 失败: %w", i+1, partTotal, err)
		}
		// 注入分片元数据（DEC-012）；所有分片携带 overallHash 作为整体关联键，
		// 还原端用同一 overallHash 把各分片归入同一交换。仅 part 0 额外携带整体
		// name/size（整体文件信息，拼接落盘用），避免冗余携带大文件名。
		st.meta.PartIndex = i
		st.meta.PartTotal = partTotal
		st.meta.OverallHash = overallHash
		if i == 0 {
			st.meta.OverallName = load.Name
			st.meta.OverallSize = int64(len(load.Data))
		}
		streams = append(streams, st)
	}
	return streams, nil
}

// maxFramePayload 计算单帧 payload（不含 32B 头与 4B CRC）在给定 version/ecc 下的字节上限。
func maxFramePayload(version int, ecc string) (int, error) {
	capacity, err := qrcodeMaxCapacity(version, ecc)
	if err != nil {
		return 0, err
	}
	return capacity - frame.HeaderSize - frame.CRC32Size, nil
}

var capacityCache = map[[2]int]int{}

// qrcodeMaxCapacity 通过二分法实测 QR version+ecc 的最大字节容量（8-bit 模式）。
// skip2/go-qrcode 未导出容量表，采用实证测量，保证与实际编码一致。
func qrcodeMaxCapacity(version int, ecc string) (int, error) {
	key := [2]int{version, 0}
	switch ecc {
	case "L":
		key[1] = 1
	case "M":
		key[1] = 2
	case "Q":
		key[1] = 3
	case "H":
		key[1] = 4
	default:
		return 0, fmt.Errorf("%w: 非法纠错级别 %q，支持 L/M/Q/H", ErrUsage, ecc)
	}
	if v, ok := capacityCache[key]; ok {
		return v, nil
	}

	// version 40-L 8-bit 容量约为 2953 字节，上界取 4096 足够
	lo, hi := 0, 4096
	for lo < hi {
		mid := (lo + hi + 1) / 2
		_, err := qrcode.Encode(make([]byte, mid), qrcode.Options{Version: version, ECC: ecc})
		if err == nil {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	capacityCache[key] = lo
	return lo, nil
}
