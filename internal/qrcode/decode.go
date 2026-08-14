package qrcode

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"github.com/makiuchi-d/gozxing"
	gozxingqr "github.com/makiuchi-d/gozxing/qrcode"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// decode 从图像解码二维码，返回 gozxing Result
func decode(img image.Image, hints map[gozxing.DecodeHintType]interface{}) (*gozxing.Result, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil, fmt.Errorf("qrcode: 图像转位图失败: %w", err)
	}
	reader := gozxingqr.NewQRCodeReader()
	res, err := reader.Decode(bmp, hints)
	if err != nil {
		return nil, fmt.Errorf("qrcode: 二维码解码失败: %w", err)
	}
	return res, nil
}

// DecodeImage 从图像解码二维码内容（文本）。
func DecodeImage(img image.Image) (string, error) {
	res, err := decode(img, nil)
	if err != nil {
		return "", err
	}
	return res.GetText(), nil
}

// DecodeImageBytes 从图像解码二维码原始字节。
// 通过 CHARACTER_SET=ISO-8859-1 使 byte 模式 1:1 还原，适用于二进制帧载荷。
// 解码得到的文本 rune 范围 0~255，再经 ISO-8859-1 编码器还原为原始字节，
// 避免 Go 字符串 UTF-8 存储导致的高字节失真。
func DecodeImageBytes(img image.Image) ([]byte, error) {
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_CHARACTER_SET: charmap.ISO8859_1,
		gozxing.DecodeHintType_TRY_HARDER:    struct{}{},
	}
	res, err := decode(img, hints)
	if err != nil {
		return nil, err
	}
	enc := charmap.ISO8859_1.NewEncoder()
	out, _, err := transform.Bytes(enc, []byte(res.GetText()))
	if err != nil {
		return nil, fmt.Errorf("qrcode: 字节还原失败: %w", err)
	}
	return out, nil
}

// DecodePNG 从 PNG 字节解码二维码内容（文本）。
func DecodePNG(data []byte) (string, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("qrcode: PNG 解码失败: %w", err)
	}
	return DecodeImage(img)
}

// DecodeBitmap 从位图解码二维码内容（文本）。
func DecodeBitmap(bitmap [][]bool) (string, error) {
	code := &Code{Bitmap: bitmap}
	return DecodeImage(code.Image(4))
}
