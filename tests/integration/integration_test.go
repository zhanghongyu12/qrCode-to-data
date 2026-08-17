package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qrcd/internal/payload"
	"qrcd/internal/qrcode"
	"qrcd/internal/receive"
	"qrcd/internal/send"
)

// 集成测试：调用完整 send.Send / receive.Receive API（而非直接操作 Processor），
// 覆盖真实文件读写闭环、参数矩阵、覆盖控制、载荷类型。
// 与 tests/unit/sendreceive 互补：后者用 Processor 内部 API，本层走完整 Receive 路径
// （capture file 源 → splitFrames → Processor → printResult）。

// buildFrames 用 send.BuildStream 生成全部帧字节。
func buildFrames(t *testing.T, data []byte, opts send.Options) (*send.Stream, [][]byte) {
	t.Helper()
	load := &payload.Load{Data: data, Name: "itest.bin", PayloadType: "file", MimeType: "application/octet-stream"}
	stream, err := send.BuildStream(load, opts)
	if err != nil {
		t.Fatalf("BuildStream: %v", err)
	}
	var frames [][]byte
	for {
		it, ok := stream.Next()
		if !ok {
			break
		}
		frames = append(frames, it.Bytes)
	}
	return stream, frames
}

// writeQRImages 将帧字节编码为 QR PNG 写入临时目录，返回目录。
func writeQRImages(t *testing.T, frames [][]byte) string {
	t.Helper()
	dir := t.TempDir()
	for i, fb := range frames {
		code, err := qrcode.Encode(fb, qrcode.Options{Version: 20, ECC: "L"})
		if err != nil {
			t.Fatalf("帧 %d 编码: %v", i, err)
		}
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%04d.png", i)))
		if err != nil {
			t.Fatalf("创建图片: %v", err)
		}
		if err := png.Encode(f, code.Image(8)); err != nil {
			f.Close()
			t.Fatalf("png: %v", err)
		}
		f.Close()
	}
	return dir
}

// roundtripViaFileSource 生成帧→QR 图片→file 源完整还原，返回还原字节。
func roundtripViaFileSource(t *testing.T, data []byte, sendOpts send.Options) []byte {
	t.Helper()
	_, frames := buildFrames(t, data, sendOpts)
	dir := writeQRImages(t, frames)
	outDir := t.TempDir()
	res, err := receive.Receive(context.Background(), receive.Options{
		Source: "file", FilePath: dir, Output: outDir,
		Overwrite: true, Out: io.Discard, ProgressOut: io.Discard, FPS: 1000,
	})
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	got, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatalf("读取还原文件: %v", err)
	}
	return got
}

// TestSendFile_PlayAndHash send.Send 文件流：Out 接渲染，验证 SHA-256 与帧数。
func TestSendFile_PlayAndHash(t *testing.T) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i)
	}
	tf, err := os.CreateTemp("", "send-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	tf.Write(data)
	tf.Close()
	defer os.Remove(tf.Name())

	var out, prog bytes.Buffer
	res, err := send.Send(context.Background(), send.Options{
		File:       tf.Name(),
		BlockSize:  1024, Version: 20, ECC: "L", Redundancy: 0.1, FPS: 1000,
		Out: &out, ProgressOut: &prog,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if res.ShortText {
		t.Error("文件载荷不应走短文本直传")
	}
	if res.Frames == 0 {
		t.Error("帧流应产出数据帧")
	}
	if res.SHA256 != payload.SHA256Hex(data) {
		t.Errorf("SHA-256 不一致: %s vs %s", res.SHA256, payload.SHA256Hex(data))
	}
	if !strings.Contains(prog.String(), "SHA-256:") {
		t.Errorf("应输出 SHA-256，实际: %q", prog.String())
	}
	if out.Len() == 0 {
		t.Error("应渲染 QR 到 Out")
	}
}

// TestSendTextOverLimit_SwitchToStream 文本超 200B 自动切帧流。
func TestSendTextOverLimit_SwitchToStream(t *testing.T) {
	text := strings.Repeat("码", 120) // 360 字节 > 200
	var out, prog, stderr bytes.Buffer
	res, err := send.Send(context.Background(), send.Options{
		Text:       text,
		BlockSize:  1024, Version: 20, ECC: "L", Redundancy: 0.1, FPS: 1000,
		Out: &out, ProgressOut: &prog,
	})
	_ = stderr
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if res.ShortText {
		t.Error("超 200B 文本不应走短文本直传")
	}
	if res.Frames == 0 {
		t.Error("应切换为帧流并产出数据帧")
	}
	if res.SHA256 != payload.SHA256Hex([]byte(text)) {
		t.Errorf("SHA-256 不一致")
	}
}

// TestReceiveFromFileSource 完整 Receive 路径（file 源）还原 + 校验通过输出。
func TestReceiveFromFileSource(t *testing.T) {
	data := make([]byte, 8192)
	for i := range data {
		data[i] = byte(i * 5)
	}
	_, frames := buildFrames(t, data, send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.2})
	dir := writeQRImages(t, frames)

	var prog bytes.Buffer
	outDir := t.TempDir()
	res, err := receive.Receive(context.Background(), receive.Options{
		Source: "file", FilePath: dir, Output: outDir,
		Overwrite: true, Out: &prog, ProgressOut: &prog, FPS: 1000,
	})
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	got, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatalf("读取还原文件: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("还原数据与原文不一致\n原文: %d, 还原: %d", len(data), len(got))
	}
	if !strings.Contains(prog.String(), "校验通过") {
		t.Errorf("应输出校验通过，实际: %q", prog.String())
	}
	if !strings.Contains(prog.String(), "SHA-256:") {
		t.Errorf("应输出 SHA-256")
	}
}

// TestOverwriteControl Overwrite=false 已存在文件报错，true 覆盖。
func TestOverwriteControl(t *testing.T) {
	data := []byte("overwrite control test")
	_, frames := buildFrames(t, data, send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.1})
	dir := writeQRImages(t, frames)
	outDir := t.TempDir()

	// 首次写入成功
	if _, err := receive.Receive(context.Background(), receive.Options{
		Source: "file", FilePath: dir, Output: outDir, Overwrite: true,
		Out: io.Discard, ProgressOut: io.Discard, FPS: 1000,
	}); err != nil {
		t.Fatalf("首次写入失败: %v", err)
	}

	// Overwrite=false 且文件已存在应报错
	_, err := receive.Receive(context.Background(), receive.Options{
		Source: "file", FilePath: dir, Output: outDir, Overwrite: false,
		Out: io.Discard, ProgressOut: io.Discard, FPS: 1000,
	})
	if err == nil {
		t.Error("Overwrite=false 且文件已存在应报错")
	}

	// Overwrite=true 覆盖成功
	if _, err := receive.Receive(context.Background(), receive.Options{
		Source: "file", FilePath: dir, Output: outDir, Overwrite: true,
		Out: io.Discard, ProgressOut: io.Discard, FPS: 1000,
	}); err != nil {
		t.Errorf("Overwrite=true 应成功: %v", err)
	}
}

// TestBlockSizeMatrix 不同分块大小均能闭环还原。
func TestBlockSizeMatrix(t *testing.T) {
	data := make([]byte, 8192)
	for i := range data {
		data[i] = byte(i)
	}
	for _, bs := range []int{256, 1024, 4096} {
		t.Run(fmt.Sprintf("bs=%d", bs), func(t *testing.T) {
			got := roundtripViaFileSource(t, data, send.Options{BlockSize: bs, Version: 20, ECC: "L", Redundancy: 0.1})
			if !bytes.Equal(got, data) {
				t.Errorf("BlockSize=%d 还原不一致", bs)
			}
		})
	}
}

// TestECCMatrix 不同纠错级别均能闭环还原。
func TestECCMatrix(t *testing.T) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i)
	}
	for _, ecc := range []string{"L", "M", "Q", "H"} {
		t.Run(fmt.Sprintf("ecc=%s", ecc), func(t *testing.T) {
			got := roundtripViaFileSource(t, data, send.Options{BlockSize: 1024, Version: 20, ECC: ecc, Redundancy: 0.1})
			if !bytes.Equal(got, data) {
				t.Errorf("ECC=%s 还原不一致", ecc)
			}
		})
	}
}

// TestTextAndBinaryPayload 文本与二进制两类载荷均能闭环。
func TestTextAndBinaryPayload(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		text := strings.Repeat("中文文本载荷测试", 40) // >200B
		got := roundtripViaFileSource(t, []byte(text), send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.1})
		if string(got) != text {
			t.Errorf("文本载荷还原不一致")
		}
	})
	t.Run("binary", func(t *testing.T) {
		data := make([]byte, 6000)
		for i := range data {
			data[i] = byte(i % 256)
		}
		got := roundtripViaFileSource(t, data, send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.1})
		if !bytes.Equal(got, data) {
			t.Errorf("二进制载荷还原不一致")
		}
	})
}
