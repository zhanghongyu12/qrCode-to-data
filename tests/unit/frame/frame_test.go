package frame_test

import (
	"bytes"
	"testing"

	"qrcd/internal/frame"
)

// TestRoundtripMetaFrame 测试元数据帧「组帧→拆帧」字节一致
func TestRoundtripMetaFrame(t *testing.T) {
	tid := frame.MustNewTransferID()
	meta := &frame.MetaData{
		Name:        "test.txt",
		Size:        1024,
		BlockSize:   256,
		BlockCount:  4,
		HashAlgo:    "sha256",
		Hash:        "abcdef1234567890",
		FEC:         "raptor",
		Redundancy:  0.1,
		PayloadType: "file",
		MimeType:    "text/plain",
	}

	f, err := frame.BuildMetaFrame(tid, meta)
	if err != nil {
		t.Fatalf("BuildMetaFrame 失败: %v", err)
	}

	raw := frame.MarshalFrame(f)
	parsed, err := frame.UnmarshalFrame(raw)
	if err != nil {
		t.Fatalf("UnmarshalFrame 失败: %v", err)
	}

	if !parsed.IsMeta() {
		t.Fatal("解析后的帧应为元数据帧")
	}

	parsedMeta, err := parsed.ParseMeta()
	if err != nil {
		t.Fatalf("ParseMeta 失败: %v", err)
	}

	if parsedMeta.Name != meta.Name {
		t.Errorf("Name 不匹配: got %q, want %q", parsedMeta.Name, meta.Name)
	}
	if parsedMeta.Size != meta.Size {
		t.Errorf("Size 不匹配: got %d, want %d", parsedMeta.Size, meta.Size)
	}
	if parsedMeta.FEC != meta.FEC {
		t.Errorf("FEC 不匹配: got %q, want %q", parsedMeta.FEC, meta.FEC)
	}

	// 验证字节完全一致
	if !bytes.Equal(raw, frame.MarshalFrame(parsed)) {
		t.Error("MarshalFrame 往返结果不一致")
	}
}

// TestRoundtripDataFrame 测试数据帧「组帧→拆帧」字节一致
func TestRoundtripDataFrame(t *testing.T) {
	tid := frame.MustNewTransferID()
	payload := []byte("hello world data frame payload")
	seq := uint32(42)

	f := frame.BuildDataFrame(tid, seq, payload, "file", true)
	raw := frame.MarshalFrame(f)

	parsed, err := frame.UnmarshalFrame(raw)
	if err != nil {
		t.Fatalf("UnmarshalFrame 失败: %v", err)
	}

	if !parsed.IsData() {
		t.Fatal("解析后的帧应为数据帧")
	}
	if parsed.Header.Seq != seq {
		t.Errorf("Seq 不匹配: got %d, want %d", parsed.Header.Seq, seq)
	}
	if !bytes.Equal(parsed.Payload, payload) {
		t.Error("Payload 不匹配")
	}
	if parsed.Header.TransferID != tid {
		t.Error("TransferID 不匹配")
	}

	// 验证字节完全一致
	if !bytes.Equal(raw, frame.MarshalFrame(parsed)) {
		t.Error("MarshalFrame 往返结果不一致")
	}
}

// TestCRC32Tamper 测试 CRC32 篡改 1 字节→拆帧报错并丢弃
func TestCRC32Tamper(t *testing.T) {
	tid := frame.MustNewTransferID()
	payload := []byte("sensitive data")

	f := frame.BuildDataFrame(tid, 1, payload, "file", true)
	raw := frame.MarshalFrame(f)

	// 篡改 payload 中 1 字节
	raw[frame.HeaderSize+1] ^= 0xFF

	_, err := frame.UnmarshalFrame(raw)
	if err == nil {
		t.Fatal("CRC32 篡改应导致拆帧失败，但未报错")
	}
	if err != frame.ErrCRC32Mismatch {
		t.Logf("CRC32 篡改错误: %v (预期 ErrCRC32Mismatch)", err)
	}
}

// TestInvalidMagic 测试 magic 非法→丢弃
func TestInvalidMagic(t *testing.T) {
	tid := frame.MustNewTransferID()
	f := frame.BuildDataFrame(tid, 1, []byte("data"), "file", true)
	raw := frame.MarshalFrame(f)

	// 篡改 magic
	raw[0] = 0x00
	raw[1] = 0x00
	raw[2] = 0x00
	raw[3] = 0x00

	_, err := frame.UnmarshalFrame(raw)
	if err == nil {
		t.Fatal("magic 非法应导致拆帧失败，但未报错")
	}
	if err != frame.ErrInvalidMagic {
		t.Logf("magic 非法错误: %v (预期 ErrInvalidMagic)", err)
	}
}

// TestInvalidVersion 测试 version 非法→丢弃
func TestInvalidVersion(t *testing.T) {
	tid := frame.MustNewTransferID()
	f := frame.BuildDataFrame(tid, 1, []byte("data"), "file", true)
	raw := frame.MarshalFrame(f)

	// 篡改 version
	raw[4] = 0xFF

	_, err := frame.UnmarshalFrame(raw)
	if err == nil {
		t.Fatal("version 非法应导致拆帧失败，但未报错")
	}
	if err != frame.ErrInvalidVersion {
		t.Logf("version 非法错误: %v (预期 ErrInvalidVersion)", err)
	}
}

// TestSeqDedup 测试 seq 重复帧→去重忽略
func TestSeqDedup(t *testing.T) {
	tid := frame.MustNewTransferID()
	session := frame.NewSession(tid)

	seq := uint32(5)
	if !session.MarkReceived(seq) {
		t.Fatal("首次 MarkReceived 应返回 true")
	}
	if session.MarkReceived(seq) {
		t.Fatal("重复 MarkReceived 应返回 false")
	}
	if session.ReceivedCount() != 1 {
		t.Errorf("ReceivedCount 应为 1，实际为 %d", session.ReceivedCount())
	}
}

// TestSessionManager 测试 transfer_id 会话区分
func TestSessionManager(t *testing.T) {
	sm := frame.NewSessionManager()

	tid1 := frame.MustNewTransferID()
	tid2 := frame.MustNewTransferID()

	// 确保两个 ID 不同
	if tid1 == tid2 {
		t.Fatal("两个 transfer_id 相同，无法测试区分")
	}

	s1 := sm.GetOrCreate(tid1)
	s2 := sm.GetOrCreate(tid2)

	if s1 == s2 {
		t.Fatal("不同 transfer_id 应返回不同 session")
	}

	s1.MarkReceived(1)
	if s1.ReceivedCount() != 1 {
		t.Error("s1 应收到 1 个符号")
	}
	if s2.ReceivedCount() != 0 {
		t.Error("s2 不应收到符号")
	}

	// 验证 Get 返回相同 session
	if sm.Get(tid1) != s1 {
		t.Error("Get 应返回已有 session")
	}
	if sm.Get(tid2) != s2 {
		t.Error("Get 应返回已有 session")
	}

	// 验证移除
	sm.Remove(tid1)
	if sm.Get(tid1) != nil {
		t.Error("Remove 后 Get 应返回 nil")
	}
	if sm.Get(tid2) == nil {
		t.Error("移除 tid1 不应影响 tid2")
	}
}

// TestEmptyPayload 测试空 payload 帧
func TestEmptyPayload(t *testing.T) {
	tid := frame.MustNewTransferID()
	f := frame.BuildDataFrame(tid, 0, []byte{}, "file", false)
	raw := frame.MarshalFrame(f)

	parsed, err := frame.UnmarshalFrame(raw)
	if err != nil {
		t.Fatalf("空 payload 拆帧失败: %v", err)
	}
	if len(parsed.Payload) != 0 {
		t.Errorf("空 payload 解析后长度应为 0，实际为 %d", len(parsed.Payload))
	}
}

// TestFrameSize 验证帧总大小 = 32B 头 + payload + 4B CRC32
func TestFrameSize(t *testing.T) {
	tid := frame.MustNewTransferID()
	payload := make([]byte, 100)
	f := frame.BuildDataFrame(tid, 1, payload, "file", false)
	raw := frame.MarshalFrame(f)

	expectedSize := frame.HeaderSize + len(payload) + frame.CRC32Size
	if len(raw) != expectedSize {
		t.Errorf("帧大小不匹配: got %d, want %d", len(raw), expectedSize)
	}
}

// TestMetaDataJSONRoundtrip 测试元数据 JSON 序列化/反序列化
func TestMetaDataJSONRoundtrip(t *testing.T) {
	meta := &frame.MetaData{
		Name:        "backup.tar.gz",
		Size:        1048576,
		BlockSize:   1024,
		BlockCount:  1024,
		HashAlgo:    "sha256",
		Hash:        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		FEC:         "raptor",
		Redundancy:  0.1,
		PayloadType: "file",
		MimeType:    "application/octet-stream",
	}

	jsonBytes, err := frame.MarshalMetaJSON(meta)
	if err != nil {
		t.Fatalf("MarshalMetaJSON 失败: %v", err)
	}

	parsed, err := frame.UnmarshalMetaJSON(jsonBytes)
	if err != nil {
		t.Fatalf("UnmarshalMetaJSON 失败: %v", err)
	}

	if parsed.Name != meta.Name {
		t.Errorf("Name 不匹配: got %q, want %q", parsed.Name, meta.Name)
	}
	if parsed.Size != meta.Size {
		t.Errorf("Size 不匹配")
	}
	if parsed.BlockSize != meta.BlockSize {
		t.Errorf("BlockSize 不匹配")
	}
	if parsed.BlockCount != meta.BlockCount {
		t.Errorf("BlockCount 不匹配")
	}
	if parsed.FEC != meta.FEC {
		t.Errorf("FEC 不匹配")
	}
	if parsed.Redundancy != meta.Redundancy {
		t.Errorf("Redundancy 不匹配")
	}
	if parsed.PayloadType != meta.PayloadType {
		t.Errorf("PayloadType 不匹配")
	}
}

// TestMetaFrameFields 验证元数据帧字段与 04_api.md §2.2 一致
func TestMetaFrameFields(t *testing.T) {
	tid := frame.MustNewTransferID()
	meta := &frame.MetaData{
		Name:        "test.bin",
		Size:        500,
		BlockSize:   100,
		BlockCount:  5,
		HashAlgo:    "sha256",
		Hash:        "abc",
		FEC:         "none",
		Redundancy:  0,
		PayloadType: "text",
		MimeType:    "text/plain",
	}

	f, err := frame.BuildMetaFrame(tid, meta)
	if err != nil {
		t.Fatalf("BuildMetaFrame 失败: %v", err)
	}

	// 验证帧头字段
	if f.Header.Version != frame.Version {
		t.Errorf("Version 应为 0x01")
	}
	if f.Header.Type != frame.TypeMeta {
		t.Errorf("Type 应为 0x01 (元数据帧)")
	}
	if f.Header.Seq != 0 {
		t.Errorf("元数据帧 Seq 应为 0")
	}
	if f.Header.Len != uint32(len(f.Payload)) {
		t.Errorf("Len 应与 payload 长度一致")
	}

	// 验证 flags: text 且 fec=none
	expectedFlags := frame.SetFlags("text", false)
	if f.Header.Flags != expectedFlags {
		t.Errorf("Flags 不匹配: got 0x%04X, want 0x%04X", f.Header.Flags, expectedFlags)
	}
}

// TestDataFrameFlags 验证数据帧 flags
func TestDataFrameFlags(t *testing.T) {
	tid := frame.MustNewTransferID()

	// file + FEC
	f1 := frame.BuildDataFrame(tid, 1, []byte("data"), "file", true)
	expectedFlags1 := frame.SetFlags("file", true)
	if f1.Header.Flags != expectedFlags1 {
		t.Errorf("file+FEC flags 不匹配: got 0x%04X, want 0x%04X", f1.Header.Flags, expectedFlags1)
	}

	// text + no FEC
	f2 := frame.BuildDataFrame(tid, 2, []byte("data"), "text", false)
	expectedFlags2 := frame.SetFlags("text", false)
	if f2.Header.Flags != expectedFlags2 {
		t.Errorf("text+noFEC flags 不匹配: got 0x%04X, want 0x%04X", f2.Header.Flags, expectedFlags2)
	}
}

// TestInvalidLength 测试 payload 长度与实际数据不符
func TestInvalidLength(t *testing.T) {
	tid := frame.MustNewTransferID()
	f := frame.BuildDataFrame(tid, 1, []byte("hello"), "file", false)
	raw := frame.MarshalFrame(f)

	// 截断数据（缺少部分 payload + CRC32）
	truncated := raw[:len(raw)-3]
	_, err := frame.UnmarshalFrame(truncated)
	if err == nil {
		t.Fatal("截断数据应导致拆帧失败")
	}
}

// TestNewTransferID 测试 transfer_id 生成唯一性
func TestNewTransferID(t *testing.T) {
	id1, err := frame.NewTransferID()
	if err != nil {
		t.Fatalf("NewTransferID 失败: %v", err)
	}
	id2, err := frame.NewTransferID()
	if err != nil {
		t.Fatalf("NewTransferID 失败: %v", err)
	}

	if id1 == id2 {
		t.Error("两次生成的 transfer_id 不应相同")
	}
}