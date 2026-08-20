package fec

import (
	"math"

	fountain "github.com/google/gofountain"
)

// raptorCodecImpl Raptor 码实现（RFC 5053）
type raptorCodecImpl struct {
	sourceSymbols int
}

// NewRaptorCodec 创建 Raptor Codec
// sourceSymbols: 源符号数，须在 [4, 8192] 范围内。
// 注意：gofountain 在 K=8192 时内部矩阵求解会越界 panic，实际安全上限约 4096；
// 发送端经多会话分片（DEC-012）保证单会话 K ≤ 1024，本层仅按库能力放宽。
func NewRaptorCodec(sourceSymbols int) Codec {
	return &raptorCodecImpl{sourceSymbols: sourceSymbols}
}

// Scheme 返回 "raptor"
func (c *raptorCodecImpl) Scheme() string {
	return "raptor"
}

// NewEncoder 创建 Raptor 编码器
func (c *raptorCodecImpl) NewEncoder(data []byte, blockSize int, redundancy float64) (Encoder, error) {
	if len(data) == 0 {
		return &noneEncoder{data: data}, nil
	}

	// 计算源符号数
	sourceSymbols := c.sourceSymbols
	if sourceSymbols <= 0 {
		sourceSymbols = max(1, (len(data)+blockSize-1)/blockSize)
	}

	// 一次性预生成全部编码符号（含冗余）。gofountain 的 EncodeLTBlocks 每次调用
	// 都会重建 O(K²) 的中间符号块；逐符号调用总代价为 O(K³)（K=1024 实测约 95s），
	// 大文件完全不可用。批量传入全部符号 id，中间块只重建一次，总代价 O(K²)
	// （K=1024 实测约 110ms，K=4096 约 2s）。符号 id 从 0 递增，解码端按 id 匹配，
	// 输出与旧逐符号方式完全一致，仅速度快数百倍。
	total := sourceSymbols + int(math.Ceil(redundancy*float64(sourceSymbols)))
	if total < sourceSymbols {
		total = sourceSymbols
	}
	ids := make([]int64, total)
	for i := range ids {
		ids[i] = int64(i)
	}
	// EncodeLTBlocks 破坏性修改入参，传入副本
	src := make([]byte, len(data))
	copy(src, data)
	fc := fountain.NewRaptorCodec(sourceSymbols, 4)
	blocks := fountain.EncodeLTBlocks(src, ids, fc)

	// 另存原始数据副本：按需生成时 EncodeLTBlocks 仍会破坏性修改入参，
	// 且不能假设调用方不修改 data，须从干净副本重建中间块。
	encData := make([]byte, len(data))
	copy(encData, data)

	return &raptorEncoderImpl{
		sourceSymbols: sourceSymbols,
		data:          encData,
		fc:            fc,
		symbols:       blocks,
	}, nil
}

// NewDecoder 创建 Raptor 解码器
// sourceSymbols: 源符号数, messageLength: 原始数据长度
func (c *raptorCodecImpl) NewDecoder(sourceSymbols int, messageLength int) Decoder {
	fc := fountain.NewRaptorCodec(sourceSymbols, 4)
	return &raptorDecoderImpl{
		messageLength: messageLength,
		sourceSymbols: sourceSymbols,
		received:      make(map[uint32]bool),
		fc:            fc,
		decoder:       fc.NewDecoder(messageLength),
	}
}

// raptorEncoderImpl Raptor 编码器实现。
// 预生成 K+冗余 个编码符号（避免 O(K³) 逐符号代价），
// 耗尽后按需生成新符号（单次 O(K²)），保证可无限产出冗余符号。
type raptorEncoderImpl struct {
	sourceSymbols int
	data          []byte             // 原始数据副本（用于后续按需生成新符号）
	fc            fountain.Codec     // 喷泉码编解码器（用于后续按需生成新符号）
	symbols       []fountain.LTBlock // 预生成的首批符号
	nextID        uint32
}

// SourceSymbols 返回源符号数量
func (e *raptorEncoderImpl) SourceSymbols() int {
	return e.sourceSymbols
}

// NextSymbol 返回下一个编码符号（含符号 id），可无限产出冗余符号。
// 预生成符号耗尽后自动按需生成新符号，保证调用方始终能获得新符号。
func (e *raptorEncoderImpl) NextSymbol() (uint32, []byte) {
	if int(e.nextID) < len(e.symbols) {
		b := e.symbols[e.nextID]
		e.nextID++
		return uint32(b.BlockCode), b.Data
	}
	// 预生成符号耗尽：按需生成一个新符号。
	// EncodeLTBlocks 破坏性修改入参，传入副本。
	src := make([]byte, len(e.data))
	copy(src, e.data)
	ids := []int64{int64(e.nextID)}
	blocks := fountain.EncodeLTBlocks(src, ids, e.fc)
	if len(blocks) == 0 {
		return e.nextID, nil
	}
	b := blocks[0]
	e.nextID++
	return uint32(b.BlockCode), b.Data
}

// raptorDecoderImpl Raptor 解码器实现
// 延迟创建 gofountain decoder，需要从收到的符号中推断 messageLength
type raptorDecoderImpl struct {
	messageLength int
	sourceSymbols int
	received      map[uint32]bool
	fc            fountain.Codec
	decoder       fountain.Decoder
	done          bool
	decoded       []byte
}

// AddSymbol 送入一个编码符号
// 复用同一个 gofountain decoder 增量投喂，避免重复投喂已损坏的 block 数据
// （gofountain 的 AddBlocks 会原地 XOR 修改 block.Data）
func (d *raptorDecoderImpl) AddSymbol(id uint32, symbol []byte) (bool, error) {
	if d.received[id] {
		return d.done, nil
	}
	d.received[id] = true

	ready := d.decoder.AddBlocks([]fountain.LTBlock{{BlockCode: int64(id), Data: symbol}})
	if ready && !d.done {
		d.decoded = d.decoder.Decode()
		d.done = true
	}

	return d.done, nil
}

// Decode 还原源数据
func (d *raptorDecoderImpl) Decode() ([]byte, error) {
	if !d.done {
		return nil, ErrDecodeNotReady
	}
	if d.decoded == nil {
		return nil, ErrDecodeNotReady
	}
	return d.decoded, nil
}

// Received 返回已收到的不重复符号数
func (d *raptorDecoderImpl) Received() int {
	return len(d.received)
}
