package frame

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
)

// 帧协议常量
const (
	// MagicBytes 帧标识 "QRCD"
	MagicBytes = "QRCD"
	// HeaderSize 帧头固定 32 字节
	HeaderSize = 32
	// CRC32Size CRC32 校验 4 字节
	CRC32Size = 4
	// Version 当前协议版本
	Version byte = 0x01

	// TypeMeta 元数据帧
	TypeMeta byte = 0x01
	// TypeData 数据帧
	TypeData byte = 0x02

	// FlagPayloadTypeText 载荷类型：文本
	FlagPayloadTypeText uint16 = 0
	// FlagPayloadTypeFile 载荷类型：文件
	FlagPayloadTypeFile uint16 = 0x0001
	// FlagFEC 喷泉码启用
	FlagFEC uint16 = 0x0002
)

// 错误定义
var (
	ErrInvalidMagic   = fmt.Errorf("frame: magic 不匹配，期望 %q", MagicBytes)
	ErrInvalidVersion = fmt.Errorf("frame: 不支持的协议版本")
	ErrCRC32Mismatch  = fmt.Errorf("frame: CRC32 校验失败")
	ErrInvalidLength  = fmt.Errorf("frame: payload 长度与实际数据不符")
	ErrDuplicateSeq   = fmt.Errorf("frame: 重复的 seq")
)

// MetaData 元数据帧 JSON 载荷，字段与 docs/04_api.md §2.3 一致
type MetaData struct {
	Name        string  `json:"name"`
	Size        int64   `json:"size"`
	BlockSize   int     `json:"blockSize"`
	BlockCount  int     `json:"blockCount"`
	HashAlgo    string  `json:"hashAlgo"`
	Hash        string  `json:"hash"`
	FEC         string  `json:"fec"`
	Redundancy  float64 `json:"redundancy"`
	PayloadType string  `json:"payloadType"`
	MimeType    string  `json:"mimeType"`
	// TotalSymbols 发送端计划发送的编码符号总数（含冗余，不含元数据重播帧）。
	// 接收端用作进度基准，使"已收/总数"与发送端、手机端的计数对齐。
	TotalSymbols int `json:"totalSymbols,omitempty"`
}

// Header 帧头结构（32 字节）
type Header struct {
	Magic      [4]byte // "QRCD"
	Version    byte    // 协议版本 0x01
	Type       byte    // 0x01 元数据帧 / 0x02 数据帧
	Flags      uint16  // bit0 载荷类型 / bit1 FEC
	TransferID [16]byte // 会话 ID
	Seq        uint32  // 帧序号
	Len        uint32  // payload 长度
}

// Frame 完整帧结构
type Frame struct {
	Header  Header
	Payload []byte
	CRC32   uint32
}

// NewTransferID 生成随机 16 字节 transfer_id
func NewTransferID() ([16]byte, error) {
	var id [16]byte
	_, err := rand.Read(id[:])
	return id, err
}

// MustNewTransferID 生成随机 transfer_id，失败时 panic
func MustNewTransferID() [16]byte {
	id, err := NewTransferID()
	if err != nil {
		panic(fmt.Sprintf("frame: 生成 transfer_id 失败: %v", err))
	}
	return id
}

// SetFlags 根据参数设置 flags 字段
func SetFlags(payloadType string, fec bool) uint16 {
	var flags uint16
	if payloadType == "file" {
		flags |= FlagPayloadTypeFile
	}
	if fec {
		flags |= FlagFEC
	}
	return flags
}

// CRC32 计算 IEEE CRC32（大端）覆盖 header + payload
var ieeeTable = crc32.MakeTable(crc32.IEEE)

func computeCRC32(headerBytes, payload []byte) uint32 {
	h := crc32.New(ieeeTable)
	h.Write(headerBytes)
	h.Write(payload)
	return h.Sum32()
}

// MarshalHeader 将 Header 序列化为 32 字节大端
func (h *Header) MarshalHeader() []byte {
	buf := make([]byte, HeaderSize)
	copy(buf[0:4], h.Magic[:])
	buf[4] = h.Version
	buf[5] = h.Type
	binary.BigEndian.PutUint16(buf[6:8], h.Flags)
	copy(buf[8:24], h.TransferID[:])
	binary.BigEndian.PutUint32(buf[24:28], h.Seq)
	binary.BigEndian.PutUint32(buf[28:32], h.Len)
	return buf
}

// UnmarshalHeader 从 32 字节大端字节切片解析 Header
func UnmarshalHeader(data []byte) (*Header, error) {
	if len(data) < HeaderSize {
		return nil, fmt.Errorf("frame: 数据不足 %d 字节，无法解析帧头", HeaderSize)
	}
	h := &Header{}
	copy(h.Magic[:], data[0:4])
	h.Version = data[4]
	h.Type = data[5]
	h.Flags = binary.BigEndian.Uint16(data[6:8])
	copy(h.TransferID[:], data[8:24])
	h.Seq = binary.BigEndian.Uint32(data[24:28])
	h.Len = binary.BigEndian.Uint32(data[28:32])
	return h, nil
}

// Validate 校验帧头合法性
func (h *Header) Validate() error {
	if string(h.Magic[:]) != MagicBytes {
		return ErrInvalidMagic
	}
	if h.Version != Version {
		return ErrInvalidVersion
	}
	return nil
}

// MarshalMetaJSON 将 MetaData 序列化为 JSON 字节（单行、无 BOM）
func MarshalMetaJSON(m *MetaData) ([]byte, error) {
	return json.Marshal(m)
}

// UnmarshalMetaJSON 从 JSON 字节解析 MetaData
func UnmarshalMetaJSON(data []byte) (*MetaData, error) {
	m := &MetaData{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("frame: 元数据 JSON 解析失败: %w", err)
	}
	return m, nil
}