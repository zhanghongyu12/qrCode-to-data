package fec

import (
	"math/rand"

	fountain "github.com/google/gofountain"
)

// ltCodecImpl LT 码实现
type ltCodecImpl struct {
	sourceSymbols int
}

// NewLTCodec 创建 LT Codec
// sourceSymbols: 源符号数
func NewLTCodec(sourceSymbols int) Codec {
	return &ltCodecImpl{sourceSymbols: sourceSymbols}
}

// Scheme 返回 "lt"
func (c *ltCodecImpl) Scheme() string {
	return "lt"
}

// NewEncoder 创建 LT 编码器
func (c *ltCodecImpl) NewEncoder(data []byte, blockSize int, redundancy float64) (Encoder, error) {
	if len(data) == 0 {
		return &noneEncoder{data: data}, nil
	}

	sourceSymbols := c.sourceSymbols
	if sourceSymbols <= 0 {
		sourceSymbols = max(1, (len(data)+blockSize-1)/blockSize)
	}

	// 使用 soliton distribution 作为 degree CDF
	degreeCDF := solitonCDF(sourceSymbols)
	fc := fountain.NewLubyCodec(sourceSymbols, rand.New(rand.NewSource(0)), degreeCDF)

	return &ltEncoderImpl{
		data:          data,
		sourceSymbols: sourceSymbols,
		nextID:        0,
		fc:            fc,
	}, nil
}

// NewDecoder 创建 LT 解码器
// sourceSymbols: 源符号数, messageLength: 原始数据长度
func (c *ltCodecImpl) NewDecoder(sourceSymbols int, messageLength int) Decoder {
	degreeCDF := solitonCDF(sourceSymbols)
	fc := fountain.NewLubyCodec(sourceSymbols, rand.New(rand.NewSource(0)), degreeCDF)
	return &ltDecoderImpl{
		messageLength:  messageLength,
		sourceSymbols: sourceSymbols,
		received:      make(map[uint32]bool),
		fc:            fc,
		decoder:       fc.NewDecoder(messageLength),
	}
}

// solitonCDF 返回 soliton distribution 的 CDF
func solitonCDF(n int) []float64 {
	cdf := make([]float64, n+1)
	cdf[1] = 1.0 / float64(n)
	for i := 2; i < len(cdf); i++ {
		cdf[i] = cdf[i-1] + (1.0 / (float64(i) * float64(i-1)))
	}
	return cdf
}

// ltEncoderImpl LT 编码器实现
type ltEncoderImpl struct {
	data          []byte
	sourceSymbols int
	nextID        uint32
	fc            fountain.Codec
}

// SourceSymbols 返回源符号数量
func (e *ltEncoderImpl) SourceSymbols() int {
	return e.sourceSymbols
}

// NextSymbol 返回下一个编码符号
func (e *ltEncoderImpl) NextSymbol() (uint32, []byte) {
	id := e.nextID
	e.nextID++

	dataCopy := make([]byte, len(e.data))
	copy(dataCopy, e.data)

	blocks := fountain.EncodeLTBlocks(dataCopy, []int64{int64(id)}, e.fc)
	if len(blocks) == 0 {
		return id, nil
	}
	return id, blocks[0].Data
}

// ltDecoderImpl LT 解码器实现
type ltDecoderImpl struct {
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
func (d *ltDecoderImpl) AddSymbol(id uint32, symbol []byte) (bool, error) {
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
func (d *ltDecoderImpl) Decode() ([]byte, error) {
	if !d.done {
		return nil, ErrDecodeNotReady
	}
	if d.decoded == nil {
		return nil, ErrDecodeNotReady
	}
	return d.decoded, nil
}

// Received 返回已收到的不重复符号数
func (d *ltDecoderImpl) Received() int {
	return len(d.received)
}