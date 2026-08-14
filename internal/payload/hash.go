package payload

import (
	"crypto/sha256"
	"encoding/hex"
	"hash/crc32"
)

// SHA256Hex 返回 data 的 SHA-256 十六进制（小写，64 字符）
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// CRC32 返回 data 的 IEEE CRC32
func CRC32(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// VerifySHA256 校验 data 的 SHA-256 是否与 expectedHex（十六进制）一致。
// expectedHex 为空时视为无需校验，返回 true。
func VerifySHA256(data []byte, expectedHex string) bool {
	if expectedHex == "" {
		return true
	}
	return SHA256Hex(data) == expectedHex
}
