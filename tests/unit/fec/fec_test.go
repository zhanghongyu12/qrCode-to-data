package fec_test

import (
	"bytes"
	"math/rand"
	"testing"

	"qrcd/internal/fec"
)

// raptrSym 编码符号（id + 数据）。
type raptrSym struct {
	id   uint32
	data []byte
}

// TestRaptorRoundtrip 测试 Raptor 乱序 + 去重投喂 ≥ 阈值符号后还原
func TestRaptorRoundtrip(t *testing.T) {
	// 使用能被 sourceSymbols 整除的数据大小，简化测试
	data := []byte("Hello, Raptor Fountain Code! This is a test message for roundtrip verification.")
	sourceSymbols := 8
	blockSize := (len(data) + sourceSymbols - 1) / sourceSymbols

	codec := fec.NewRaptorCodec(sourceSymbols)
	enc, err := codec.NewEncoder(data, blockSize, 0.1)
	if err != nil {
		t.Fatalf("NewEncoder 失败: %v", err)
	}

	// 收集编码符号：预生成批次（K + ceil(red·K)）耗尽后 Raptor 仍可无限产出，
	// 故用固定次数收集（2K 个，覆盖预生成 + 按需生成两条路径）。
	var symbols []raptrSym
	for i := 0; i < sourceSymbols*2; i++ {
		id, s := enc.NextSymbol()
		if s == nil {
			t.Fatalf("NextSymbol 第 %d 次返回 nil（Raptor 应可无限产出冗余符号）", i)
		}
		symbols = append(symbols, raptrSym{id: id, data: s})
	}
	if len(symbols) < sourceSymbols {
		t.Fatalf("编码符号数 %d 少于源符号数 %d", len(symbols), sourceSymbols)
	}

	// 创建解码器
	dec := codec.NewDecoder(sourceSymbols, len(data))

	// 乱序投喂（反序）
	done := false
	for i := len(symbols) - 1; i >= 0 && !done; i-- {
		var err error
		done, err = dec.AddSymbol(symbols[i].id, symbols[i].data)
		if err != nil {
			t.Fatalf("AddSymbol 失败: %v", err)
		}
	}

	if !done {
		t.Fatal("未收到足够符号进行解码")
	}

	decoded, err := dec.Decode()
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	// 验证原始数据与解码数据完全一致
	if !bytes.Equal(decoded, data) {
		t.Errorf("解码结果与原文不匹配\n原始数据长度: %d, 解码数据长度: %d", len(data), len(decoded))
		for i := 0; i < len(data) && i < len(decoded); i++ {
			if data[i] != decoded[i] {
				t.Logf("首个差异字节位置: %d, 原始=0x%02x, 解码=0x%02x", i, data[i], decoded[i])
				t.Logf("原始[%d:]: %x", i, data[i:min(i+16, len(data))])
				t.Logf("解码[%d:]: %x", i, decoded[i:min(i+16, len(decoded))])
				break
			}
		}
	}
}

// TestRaptorPacketLoss 测试 Raptor 丢块场景可还原
// 喷泉码特性：生成远多于源符号数的编码符号，随机丢弃约 20%，
// 剩余互不相同符号数足够时即可还原。使用较大 K 与较长数据，
// 保证解码矩阵约束充分（gofountain 在低 K + 顺序 id 下存在伪满秩局限）。
func TestRaptorPacketLoss(t *testing.T) {
	// 2000 字节数据，确定性填充
	data := make([]byte, 2000)
	for i := range data {
		data[i] = byte(i)
	}
	sourceSymbols := 50
	blockSize := (len(data) + sourceSymbols - 1) / sourceSymbols

	codec := fec.NewRaptorCodec(sourceSymbols)
	// 预生成全部编码符号：redundancy=1.4 → 50 + ceil(70) = 120 个（2.4 倍冗余）
	enc, err := codec.NewEncoder(data, blockSize, 1.4)
	if err != nil {
		t.Fatalf("NewEncoder 失败: %v", err)
	}

	// 生成 120 个符号（2.4 倍冗余）
	totalSymbols := 120
	symbols := make([]raptrSym, 0, totalSymbols)
	for i := 0; i < totalSymbols; i++ {
		id, s := enc.NextSymbol()
		if s == nil {
			t.Fatalf("第 %d 次 NextSymbol 返回 nil（应预生成 120 个符号）", i)
		}
		symbols = append(symbols, raptrSym{id: id, data: s})
	}

	// 确定性随机丢弃约 20%（24 个），保留 96 个
	rng := rand.New(rand.NewSource(42))
	rng.Shuffle(len(symbols), func(i, j int) { symbols[i], symbols[j] = symbols[j], symbols[i] })
	keep := symbols[24:]

	dec := codec.NewDecoder(sourceSymbols, len(data))
	done := false
	for _, s := range keep {
		done, err = dec.AddSymbol(s.id, s.data)
		if err != nil {
			t.Fatalf("AddSymbol 失败: %v", err)
		}
		if done {
			break
		}
	}

	if !done {
		t.Fatal("丢包场景下未收到足够符号进行解码")
	}

	decoded, err := dec.Decode()
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	if !bytes.Equal(decoded, data) {
		t.Errorf("丢包场景解码结果与原文不匹配\n原始长度: %d, 解码长度: %d", len(data), len(decoded))
		for i := 0; i < len(data) && i < len(decoded); i++ {
			if data[i] != decoded[i] {
				t.Logf("首个差异字节位置: %d, 原始=0x%02x, 解码=0x%02x", i, data[i], decoded[i])
				break
			}
		}
	}
}

// TestEmptyPayload 测试空载荷返回空字节不 panic
func TestEmptyPayload(t *testing.T) {
	codec := fec.NewRaptorCodec(4)
	enc, err := codec.NewEncoder([]byte{}, 256, 0.1)
	if err != nil {
		t.Fatalf("空载荷 NewEncoder 失败: %v", err)
	}

	if enc.SourceSymbols() != 0 {
		t.Errorf("空载荷 SourceSymbols 应为 0，实际为 %d", enc.SourceSymbols())
	}

	id, sym := enc.NextSymbol()
	if sym != nil {
		t.Errorf("空载荷 NextSymbol 应返回 nil 符号，实际返回 %d 字节", len(sym))
	}
	_ = id
}

// TestSingleBlockNone 测试单块载荷走 none 短路
func TestSingleBlockNone(t *testing.T) {
	data := []byte("single block data")

	codec := fec.NewNoneCodec()
	if codec.Scheme() != "none" {
		t.Errorf("Scheme 应为 none，实际为 %s", codec.Scheme())
	}

	enc, err := codec.NewEncoder(data, 1024, 0)
	if err != nil {
		t.Fatalf("NewEncoder 失败: %v", err)
	}

	if enc.SourceSymbols() != 1 {
		t.Errorf("none encoder SourceSymbols 应为 1，实际为 %d", enc.SourceSymbols())
	}

	id, sym := enc.NextSymbol()
	if id != 0 {
		t.Errorf("none encoder 符号 id 应为 0，实际为 %d", id)
	}
	if !bytes.Equal(sym, data) {
		t.Error("none encoder 符号应与原始数据一致")
	}

	// 第二次调用返回 nil
	_, sym2 := enc.NextSymbol()
	if sym2 != nil {
		t.Error("none encoder 第二次 NextSymbol 应返回 nil")
	}

	// 解码
	dec := codec.NewDecoder(1, len(data))
	done, err := dec.AddSymbol(0, data)
	if err != nil {
		t.Fatalf("none decoder AddSymbol 失败: %v", err)
	}
	if !done {
		t.Fatal("none decoder 应在收到第一个符号后完成")
	}

	decoded, err := dec.Decode()
	if err != nil {
		t.Fatalf("none decoder Decode 失败: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Error("none decoder 解码结果与原文不一致")
	}
}

// TestScheme 测试 Scheme() 返回正确值
func TestScheme(t *testing.T) {
	if s := fec.NewRaptorCodec(4).Scheme(); s != "raptor" {
		t.Errorf("Raptor Scheme 应为 raptor，实际为 %s", s)
	}
	if s := fec.NewLTCodec(4).Scheme(); s != "lt" {
		t.Errorf("LT Scheme 应为 lt，实际为 %s", s)
	}
	if s := fec.NewNoneCodec().Scheme(); s != "none" {
		t.Errorf("None Scheme 应为 none，实际为 %s", s)
	}
}

// TestNextSymbolInfinite 测试 Raptor NextSymbol 预生成符号序列与无限冗余语义：
//  1. 预生成批次（K + ceil(redundancy·K) 个）内符号非空且 id 从 0 连续递增；
//  2. 超出预生成批次后仍按需产出新符号（可无限产出冗余符号，不返回 nil），
//     与 04_api §3「可无限产出冗余符号」契约一致。发送端靠 totalData 计数终止
//     （见 send/stream.go），不依赖 nil。
func TestNextSymbolInfinite(t *testing.T) {
	data := []byte("test data for infinite symbol generation")
	sourceSymbols := 8
	redundancy := 0.25
	codec := fec.NewRaptorCodec(sourceSymbols)
	enc, err := codec.NewEncoder(data, 16, redundancy)
	if err != nil {
		t.Fatalf("NewEncoder 失败: %v", err)
	}

	// 预生成批次：K + ceil(red·K) = 8 + 2 = 10 个，符号非空且 id 连续递增
	pregen := sourceSymbols + int(ceil(redundancy*float64(sourceSymbols)))
	for i := 0; i < pregen; i++ {
		id, sym := enc.NextSymbol()
		if sym == nil || len(sym) == 0 {
			t.Fatalf("NextSymbol 第 %d 次（共预生成 %d）应返回非空符号", i, pregen)
		}
		if id != uint32(i) {
			t.Errorf("NextSymbol 第 %d 次 id=%d，应从 0 连续递增", i, id)
		}
	}
	// 超出预生成批次：继续按需产出新符号（无限冗余），不返回 nil
	for i := 0; i < 8; i++ {
		id, sym := enc.NextSymbol()
		if sym == nil || len(sym) == 0 {
			t.Fatalf("超出预生成批次后 NextSymbol 第 %d 次返回 nil（应可无限产出冗余符号）", i)
		}
		if id != uint32(pregen+i) {
			t.Errorf("按需生成符号 id 应为 %d，实际 %d", pregen+i, id)
		}
	}
}

// ceil 整数上取整（测试辅助，避免引入 math 包）。
func ceil(f float64) int {
	i := int(f)
	if float64(i) < f {
		return i + 1
	}
	return i
}

// TestReceived 测试 Received() 正确统计不重复符号数
func TestReceived(t *testing.T) {
	data := []byte("received count test data for fountain code")
	sourceSymbols := 4
	codec := fec.NewRaptorCodec(sourceSymbols)
	enc, err := codec.NewEncoder(data, 16, 0.1)
	if err != nil {
		t.Fatalf("NewEncoder 失败: %v", err)
	}

	dec := codec.NewDecoder(sourceSymbols, len(data))

	// 生成 3 个符号
	symbols := make(map[uint32][]byte)
	for i := 0; i < 3; i++ {
		id, sym := enc.NextSymbol()
		symbols[id] = sym
	}

	// 投喂 3 个不同符号
	for id, sym := range symbols {
		dec.AddSymbol(id, sym)
	}

	if dec.Received() != 3 {
		t.Errorf("Received 应为 3，实际为 %d", dec.Received())
	}

	// 重复投喂第一个符号
	for id := range symbols {
		dec.AddSymbol(id, symbols[id])
		break
	}

	if dec.Received() != 3 {
		t.Errorf("重复投喂后 Received 仍应为 3，实际为 %d", dec.Received())
	}
}

// TestLTRoundtrip 测试 LT 码编解码闭环
func TestLTRoundtrip(t *testing.T) {
	data := []byte("LT fountain code roundtrip test data for verification")
	sourceSymbols := 6
	blockSize := (len(data) + sourceSymbols - 1) / sourceSymbols

	codec := fec.NewLTCodec(sourceSymbols)
	enc, err := codec.NewEncoder(data, blockSize, 0.1)
	if err != nil {
		t.Fatalf("LT NewEncoder 失败: %v", err)
	}

	// 收集符号
	dec := codec.NewDecoder(sourceSymbols, len(data))
	done := false
	for i := 0; i < sourceSymbols*3; i++ {
		id, sym := enc.NextSymbol()
		var err error
		done, err = dec.AddSymbol(id, sym)
		if err != nil {
			t.Fatalf("LT AddSymbol 失败: %v", err)
		}
		if done {
			break
		}
	}

	if !done {
		t.Fatal("LT 未收到足够符号")
	}

	decoded, err := dec.Decode()
	if err != nil {
		t.Fatalf("LT Decode 失败: %v", err)
	}

	if !bytes.Equal(decoded, data) {
		t.Errorf("LT 解码结果与原文不匹配\n原始长度: %d, 解码长度: %d", len(data), len(decoded))
	}
}

// TestNewCodec 测试 NewCodec 工厂函数
func TestNewCodec(t *testing.T) {
	c, err := fec.NewCodec("raptor", 4)
	if err != nil {
		t.Fatalf("NewCodec raptor 失败: %v", err)
	}
	if c.Scheme() != "raptor" {
		t.Errorf("Scheme 应为 raptor")
	}

	c, err = fec.NewCodec("lt", 4)
	if err != nil {
		t.Fatalf("NewCodec lt 失败: %v", err)
	}
	if c.Scheme() != "lt" {
		t.Errorf("Scheme 应为 lt")
	}

	c, err = fec.NewCodec("none", 1)
	if err != nil {
		t.Fatalf("NewCodec none 失败: %v", err)
	}
	if c.Scheme() != "none" {
		t.Errorf("Scheme 应为 none")
	}

	_, err = fec.NewCodec("invalid", 4)
	if err == nil {
		t.Fatal("NewCodec invalid 应返回错误")
	}
}

// TestSourceSymbols 测试 SourceSymbols 返回值
func TestSourceSymbols(t *testing.T) {
	data := []byte("source symbols test")
	codec := fec.NewRaptorCodec(8)
	enc, err := codec.NewEncoder(data, 4, 0.1)
	if err != nil {
		t.Fatalf("NewEncoder 失败: %v", err)
	}
	if enc.SourceSymbols() != 8 {
		t.Errorf("SourceSymbols 应为 8，实际为 %d", enc.SourceSymbols())
	}
}

// TestDecodeNotReady 测试符号不足时 Decode 返回错误
func TestDecodeNotReady(t *testing.T) {
	codec := fec.NewRaptorCodec(8)
	dec := codec.NewDecoder(8, 100)

	_, err := dec.Decode()
	if err == nil {
		t.Fatal("符号不足时 Decode 应返回错误")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
