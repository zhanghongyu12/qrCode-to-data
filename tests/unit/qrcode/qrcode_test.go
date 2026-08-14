package qrcode_test

import (
	"bytes"
	"strings"
	"testing"

	"qrcd/internal/qrcode"
)

// TestEncodeTextRoundtrip 生成文本二维码 → gozxing 解码回原文（闭环）
func TestEncodeTextRoundtrip(t *testing.T) {
	text := "Hello QR Code! 二维码闭环测试 #123"
	code, err := qrcode.Encode([]byte(text), qrcode.DefaultOptions())
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}
	decoded, err := qrcode.DecodeImage(code.Image(8))
	if err != nil {
		t.Fatalf("DecodeImage 失败: %v", err)
	}
	if decoded != text {
		t.Errorf("闭环解码结果不匹配\n原始: %q\n解码: %q", text, decoded)
	}
}

// TestEncodeTextShortRoundtrip 短文本直传编码（≤200 字节）
func TestEncodeTextShortRoundtrip(t *testing.T) {
	text := "短文本直传测试"
	code, err := qrcode.EncodeText(text)
	if err != nil {
		t.Fatalf("EncodeText 失败: %v", err)
	}
	decoded, err := qrcode.DecodeImage(code.Image(8))
	if err != nil {
		t.Fatalf("DecodeImage 失败: %v", err)
	}
	if decoded != text {
		t.Errorf("闭环解码结果不匹配\n原始: %q\n解码: %q", text, decoded)
	}
}

// TestEncodeBytesRoundtrip 二进制载荷（帧字节）→ gozxing 解码回原文（闭环）
func TestEncodeBytesRoundtrip(t *testing.T) {
	// 模拟 QRCD 帧：32B 头 + payload + 4B CRC，含 0x00~0xFF 混合字节
	payload := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x80, 0x7F}
	data := make([]byte, 0, 32+len(payload)+4)
	data = append(data, []byte("QRCD")...)
	data = append(data, 0x01, 0x02, 0x00, 0x00)
	data = append(data, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x00)
	data = append(data, 0x00, 0x00, 0x00, 0x2A) // seq=42
	data = append(data, 0x00, 0x00, 0x00, byte(len(payload)))
	data = append(data, payload...)
	data = append(data, 0x12, 0x34, 0x56, 0x78) // crc

	code, err := qrcode.Encode(data, qrcode.DefaultOptions())
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}
	decoded, err := qrcode.DecodeImageBytes(code.Image(8))
	if err != nil {
		t.Fatalf("DecodeImageBytes 失败: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Errorf("二进制闭环解码结果不匹配\n原始(%d): %x\n解码(%d): %x", len(data), data, len(decoded), decoded)
	}
}

// TestEncodeAutoDowngrade 载荷不足自动降级：小载荷 + version 上限 20 → 实际版本应远小于 20
func TestEncodeAutoDowngrade(t *testing.T) {
	opts := qrcode.Options{Version: 20, ECC: "L"}
	code, err := qrcode.Encode([]byte("tiny"), opts)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}
	if code.Version <= 0 || code.Version > 20 {
		t.Errorf("自动降级后版本应在 [1,20]，实际为 %d", code.Version)
	}
	if code.Version > 4 {
		t.Errorf("tiny 载荷应降级到很小版本，实际为 %d", code.Version)
	}
}

// TestEncodeWithinVersionLimit 数据可容纳于上限版本内 → 自动降级到最小版本且闭环
func TestEncodeWithinVersionLimit(t *testing.T) {
	// 500 字节可容纳于 version 20 以内（自动降级到最小版本）
	data := bytes.Repeat([]byte("abcdefghij"), 50)
	code, err := qrcode.Encode(data, qrcode.Options{Version: 20, ECC: "L"})
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}
	if code.Version < 1 || code.Version > 20 {
		t.Errorf("版本应在 [1,20]，实际为 %d", code.Version)
	}
	decoded, err := qrcode.DecodeImageBytes(code.Image(6))
	if err != nil {
		t.Fatalf("DecodeImageBytes 失败: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Error("自动降级下二进制闭环不一致")
	}
}

// TestEncodeExceedsVersionLimit 数据超出上限版本容量 → 明确错误，不截断
func TestEncodeExceedsVersionLimit(t *testing.T) {
	// 900 字节：version 20 (L) 容量约 858B，需 version 21 → 超出上限报错
	data := bytes.Repeat([]byte("x"), 900)
	_, err := qrcode.Encode(data, qrcode.Options{Version: 20, ECC: "L"})
	if err == nil {
		t.Fatal("超出 version 20 容量应报错")
	}
	if !strings.Contains(err.Error(), "超出") {
		t.Errorf("错误信息应提示超出容量，实际: %v", err)
	}
}

// TestEncodeOverCapacity 载荷超出上限版本容量 → 明确错误，不截断
func TestEncodeOverCapacity(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 3000) // 超过 version 40 最大容量（~2953B）
	opts := qrcode.Options{Version: 40, ECC: "L"}
	_, err := qrcode.Encode(data, opts)
	if err == nil {
		t.Fatal("超容量编码应返回错误")
	}
	// 小版本上限超容量
	opts2 := qrcode.Options{Version: 1, ECC: "L"}
	_, err = qrcode.Encode(bytes.Repeat([]byte("y"), 100), opts2)
	if err == nil {
		t.Fatal("超出 version 1 容量应返回错误")
	}
}

// TestValidate 参数范围校验
func TestValidate(t *testing.T) {
	// version 0 → 默认 20
	norm, err := qrcode.Validate(qrcode.Options{Version: 0, ECC: ""})
	if err != nil {
		t.Fatalf("version=0 应视为默认，实际报错: %v", err)
	}
	if norm.Version != qrcode.DefaultVersion || norm.ECC != qrcode.DefaultECC {
		t.Errorf("默认规范化不正确: %+v", norm)
	}

	// 非法 version
	if _, err := qrcode.Validate(qrcode.Options{Version: 41, ECC: "L"}); err == nil {
		t.Error("version=41 应报错")
	}
	if _, err := qrcode.Validate(qrcode.Options{Version: -1, ECC: "L"}); err == nil {
		t.Error("version=-1 应报错")
	}

	// 非法 ecc
	if _, err := qrcode.Validate(qrcode.Options{Version: 20, ECC: "X"}); err == nil {
		t.Error("ecc=X 应报错")
	}
	if _, err := qrcode.Validate(qrcode.Options{Version: 20, ECC: "LL"}); err == nil {
		t.Error("ecc=LL 应报错")
	}

	// 合法值
	if _, err := qrcode.Validate(qrcode.Options{Version: 1, ECC: "L"}); err != nil {
		t.Errorf("version=1 应合法: %v", err)
	}
	if _, err := qrcode.Validate(qrcode.Options{Version: 40, ECC: "H"}); err != nil {
		t.Errorf("version=40 H 应合法: %v", err)
	}
}

// TestRenderANSI 渲染仅含半块字符/空格、含静区、无彩色
func TestRenderANSI(t *testing.T) {
	code, err := qrcode.EncodeText("ANSI render test")
	if err != nil {
		t.Fatalf("EncodeText 失败: %v", err)
	}
	out := qrcode.RenderANSI(code, false)
	if out == "" {
		t.Fatal("渲染结果为空")
	}
	if strings.ContainsRune(out, '\x1b') {
		t.Error("渲染结果不应包含 ANSI 转义（无彩色）")
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	for i, line := range lines {
		for _, r := range line {
			if r != ' ' && r != '▀' && r != '▄' && r != '█' {
				t.Errorf("第 %d 行含非法字符 %q", i, r)
			}
		}
	}
	// 顶部静区：前 2 个字符行应全为空格（静区 ≥4 模块）
	if strings.TrimSpace(lines[0]) != "" || strings.TrimSpace(lines[1]) != "" {
		t.Error("顶部应有静区（全空格行）")
	}
	// 底部静区：最后 2 行应全为空格
	if strings.TrimSpace(lines[len(lines)-1]) != "" || strings.TrimSpace(lines[len(lines)-2]) != "" {
		t.Error("底部应有静区（全空格行）")
	}
}

// TestRenderANSIInvert 反色渲染
func TestRenderANSIInvert(t *testing.T) {
	code, err := qrcode.EncodeText("invert")
	if err != nil {
		t.Fatalf("EncodeText 失败: %v", err)
	}
	normal := qrcode.RenderANSI(code, false)
	inverted := qrcode.RenderANSI(code, true)
	if normal == inverted {
		t.Error("反色渲染应与正常渲染不同")
	}
}

// TestRenderASCII ASCII 降级可用
func TestRenderASCII(t *testing.T) {
	code, err := qrcode.EncodeText("ascii")
	if err != nil {
		t.Fatalf("EncodeText 失败: %v", err)
	}
	out := qrcode.RenderASCII(code, false)
	if out == "" {
		t.Fatal("ASCII 渲染为空")
	}
	if !strings.ContainsRune(out, '#') {
		t.Error("ASCII 渲染应包含 '#' 黑模块")
	}
	if !strings.Contains(out, " ") {
		t.Error("ASCII 渲染应包含空格白模块")
	}
}

// TestEncodeTextTooLong 短文本超过 200 字节 → 报错提示走帧流
func TestEncodeTextTooLong(t *testing.T) {
	long := strings.Repeat("长文本", 100) // 300 字节
	_, err := qrcode.EncodeText(long)
	if err == nil {
		t.Fatal("超过 200 字节的文本直传应报错")
	}
}

// TestEncodeEmpty 空数据 → 报错
func TestEncodeEmpty(t *testing.T) {
	_, err := qrcode.Encode([]byte{}, qrcode.DefaultOptions())
	if err == nil {
		t.Fatal("空数据编码应报错")
	}
}
