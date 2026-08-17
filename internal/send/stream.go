package send

import (
	"fmt"
	"math"

	"qrcd/internal/fec"
	"qrcd/internal/frame"
	"qrcd/internal/payload"
	"qrcd/internal/qrcode"
)

// metaReplayEvery 每发送 N 个数据帧重播一次元数据帧，
// 保证接收端中途加入或漏收元数据也能初始化会话。
const metaReplayEvery = 20

// StreamItem 一帧播放单元（元数据帧或数据帧），Bytes 为已序列化帧字节。
type StreamItem struct {
	IsMeta bool
	Seq    uint32
	Bytes  []byte
}

// Stream 发送帧流，逐帧产出（惰性生成，避免一次性生成全部帧占用内存）。
type Stream struct {
	load        *payload.Load
	transferID  [16]byte
	meta        *frame.MetaData
	codec       fec.Codec
	enc         fec.Encoder
	sourceK     int // FEC 源符号数（= blockCount）
	totalData   int // 计划发送的数据帧总数（含冗余）
	dataSent    int // 已发送的数据帧数
	pendingMeta bool
	done        bool
}

// Meta 返回本次传输的元数据。
func (s *Stream) Meta() *frame.MetaData { return s.meta }

// TransferID 返回本次传输会话 ID。
func (s *Stream) TransferID() [16]byte { return s.transferID }

// TotalData 返回计划发送的数据帧总数（含冗余）。
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

// BuildStream 构建发送帧流（不渲染 QR）。
// 自动适配块大小：保证单个编码符号 ≤ 单帧 QR 容量（默认 version 20 扣除帧头/CRC）。
func BuildStream(load *payload.Load, opts Options) (*Stream, error) {
	if load == nil || len(load.Data) == 0 {
		return nil, fmt.Errorf("%w: 载荷为空，无需发送", ErrUsage)
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

	// 自适应分块：符号大小 = ceil(数据长度 / 块数)，须 ≤ 单帧载荷上限
	blockSize := opts.BlockSize
	blockCount := payload.BlockCount(load.Data, blockSize)
	if blockCount < 1 {
		blockCount = 1
	}
	symbolSize := (len(load.Data) + blockCount - 1) / blockCount
	for symbolSize > maxPayload && blockCount < len(load.Data) {
		blockCount++
		symbolSize = (len(load.Data) + blockCount - 1) / blockCount
	}
	if symbolSize > maxPayload {
		return nil, fmt.Errorf("%w: 数据 %d 字节无法适配 QR version %d（EC %s）单帧容量 %d 字节，请增大 --version 或减小 --block-size",
			ErrUsage, len(load.Data), opts.Version, opts.ECC, maxPayload)
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
		if sourceK > 8192 {
			return nil, fmt.Errorf("%w: 数据 %d 字节超过 Raptor 单会话上限（K≤8192，约 %d 字节），请拆分数据或减小 --block-size",
				ErrUsage, len(load.Data), 8192*maxPayload)
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
		Name:        load.Name,
		Size:        int64(len(load.Data)),
		BlockSize:   blockSize,
		BlockCount:  blockCount,
		HashAlgo:    "sha256",
		Hash:        payload.SHA256Hex(load.Data),
		FEC:         scheme,
		Redundancy:  opts.Redundancy,
		PayloadType: load.PayloadType,
		MimeType:    load.MimeType,
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
