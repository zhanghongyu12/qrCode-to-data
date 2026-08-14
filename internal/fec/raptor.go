package fec

import (
	fountain "github.com/google/gofountain"
)

// raptorCodecImpl Raptor 码实现（RFC 5053）
type raptorCodecImpl struct {
	sourceSymbols int
}

// NewRaptorCodec 创建 Raptor Codec
// sourceSymbols: 源符号数，须在 [4, 8192] 范围内
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

	// 创建 gofountain Raptor codec，alignmentSize=4
	fc := fountain.NewRaptorCodec(sourceSymbols, 4)

	return &raptorEncoderImpl{
		data:          data,
		sourceSymbols: sourceSymbols,
		nextID:        0,
		fc:            fc,
	}, nil
}

// NewDecoder 创建 Raptor 解码器
// sourceSymbols: 源符号数, messageLength: 原始数据长度
func (c *raptorCodecImpl) NewDecoder(sourceSymbols int, messageLength int) Decoder {
	fc := fountain.NewRaptorCodec(sourceSymbols, 4)
	return &raptorDecoderImpl{
		messageLength:  messageLength,
		sourceSymbols: sourceSymbols,
		received:      make(map[uint32]bool),
		fc:            fc,
		decoder:       fc.NewDecoder(messageLength),
	}
}

// raptorEncoderImpl Raptor 编码器实现
type raptorEncoderImpl struct {
	data          []byte
	sourceSymbols int
	nextID        uint32
	fc            fountain.Codec
}

// SourceSymbols 返回源符号数量
func (e *raptorEncoderImpl) SourceSymbols() int {
	return e.sourceSymbols
}

// NextSymbol 返回下一个编码符号
// 每次调用创建 data 副本，因为 EncodeLTBlocks 会破坏性修改输入
func (e *raptorEncoderImpl) NextSymbol() (uint32, []byte) {
	id := e.nextID
	e.nextID++

	// 复制数据，因为 EncodeLTBlocks 是破坏性的
	dataCopy := make([]byte, len(e.data))
	copy(dataCopy, e.data)

	blocks := fountain.EncodeLTBlocks(dataCopy, []int64{int64(id)}, e.fc)
	if len(blocks) == 0 {
		return id, nil
	}
	return id, blocks[0].Data
}

// raptorDecoderImpl Raptor 解码器实现
// 延迟创建 gofountain decoder，需要从收到的符号中推断 messageLength
type raptorDecoderImpl struct {
	messageLength  int
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