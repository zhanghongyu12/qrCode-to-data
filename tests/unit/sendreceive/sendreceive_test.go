package sendreceive_test

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qrcd/internal/capture"
	"qrcd/internal/payload"
	"qrcd/internal/qrcode"
	"qrcd/internal/receive"
	"qrcd/internal/send"
)

// collectStream 收集帧流全部帧字节。
func collectStream(t *testing.T, data []byte, opts send.Options) (*send.Stream, [][]byte) {
	t.Helper()
	load := &payload.Load{Data: data, Name: "test.bin", PayloadType: "file", MimeType: "application/octet-stream"}
	stream, err := send.BuildStream(load, opts)
	if err != nil {
		t.Fatalf("BuildStream 失败: %v", err)
	}
	var frames [][]byte
	for {
		item, ok := stream.Next()
		if !ok {
			break
		}
		frames = append(frames, item.Bytes)
	}
	return stream, frames
}

// feedAll 将帧字节喂给接收处理器，返回结果。
func feedAll(t *testing.T, proc *receive.Processor, frames [][]byte) *receive.Result {
	t.Helper()
	for _, fb := range frames {
		if err := proc.Process(fb); err != nil {
			t.Fatalf("Process 失败: %v", err)
		}
		if proc.Done() {
			break
		}
	}
	res, err := proc.Finish()
	if err != nil {
		t.Fatalf("Finish 失败: %v", err)
	}
	return res
}

// newTestProcessor 创建输出到临时目录的接收处理器。
func newTestProcessor(t *testing.T, outDir string, mut func(*receive.Options)) *receive.Processor {
	t.Helper()
	opts := receive.Options{
		Output:      outDir,
		Overwrite:   true,
		Out:         &bytes.Buffer{},
		ProgressOut: &bytes.Buffer{},
	}
	if mut != nil {
		mut(&opts)
	}
	proc, err := receive.NewProcessor(opts)
	if err != nil {
		t.Fatalf("NewProcessor 失败: %v", err)
	}
	return proc
}

// TestE2EInMemoryRoundtrip 内存串联：分块→FEC→组帧→拆帧→FEC→校验→落盘，逐字节一致。
func TestE2EInMemoryRoundtrip(t *testing.T) {
	// 5KB 确定性数据 → K≥7（raptor）
	data := make([]byte, 5120)
	for i := range data {
		data[i] = byte(i * 7)
	}
	opts := send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.1}
	stream, frames := collectStream(t, data, opts)

	outDir := t.TempDir()
	proc := newTestProcessor(t, outDir, nil)
	res := feedAll(t, proc, frames)

	got, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatalf("读取还原文件失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("还原数据与原文不一致\n原文长度: %d, 还原长度: %d", len(data), len(got))
	}
	if res.SHA256 != payload.SHA256Hex(data) {
		t.Errorf("SHA-256 不一致: %s vs %s", res.SHA256, payload.SHA256Hex(data))
	}
	if res.Size != int64(len(data)) {
		t.Errorf("大小不一致: %d vs %d", res.Size, len(data))
	}
	if stream.Meta().BlockCount < 4 {
		t.Errorf("期望 FEC K≥4（raptor），实际 K=%d", stream.Meta().BlockCount)
	}
	if stream.Meta().FEC != "raptor" {
		t.Errorf("期望 raptor 方案，实际 %s", stream.Meta().FEC)
	}
}

// TestE2EPacketLoss 丢帧 20% 仍可还原（FEC 兜底，K≥50 规避伪满秩）。
func TestE2EPacketLoss(t *testing.T) {
	// 60KB → 自动分块 K≈75，冗余 0.5 → 约 113 数据帧，丢 20% 后仍 ≥ K
	data := make([]byte, 61440)
	for i := range data {
		data[i] = byte(i * 13)
	}
	opts := send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.5}
	stream, frames := collectStream(t, data, opts)

	if stream.Meta().BlockCount < 50 {
		t.Fatalf("期望 K≥50（DEC-005），实际 K=%d", stream.Meta().BlockCount)
	}

	// 丢 ~20% 数据帧（seq%5==4 丢弃），保留全部元数据帧
	var kept [][]byte
	dropped := 0
	total := 0
	for i, fb := range frames {
		// 先解析帧头判断是否数据帧
		isData := !strings.HasPrefix(string(fb), "QRCD\x01\x01") // 元数据帧 type=0x01
		if isData {
			total++
			if total%5 == 4 {
				dropped++
				continue
			}
		}
		_ = i
		kept = append(kept, fb)
	}
	if dropped < 10 {
		t.Fatalf("丢帧数过少: %d", dropped)
	}

	outDir := t.TempDir()
	proc := newTestProcessor(t, outDir, nil)
	res := feedAll(t, proc, kept)

	got, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatalf("读取还原文件失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("丢帧场景还原数据与原文不一致\n原文长度: %d, 还原长度: %d, 丢帧: %d", len(data), len(got), dropped)
	}
}

// TestE2EMetaLate 元数据帧迟到（数据帧先到）仍能初始化会话并还原。
func TestE2EMetaLate(t *testing.T) {
	data := bytes.Repeat([]byte("ABCDEFGH"), 512) // 4096 字节
	opts := send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.2}
	_, frames := collectStream(t, data, opts)

	// 分离元数据帧与数据帧
	var metas, datas [][]byte
	for _, fb := range frames {
		if strings.HasPrefix(string(fb), "QRCD\x01\x01") {
			metas = append(metas, fb)
		} else {
			datas = append(datas, fb)
		}
	}
	if len(metas) == 0 || len(datas) == 0 {
		t.Fatal("应同时包含元数据帧与数据帧")
	}

	// 先喂全部数据帧，再喂元数据帧
	var reordered [][]byte
	reordered = append(reordered, datas...)
	reordered = append(reordered, metas...)

	outDir := t.TempDir()
	proc := newTestProcessor(t, outDir, nil)
	res := feedAll(t, proc, reordered)

	got, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatalf("读取还原文件失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("元数据迟到场景还原数据与原文不一致")
	}
}

// TestE2ESingleBlockNone 单块载荷走 none 短路。
func TestE2ESingleBlockNone(t *testing.T) {
	data := []byte("single-block file content, less than one QR frame")
	opts := send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.1}
	stream, frames := collectStream(t, data, opts)

	if stream.Meta().FEC != "none" {
		t.Fatalf("期望 none 方案，实际 %s", stream.Meta().FEC)
	}
	if len(frames) != 2 { // 元数据 + 单个数据帧
		t.Fatalf("期望 2 帧，实际 %d", len(frames))
	}

	outDir := t.TempDir()
	proc := newTestProcessor(t, outDir, nil)
	res := feedAll(t, proc, frames)

	got, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatalf("读取还原文件失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("none 短路还原数据与原文不一致")
	}
}

// TestShortTextDirect 短文本（≤200 字节）走静态标准文本二维码直传。
func TestShortTextDirect(t *testing.T) {
	text := "你好，qrcd 短文本直传测试"
	var out, progress bytes.Buffer
	res, err := send.Send(nil, send.Options{
		Text:        text,
		Terminal:    "ansi",
		Out:         &out,
		ProgressOut: &progress,
		FPS:         10, Version: 20, ECC: "L", Redundancy: 0.1, BlockSize: 1024,
	})
	if err != nil {
		t.Fatalf("send.Send 失败: %v", err)
	}
	if !res.ShortText {
		t.Error("短文本应标记 ShortText=true")
	}
	if res.SHA256 != payload.SHA256Hex([]byte(text)) {
		t.Errorf("SHA-256 不一致")
	}
	if out.Len() == 0 {
		t.Error("静态二维码渲染输出为空")
	}
	if !strings.Contains(out.String(), "▀") && !strings.Contains(out.String(), "█") {
		t.Error("ANSI 渲染应包含半块字符")
	}
}

// TestMetaReplay 元数据帧周期性重播（每 20 个数据帧一次）。
func TestMetaReplay(t *testing.T) {
	// 80KB → 约 100 数据帧 → 应有 1 + 100/20 = 6 个元数据帧
	data := make([]byte, 81920)
	for i := range data {
		data[i] = byte(i)
	}
	opts := send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.0}
	stream, frames := collectStream(t, data, opts)

	if stream.TotalData() < 20 {
		t.Fatalf("数据帧过少: %d", stream.TotalData())
	}
	metaCount := 0
	dataCount := 0
	for _, fb := range frames {
		if strings.HasPrefix(string(fb), "QRCD\x01\x01") {
			metaCount++
		} else {
			dataCount++
		}
	}
	expectedMeta := 1 + (dataCount-1)/20
	if metaCount != expectedMeta {
		t.Errorf("元数据重播次数应约 %d，实际 %d（数据帧 %d）", expectedMeta, metaCount, dataCount)
	}
	if metaCount < 2 {
		t.Error("元数据周期重播应可观察（至少 2 次）")
	}
}

// TestReceiveDedup 重复帧去重不重复计数。
func TestReceiveDedup(t *testing.T) {
	// 多块载荷（raptor，K≥4），交错重复帧在完成前处理，去重计数可观察
	data := make([]byte, 5120)
	for i := range data {
		data[i] = byte(i * 3)
	}
	opts := send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.2}
	stream, frames := collectStream(t, data, opts)

	if stream.Meta().BlockCount < 4 {
		t.Fatalf("去重测试需多块载荷，实际 K=%d", stream.Meta().BlockCount)
	}

	// 每帧重复一次并交错（meta, d0, d0, d1, d1, ...）
	var doubled [][]byte
	for _, fb := range frames {
		doubled = append(doubled, fb)
		doubled = append(doubled, fb)
	}

	outDir := t.TempDir()
	proc := newTestProcessor(t, outDir, nil)
	res := feedAll(t, proc, doubled)

	got, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatalf("读取还原文件失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("去重场景还原数据与原文不一致")
	}
	count, total, dedup := proc.Stats()
	if dedup == 0 {
		t.Error("应有去重计数")
	}
	if count == 0 {
		t.Error("唯一数据符号数应为正")
	}
	if count > total {
		t.Errorf("唯一块数不应超过期望块数 %d，实际 %d", total, count)
	}
}

// TestE2EQRImages 生成 QR 图片到临时目录，再用 file 帧源读回解码还原。
func TestE2EQRImages(t *testing.T) {
	data := bytes.Repeat([]byte("QR-IMAGE-ROUNDTRIP!"), 256) // 4096 字节
	opts := send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.2}
	stream, frames := collectStream(t, data, opts)
	_ = stream

	dir := t.TempDir()
	for i, fb := range frames {
		code, err := qrcode.Encode(fb, qrcode.Options{Version: 20, ECC: "L"})
		if err != nil {
			t.Fatalf("帧 %d 编码失败: %v", i, err)
		}
		path := filepath.Join(dir, fmt.Sprintf("%04d.png", i))
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("创建图片失败: %v", err)
		}
		if err := png.Encode(f, code.Image(8)); err != nil {
			f.Close()
			t.Fatalf("编码图片失败: %v", err)
		}
		f.Close()
	}

	// 用 file 帧源读回解码
	src, err := capture.NewSource("file", capture.Options{FilePath: dir})
	if err != nil {
		t.Fatalf("NewSource(file) 失败: %v", err)
	}
	defer src.Close()

	outDir := t.TempDir()
	proc := newTestProcessor(t, outDir, nil)
	for {
		fr, err := src.Next()
		if err != nil {
			break
		}
		if fr.Image == nil {
			t.Fatal("file 帧应提供 Image")
		}
		fb, err := qrcode.DecodeImageBytes(fr.Image)
		if err != nil {
			t.Logf("帧解码失败（跳过）: %v", err)
			continue
		}
		if err := proc.Process(fb); err != nil {
			t.Fatalf("Process 失败: %v", err)
		}
		if proc.Done() {
			break
		}
	}
	res, err := proc.Finish()
	if err != nil {
		t.Fatalf("Finish 失败: %v", err)
	}
	got, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatalf("读取还原文件失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("QR 图片闭环还原数据与原文不一致\n原文长度: %d, 还原长度: %d", len(data), len(got))
	}
}

// TestExpectSizeMismatch --expect-size 不匹配报错。
func TestExpectSizeMismatch(t *testing.T) {
	data := []byte("expect size mismatch test data")
	opts := send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.1}
	_, frames := collectStream(t, data, opts)

	proc := newTestProcessor(t, t.TempDir(), func(o *receive.Options) {
		o.ExpectSize = int64(len(data) + 100)
	})
	if err := proc.Process(frames[0]); err == nil {
		t.Fatal("expect-size 不匹配应报错")
	} else if !strings.Contains(err.Error(), "不匹配") {
		t.Errorf("错误信息应提示不匹配: %v", err)
	}
}

// TestHashMismatch --hash 预校验不匹配报错。
func TestHashMismatch(t *testing.T) {
	data := []byte("hash mismatch test data")
	opts := send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.1}
	_, frames := collectStream(t, data, opts)

	proc := newTestProcessor(t, t.TempDir(), func(o *receive.Options) {
		o.Hash = "sha256:" + strings.Repeat("0", 64)
	})
	if err := proc.Process(frames[0]); err == nil {
		t.Fatal("hash 预校验不匹配应报错")
	} else if !strings.Contains(err.Error(), "SHA-256") {
		t.Errorf("错误信息应提示 SHA-256: %v", err)
	}
}

// TestReceiveNoFrames 无足够符号 → Finish 报错。
func TestReceiveNoFrames(t *testing.T) {
	proc := newTestProcessor(t, t.TempDir(), nil)
	if _, err := proc.Finish(); err == nil {
		t.Fatal("无符号时 Finish 应报错")
	}
}
