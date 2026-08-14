package fec

import (
	"fmt"
)

// Codec 创建编码器/解码器
type Codec interface {
	// Scheme 返回方案名：none / lt / raptor
	Scheme() string

	// NewEncoder 将源块 data 切为 blockSize 大小的源符号，返回编码器
	NewEncoder(data []byte, blockSize int, redundancy float64) (Encoder, error)

	// NewDecoder 创建解码器，sourceSymbols 为源符号数，messageLength 为原始数据长度
	NewDecoder(sourceSymbols int, messageLength int) Decoder
}

// Encoder 喷泉码编码器
type Encoder interface {
	// SourceSymbols 返回源符号数量
	SourceSymbols() int
	// NextSymbol 返回下一个编码符号（含符号 id），可无限产出冗余符号
	NextSymbol() (id uint32, symbol []byte)
}

// Decoder 喷泉码解码器
type Decoder interface {
	// AddSymbol 送入一个编码符号（可乱序、可重复）；done=true 表示已收够可还原
	AddSymbol(id uint32, symbol []byte) (done bool, err error)
	// Decode 还原源数据（须在 done=true 后调用）
	Decode() ([]byte, error)
	// Received 返回已收到的不重复符号数
	Received() int
}

// ErrDecodeNotReady 解码未就绪
var ErrDecodeNotReady = fmt.Errorf("fec: 符号不足，解码未就绪")

// NewCodec 根据参数创建合适的 Codec 实现
// scheme: "raptor" / "lt" / "none"
// sourceSymbols: 源符号数
func NewCodec(scheme string, sourceSymbols int) (Codec, error) {
	switch scheme {
	case "raptor":
		return NewRaptorCodec(sourceSymbols), nil
	case "lt":
		return NewLTCodec(sourceSymbols), nil
	case "none":
		return NewNoneCodec(), nil
	default:
		return nil, fmt.Errorf("fec: 不支持的方案 %q，支持 raptor/lt/none", scheme)
	}
}

// DefaultCodec 返回默认的 Raptor Codec
func DefaultCodec(sourceSymbols int) Codec {
	return NewRaptorCodec(sourceSymbols)
}