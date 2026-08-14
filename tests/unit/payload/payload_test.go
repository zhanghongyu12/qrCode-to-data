package payload_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qrcd/internal/payload"
)

// TestReadFile 文件读取字节一致
func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	content := []byte("file payload content 0123456789")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	load, err := payload.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile 失败: %v", err)
	}
	if !bytes.Equal(load.Data, content) {
		t.Errorf("文件读取字节不一致\n期望(%d): %x\n实际(%d): %x", len(content), content, len(load.Data), load.Data)
	}
	if load.PayloadType != "file" {
		t.Errorf("PayloadType 应为 file，实际 %s", load.PayloadType)
	}
	if load.Name != "data.bin" {
		t.Errorf("Name 应为 data.bin，实际 %s", load.Name)
	}
}

// TestReadFileNotExist 文件不存在 → 中文报错
func TestReadFileNotExist(t *testing.T) {
	_, err := payload.ReadFile(filepath.Join(t.TempDir(), "no_such_file.bin"))
	if err == nil {
		t.Fatal("读取不存在的文件应报错")
	}
	if !strings.Contains(err.Error(), "读取文件失败") {
		t.Errorf("错误信息应包含中文上下文，实际: %v", err)
	}
}

// TestReadText 文本读取字节一致
func TestReadText(t *testing.T) {
	text := "文本载荷测试 hello"
	load, err := payload.ReadText(text)
	if err != nil {
		t.Fatalf("ReadText 失败: %v", err)
	}
	if string(load.Data) != text {
		t.Errorf("文本读取不一致: %q", load.Data)
	}
	if load.PayloadType != "text" {
		t.Errorf("PayloadType 应为 text，实际 %s", load.PayloadType)
	}
	if !strings.HasSuffix(load.Name, ".txt") {
		t.Errorf("文本摘要名应以 .txt 结尾，实际 %s", load.Name)
	}
}

// TestReadStdin 从 reader 读取字节一致
func TestReadStdin(t *testing.T) {
	content := []byte("stdin stream content")
	load, err := payload.ReadStdin(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("ReadStdin 失败: %v", err)
	}
	if !bytes.Equal(load.Data, content) {
		t.Errorf("stdin 读取字节不一致")
	}
}

// TestChunk 分块边界正确（含尾部不满一块）
func TestChunk(t *testing.T) {
	data := []byte("0123456789abcdefghij") // 20 字节
	chunks := payload.Chunk(data, 8)
	if len(chunks) != 3 {
		t.Fatalf("20 字节按 8 分块应为 3 块，实际 %d", len(chunks))
	}
	if string(chunks[0]) != "01234567" {
		t.Errorf("chunk[0] 错误: %q", chunks[0])
	}
	if string(chunks[1]) != "89abcdef" {
		t.Errorf("chunk[1] 错误: %q", chunks[1])
	}
	// 尾部不满一块
	if string(chunks[2]) != "ghij" {
		t.Errorf("chunk[2] 应保留尾部实际长度 ghij，实际 %q", chunks[2])
	}

	// 恰好整除
	chunks = payload.Chunk(data, 10)
	if len(chunks) != 2 {
		t.Errorf("20 字节按 10 分块应为 2 块，实际 %d", len(chunks))
	}

	// 空数据
	if n := payload.BlockCount([]byte{}, 1024); n != 0 {
		t.Errorf("空数据块数应为 0，实际 %d", n)
	}
}

// TestBlockCount 分块数计算
func TestBlockCount(t *testing.T) {
	if n := payload.BlockCount([]byte("1234567890"), 3); n != 4 {
		t.Errorf("10 字节按 3 分块应为 4，实际 %d", n)
	}
	if n := payload.BlockCount([]byte("1234"), 0); n != 0 {
		t.Errorf("blockSize=0 应为 0，实际 %d", n)
	}
}

// TestSHA256Hex 与标准 sha256sum 结果一致
func TestSHA256Hex(t *testing.T) {
	// 空字符串
	if got := payload.SHA256Hex([]byte("")); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("空字符串 SHA-256 不匹配: %s", got)
	}
	// "abc"
	if got := payload.SHA256Hex([]byte("abc")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("abc SHA-256 不匹配: %s", got)
	}
}

// TestCRC32 与标准 crc32 IEEE 结果一致
func TestCRC32(t *testing.T) {
	// "123456789" 的 CRC32 IEEE = 0xCBF43926
	if got := payload.CRC32([]byte("123456789")); got != 0xCBF43926 {
		t.Errorf("123456789 CRC32 应为 0xCBF43926，实际 0x%08X", got)
	}
}

// TestWriteFileAtomic 写入走 .part → rename，无 .part 残留
func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.bin")
	data := []byte("atomic write data")

	if err := payload.WriteFile(path, data, payload.WriteOptions{}); err != nil {
		t.Fatalf("WriteFile 失败: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取最终文件失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Error("写入内容不一致")
	}
	if _, err := os.Stat(path + ".part"); !os.IsNotExist(err) {
		t.Error(".part 临时文件应已清理")
	}
}

// TestWriteFileNoOverwrite 已存在文件且 overwrite=false → 报错
func TestWriteFileNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.bin")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("预置文件失败: %v", err)
	}
	err := payload.WriteFile(path, []byte("new"), payload.WriteOptions{Overwrite: false})
	if err == nil {
		t.Fatal("已存在文件且 overwrite=false 应报错")
	}
	if !strings.Contains(err.Error(), "已存在") {
		t.Errorf("错误信息应提示文件已存在，实际: %v", err)
	}
	// 原文件未被覆盖
	got, _ := os.ReadFile(path)
	if string(got) != "old" {
		t.Error("overwrite=false 不应覆盖原文件")
	}

	// overwrite=true 可覆盖
	if err := payload.WriteFile(path, []byte("new"), payload.WriteOptions{Overwrite: true}); err != nil {
		t.Fatalf("overwrite=true 覆盖失败: %v", err)
	}
}

// TestWriteFileVerifyFail 校验失败 → 报错、不产生最终文件、不产生 .part
func TestWriteFileVerifyFail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "verify.bin")
	wrongHash := strings.Repeat("0", 64)
	err := payload.WriteFile(path, []byte("data"), payload.WriteOptions{VerifySHA256: wrongHash})
	if err == nil {
		t.Fatal("校验失败应报错")
	}
	if !strings.Contains(err.Error(), "校验失败") {
		t.Errorf("错误信息应含校验失败，实际: %v", err)
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Error("校验失败不应产生最终文件")
	}
	if _, e := os.Stat(path + ".part"); !os.IsNotExist(e) {
		t.Error("校验失败不应产生 .part")
	}
}

// TestWriteFileVerifyOK 校验通过可落盘
func TestWriteFileVerifyOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "verify_ok.bin")
	data := []byte("verify ok data")
	hash := payload.SHA256Hex(data)
	if err := payload.WriteFile(path, data, payload.WriteOptions{VerifySHA256: hash}); err != nil {
		t.Fatalf("校验通过写入失败: %v", err)
	}
}

// TestWriteFileDirNotWritable 输出目录不存在/不可写 → 中文报错
func TestWriteFileDirNotWritable(t *testing.T) {
	// 父目录不存在 → .part 无法写入
	path := filepath.Join(t.TempDir(), "no_such_subdir", "out.bin")
	err := payload.WriteFile(path, []byte("data"), payload.WriteOptions{})
	if err == nil {
		t.Fatal("不可写目录应报错")
	}
	if !strings.Contains(err.Error(), "写入临时文件失败") {
		t.Errorf("错误信息应提示写入临时文件失败，实际: %v", err)
	}
	if _, e := os.Stat(path + ".part"); !os.IsNotExist(e) {
		t.Error("失败后不应残留 .part")
	}
}

// TestOutputPath 输出目录/完整文件名解析
func TestOutputPath(t *testing.T) {
	dir := t.TempDir()
	// 已存在目录 → 拼接文件名
	p, err := payload.OutputPath(dir, "a.txt")
	if err != nil {
		t.Fatalf("OutputPath 失败: %v", err)
	}
	if p != filepath.Join(dir, "a.txt") {
		t.Errorf("目录拼接不正确: %s", p)
	}

	// 完整文件名（不存在路径）
	file := filepath.Join(dir, "full.bin")
	p, err = payload.OutputPath(file, "a.txt")
	if err != nil {
		t.Fatalf("OutputPath 失败: %v", err)
	}
	if p != file {
		t.Errorf("完整文件名不正确: %s", p)
	}

	// 以分隔符结尾 → 视为目录
	sub := filepath.Join(dir, "subdir") + string(os.PathSeparator)
	p, err = payload.OutputPath(sub, "b.txt")
	if err != nil {
		t.Fatalf("OutputPath 失败: %v", err)
	}
	if p != filepath.Join(sub, "b.txt") {
		t.Errorf("目录分隔符结尾拼接不正确: %s", p)
	}
}

// TestSummaryName 确定性 + .txt 后缀
func TestSummaryName(t *testing.T) {
	a := payload.SummaryName("同一段文本")
	b := payload.SummaryName("同一段文本")
	if a != b {
		t.Errorf("同名文本摘要名应一致: %s vs %s", a, b)
	}
	if !strings.HasSuffix(a, ".txt") {
		t.Errorf("摘要名应以 .txt 结尾: %s", a)
	}
	if len(strings.TrimSuffix(a, ".txt")) != 16 {
		t.Errorf("摘要名前缀应为 16 位 hex，实际 %q", strings.TrimSuffix(a, ".txt"))
	}
}
