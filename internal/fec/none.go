package fec

// noneCodecImpl 直通方案（blockCount=1 时短路，不经过 FEC）
type noneCodecImpl struct{}

// NewNoneCodec 创建 None Codec（直通，不编码）
func NewNoneCodec() Codec {
	return &noneCodecImpl{}
}

// Scheme 返回 "none"
func (c *noneCodecImpl) Scheme() string {
	return "none"
}

// NewEncoder 创建直通编码器
func (c *noneCodecImpl) NewEncoder(data []byte, blockSize int, redundancy float64) (Encoder, error) {
	return &noneEncoder{data: data}, nil
}

// NewDecoder 创建直通解码器
func (c *noneCodecImpl) NewDecoder(sourceSymbols int, messageLength int) Decoder {
	return &noneDecoder{
		received: make(map[uint32]bool),
	}
}

// noneEncoder 直通编码器：将数据原样返回为单个符号
type noneEncoder struct {
	data   []byte
	returned bool
}

// SourceSymbols 返回源符号数（空载荷为 0）
func (e *noneEncoder) SourceSymbols() int {
	if len(e.data) == 0 {
		return 0
	}
	return 1
}

// NextSymbol 返回数据本身作为唯一符号（id=0）；空载荷返回 nil
func (e *noneEncoder) NextSymbol() (uint32, []byte) {
	if len(e.data) == 0 {
		return 0, nil
	}
	if e.returned {
		return 0, nil
	}
	e.returned = true
	return 0, e.data
}

// noneDecoder 直通解码器
type noneDecoder struct {
	received map[uint32]bool
	done     bool
	data     []byte
}

// AddSymbol 接收符号，id=0 的符号即为完整数据
func (d *noneDecoder) AddSymbol(id uint32, symbol []byte) (bool, error) {
	if d.received[id] {
		return d.done, nil
	}
	d.received[id] = true
	d.data = symbol
	d.done = true
	return true, nil
}

// Decode 返回原始数据
func (d *noneDecoder) Decode() ([]byte, error) {
	if !d.done {
		return nil, ErrDecodeNotReady
	}
	return d.data, nil
}

// Received 返回已收到的不重复符号数
func (d *noneDecoder) Received() int {
	return len(d.received)
}