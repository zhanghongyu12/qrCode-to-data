package qrcode

import (
	"fmt"

	skip2 "github.com/skip2/go-qrcode"
)

// 常量
const (
	// DefaultVersion 默认 QR 版本上限（载荷不足自动降级）
	DefaultVersion = 20
	// MinVersion 最小 QR 版本
	MinVersion = 1
	// MaxVersion 最大 QR 版本
	MaxVersion = 40
	// DefaultECC 默认纠错级别（配合外层喷泉码用 L 最大化单帧载荷）
	DefaultECC = "L"
	// ShortTextLimit 短文本直传字节上限（≤200 直接编码标准文本二维码）
	ShortTextLimit = 200
)

// Options 生成选项
type Options struct {
	// Version QR 版本上限（1~40），0 表示使用默认值 20
	Version int
	// ECC 纠错级别 L/M/Q/H，空表示默认 L
	ECC string
}

// DefaultOptions 返回默认生成选项
func DefaultOptions() Options {
	return Options{Version: DefaultVersion, ECC: DefaultECC}
}

// Code 表示已生成的二维码
type Code struct {
	// Version 实际采用的 QR 版本
	Version int
	// ECC 纠错级别（L/M/Q/H）
	ECC string
	// Bitmap 模块位图（[row][col]，true=黑），已含 ≥4 模块静区
	Bitmap [][]bool
	// content 原始内容
	content string
}

// Validate 校验 version/ecc 参数范围，返回规范化后的 Options
func Validate(opts Options) (Options, error) {
	v := opts.Version
	if v == 0 {
		v = DefaultVersion
	}
	if v < MinVersion || v > MaxVersion {
		return opts, fmt.Errorf("qrcode: version %d 超出范围 [%d, %d]", v, MinVersion, MaxVersion)
	}
	ecc := opts.ECC
	if ecc == "" {
		ecc = DefaultECC
	}
	switch ecc {
	case "L", "M", "Q", "H":
	default:
		return opts, fmt.Errorf("qrcode: 非法纠错级别 %q，支持 L/M/Q/H", ecc)
	}
	return Options{Version: v, ECC: ecc}, nil
}

// eccLevel 将 ECC 字符串映射为 skip2 纠错级别
// L→Low(7%), M→Medium(15%), Q→High(25%), H→Highest(30%)
func eccLevel(ecc string) skip2.RecoveryLevel {
	switch ecc {
	case "M":
		return skip2.Medium
	case "Q":
		return skip2.High
	case "H":
		return skip2.Highest
	default:
		return skip2.Low
	}
}

// Encode 将 data 编码为二维码。
// version 为上限（1~40）：载荷不足自动降级到能容纳的最小版本；
// 超出上限版本容量时返回明确错误，不产生截断。
func Encode(data []byte, opts Options) (*Code, error) {
	norm, err := Validate(opts)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("qrcode: 空数据无法编码")
	}

	content := string(data)
	level := eccLevel(norm.ECC)

	// 先尝试自动选择能容纳数据的最小版本
	q, err := skip2.New(content, level)
	if err != nil {
		return nil, fmt.Errorf("qrcode: 生成二维码失败: %w", err)
	}
	if q.VersionNumber <= norm.Version {
		return fromSkip2(q, norm), nil
	}

	// 自动选择的最小版本超出上限：尝试强制使用上限版本
	q, err = skip2.NewWithForcedVersion(content, norm.Version, level)
	if err != nil {
		return nil, fmt.Errorf("qrcode: 数据 %d 字节超出 version %d 容量（EC %s），请增大 version 或减小分块", len(data), norm.Version, norm.ECC)
	}
	return fromSkip2(q, norm), nil
}

// fromSkip2 将 skip2 的 QRCode 转换为 Code
func fromSkip2(q *skip2.QRCode, opts Options) *Code {
	return &Code{
		Version: q.VersionNumber,
		ECC:     opts.ECC,
		Bitmap:  q.Bitmap(),
		content: q.Content,
	}
}

// EncodeText 将短文本（≤200 字节）直接编码为标准文本二维码。
// 超出上限返回错误，提示应走帧流路径。
func EncodeText(text string) (*Code, error) {
	if len(text) > ShortTextLimit {
		return nil, fmt.Errorf("qrcode: 文本 %d 字节超过短文本上限 %d 字节，应走帧流路径", len(text), ShortTextLimit)
	}
	return Encode([]byte(text), DefaultOptions())
}
