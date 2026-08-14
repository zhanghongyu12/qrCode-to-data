package frame

import (
	"encoding/binary"
	"fmt"
)

// BuildMetaFrame 构建元数据帧（type=0x01）
// transferID: 会话 ID
// meta: 元数据结构体
func BuildMetaFrame(transferID [16]byte, meta *MetaData) (*Frame, error) {
	payload, err := MarshalMetaJSON(meta)
	if err != nil {
		return nil, fmt.Errorf("frame: 构建元数据帧失败: %w", err)
	}

	feck := meta.FEC != "none"
	flags := SetFlags(meta.PayloadType, feck)

	h := Header{
		Version:    Version,
		Type:       TypeMeta,
		Flags:      flags,
		TransferID: transferID,
		Seq:        0, // 元数据帧 seq 固定为 0
		Len:        uint32(len(payload)),
	}
	copy(h.Magic[:], MagicBytes)

	headerBytes := h.MarshalHeader()
	crc := computeCRC32(headerBytes, payload)

	return &Frame{
		Header:  h,
		Payload: payload,
		CRC32:   crc,
	}, nil
}

// BuildDataFrame 构建数据帧（type=0x02）
// transferID: 会话 ID
// seq: 符号 id
// data: 符号载荷字节
// payloadType: "text" 或 "file"
// fec: 是否启用 FEC
func BuildDataFrame(transferID [16]byte, seq uint32, data []byte, payloadType string, fec bool) *Frame {
	flags := SetFlags(payloadType, fec)

	h := Header{
		Version:    Version,
		Type:       TypeData,
		Flags:      flags,
		TransferID: transferID,
		Seq:        seq,
		Len:        uint32(len(data)),
	}
	copy(h.Magic[:], MagicBytes)

	headerBytes := h.MarshalHeader()
	crc := computeCRC32(headerBytes, data)

	return &Frame{
		Header:  h,
		Payload: data,
		CRC32:   crc,
	}
}

// MarshalFrame 将 Frame 序列化为完整字节切片
// 格式：[32B header][payload][4B CRC32 大端]
func MarshalFrame(f *Frame) []byte {
	headerBytes := f.Header.MarshalHeader()
	totalLen := HeaderSize + len(f.Payload) + CRC32Size
	buf := make([]byte, totalLen)
	copy(buf[0:HeaderSize], headerBytes)
	copy(buf[HeaderSize:HeaderSize+len(f.Payload)], f.Payload)
	binary.BigEndian.PutUint32(buf[HeaderSize+len(f.Payload):], f.CRC32)
	return buf
}

// UnmarshalFrame 从字节切片解析 Frame
// 校验 magic、version、CRC32，失败返回错误
func UnmarshalFrame(data []byte) (*Frame, error) {
	if len(data) < HeaderSize+CRC32Size {
		return nil, fmt.Errorf("frame: 数据长度 %d 不足，至少需要 %d 字节", len(data), HeaderSize+CRC32Size)
	}

	h, err := UnmarshalHeader(data[:HeaderSize])
	if err != nil {
		return nil, err
	}

	if err := h.Validate(); err != nil {
		return nil, err
	}

	payloadLen := int(h.Len)
	expectedTotal := HeaderSize + payloadLen + CRC32Size
	if len(data) != expectedTotal {
		return nil, fmt.Errorf("%w: 期望 %d 字节，实际 %d 字节", ErrInvalidLength, expectedTotal, len(data))
	}

	payload := make([]byte, payloadLen)
	copy(payload, data[HeaderSize:HeaderSize+payloadLen])

	expectedCRC := binary.BigEndian.Uint32(data[HeaderSize+payloadLen:])
	actualCRC := computeCRC32(data[:HeaderSize], payload)
	if expectedCRC != actualCRC {
		return nil, fmt.Errorf("%w: 期望 0x%08X，实际 0x%08X", ErrCRC32Mismatch, expectedCRC, actualCRC)
	}

	return &Frame{
		Header:  *h,
		Payload: payload,
		CRC32:   actualCRC,
	}, nil
}

// IsMeta 判断帧是否为元数据帧
func (f *Frame) IsMeta() bool {
	return f.Header.Type == TypeMeta
}

// IsData 判断帧是否为数据帧
func (f *Frame) IsData() bool {
	return f.Header.Type == TypeData
}

// ParseMeta 从元数据帧中解析 MetaData
func (f *Frame) ParseMeta() (*MetaData, error) {
	if !f.IsMeta() {
		return nil, fmt.Errorf("frame: 非元数据帧 (type=0x%02X)，无法解析 MetaData", f.Header.Type)
	}
	return UnmarshalMetaJSON(f.Payload)
}