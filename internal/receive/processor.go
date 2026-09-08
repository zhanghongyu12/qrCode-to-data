package receive

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"qrcd/internal/fec"
	"qrcd/internal/frame"
	"qrcd/internal/payload"
	"qrcd/internal/progress"
)

// 错误分类哨兵（供 cmd 层映射退出码）
var (
	// ErrUsage 参数错误（退出码 2）
	ErrUsage = errors.New("receive: 参数错误")
	// ErrEnv 环境错误（退出码 3，如摄像头不可用、输出不可写）
	ErrEnv = errors.New("receive: 环境错误")
	// ErrTransfer 交换/解码失败（退出码 1，如校验失败、符号不足、超时）
	ErrTransfer = errors.New("receive: 交换失败")
	// ErrInterrupted 用户中断（Ctrl+C）
	ErrInterrupted = errors.New("receive: 还原已停止")
)

// Result 还原结果。
type Result struct {
	OutputPath string
	Name       string
	Size       int64
	SHA256     string
}

// bufSym 元数据帧到达前缓存的编码符号。
type bufSym struct {
	id   uint32
	data []byte
}

// fecState 单个交换会话的 FEC 解码状态。
type fecState struct {
	dec      fec.Decoder
	buffered []bufSym
	done     bool
	decoded  []byte
}

// ensureDecoder 依据元数据创建 FEC 解码器（messageLength = meta.Size）。
func (fs *fecState) ensureDecoder(meta *frame.MetaData) error {
	if fs.dec != nil {
		return nil
	}
	if meta.Size == 0 {
		// 空载荷：无需符号即可完成
		fs.done = true
		fs.decoded = []byte{}
		return nil
	}
	k := meta.BlockCount
	if k < 1 {
		k = 1
	}
	codec, err := fec.NewCodec(meta.FEC, k)
	if err != nil {
		return fmt.Errorf("receive: 创建 %q 解码器失败: %w", meta.FEC, err)
	}
	// gofountain 要求 messageLength 为块数的整数倍（播放端已补齐），
	// 否则源码块不等长（padding）会导致解码结果错误。
	msgLen := int(meta.Size)
	if meta.FEC == "raptor" || meta.FEC == "lt" {
		symLen := (int(meta.Size) + k - 1) / k
		if symLen < 1 {
			symLen = 1
		}
		msgLen = k * symLen
	}
	fs.dec = codec.NewDecoder(k, msgLen)
	return nil
}

// flush 元数据到达后投喂缓存符号。
func (fs *fecState) flush() error {
	if fs.dec == nil || fs.done {
		return nil
	}
	for _, b := range fs.buffered {
		done, err := fs.dec.AddSymbol(b.id, b.data)
		if err != nil {
			continue
		}
		if done {
			return fs.finishDecode()
		}
	}
	fs.buffered = nil
	return nil
}

// add 投喂一个编码符号。
func (fs *fecState) add(id uint32, data []byte) error {
	if fs.dec == nil || fs.done {
		return nil
	}
	done, err := fs.dec.AddSymbol(id, data)
	if err != nil {
		return nil
	}
	if done {
		return fs.finishDecode()
	}
	return nil
}

func (fs *fecState) finishDecode() error {
	decoded, err := fs.dec.Decode()
	if err != nil {
		return nil
	}
	fs.decoded = decoded
	fs.done = true
	return nil
}

// partAssembly 多会话分片交换的整体聚合状态（DEC-012）。
// 各分片以独立 transfer_id/独立会话到达，FEC 各自完成后在此按 overallHash 聚合；
// 收集齐 0..PartTotal-1 全部分片后拼接、整体校验、落盘。
type partAssembly struct {
	overallHash string
	partTotal   int
	overallName string // 仅 part 0 携带；其余分片到达时若 part 0 未到则为空
	overallSize int64  // 同上
	parts       map[int][]byte // partIndex → 解码后的分片数据
	got         int            // 已收集分片数
	done        bool
}

// Processor 还原帧处理器（白盒可测核心）：
// 逐帧还原字节 → 拆帧（CRC/去重/元数据初始化）→ FEC 解码 → 校验 → 落盘。
type Processor struct {
	opts       Options
	sm         *frame.SessionManager
	fecStates  map[[16]byte]*fecState
	assemblies map[string]*partAssembly // overallHash → 分片聚合
	prog       *progress.Terminal
	monitor    *progress.Monitor
	dedupTotal int
	unique     int
	expected   int // 期望块数（来自元数据 BlockCount/TotalSymbols）
	result     *Result
	done       bool
	mu         sync.Mutex
}

// NewProcessor 创建还原帧处理器。
func NewProcessor(opts Options) (*Processor, error) {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.ProgressOut == nil {
		opts.ProgressOut = io.Discard
	}
	return &Processor{
		opts:       opts,
		sm:         frame.NewSessionManager(),
		fecStates:  make(map[[16]byte]*fecState),
		assemblies: make(map[string]*partAssembly),
		prog:       progress.NewTerminal(opts.ProgressOut, opts.Quiet),
		monitor:    progress.NewMonitor(opts.Timeout),
	}, nil
}

// Process 处理一帧字节。校验失败丢帧（返回 nil）；校验/校验失败等致命错误返回 error。
func (p *Processor) Process(frameBytes []byte) error {
	f, err := frame.UnmarshalFrame(frameBytes)
	if err != nil {
		// CRC/magic 校验失败：丢弃该帧
		return nil
	}

	p.monitor.Notify()
	id := f.Header.TransferID
	sess := p.sm.GetOrCreate(id)
	fs := p.fecStates[id]
	if fs == nil {
		fs = &fecState{}
		p.fecStates[id] = fs
	}

	if f.IsMeta() {
		meta, err := f.ParseMeta()
		if err != nil {
			return nil
		}
		if err := p.checkMeta(meta); err != nil {
			return err
		}
		sess.SetMeta(meta)
		// 进度基准：播放端计划播放的编码符号总数（含冗余），使还原端"已收/总数"
		// 与播放端、手机端的计数尽量对齐。旧版本用 BlockCount（K，源块数），
		// 导致喷泉码凑齐 K 即完成、计数停在 K，与播放端/手机端对不上。
		if meta.TotalSymbols > 0 {
			p.expected = meta.TotalSymbols
		} else {
			p.expected = meta.BlockCount
		}
		if err := fs.ensureDecoder(meta); err != nil {
			return fmt.Errorf("%w: %v", ErrTransfer, err)
		}
		if err := fs.flush(); err != nil {
			return err
		}
		if fs.done {
			return p.complete(id, fs, meta)
		}
		return nil
	}

	// 数据帧：按 seq 去重。即使已还原完成（done），仍继续去重计数，
	// 使还原端"已收"能追上手机实际扫描到的帧数，三端数字尽量一致。
	if !sess.MarkReceived(f.Header.Seq) {
		p.dedupTotal++
		return nil
	}
	p.unique++

	if p.done {
		// 已还原：不再投喂解码器，仅累计计数
		return nil
	}

	if sess.HasMeta() {
		if err := fs.ensureDecoder(sess.Meta); err != nil {
			return fmt.Errorf("%w: %v", ErrTransfer, err)
		}
		if err := fs.add(f.Header.Seq, f.Payload); err != nil {
			return err
		}
		if fs.done {
			return p.complete(id, fs, sess.Meta)
		}
	} else {
		// 元数据帧未到：缓存符号，待元数据到达后再投喂
		fs.buffered = append(fs.buffered, bufSym{id: f.Header.Seq, data: f.Payload})
	}
	return nil
}

// Done 返回是否已成功还原并落盘。
func (p *Processor) Done() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.done
}

// Result 返回最终结果；未完成时返回 ErrTransfer。
func (p *Processor) Result() (*Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done && p.result != nil {
		return p.result, nil
	}
	return nil, fmt.Errorf("%w: 符号不足，未能还原数据，请调整角度重扫或重试", ErrTransfer)
}

// Stats 返回（唯一块数, 期望块数, 去重数）。
func (p *Processor) Stats() (count, total, dedup int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.unique, p.expected, p.dedupTotal
}

// CheckTimeout 检查无新块超时（未完成时）。
func (p *Processor) CheckTimeout() error {
	if p.done {
		return nil
	}
	return p.monitor.Check()
}

// Finish 源耗尽时调用：返回结果或符号不足错误。
func (p *Processor) Finish() (*Result, error) {
	return p.Result()
}

// Close 释放资源。
func (p *Processor) Close() {
	p.prog.Close()
}

// checkMeta 元数据预校验：--expect-size / --hash 提前检查。
// 多会话分片时（PartTotal>0），--expect-size / --hash 针对整体文件（OverallSize/OverallHash）。
func (p *Processor) checkMeta(meta *frame.MetaData) error {
	expectSize := meta.OverallSize
	if expectSize == 0 {
		expectSize = meta.Size
	}
	if p.opts.ExpectSize > 0 && expectSize != p.opts.ExpectSize {
		return fmt.Errorf("%w: 预期大小 %d 与实际 %d 不匹配，请确认播放端", ErrTransfer, p.opts.ExpectSize, expectSize)
	}
	wantHash := meta.OverallHash
	if wantHash == "" {
		wantHash = meta.Hash
	}
	if p.opts.Hash != "" {
		exp := strings.TrimPrefix(p.opts.Hash, "sha256:")
		if exp != "" && exp != wantHash {
			return fmt.Errorf("%w: 预期 SHA-256 与元数据不匹配，请确认播放端", ErrTransfer)
		}
	}
	return nil
}

// complete 校验通过后落盘。多会话分片时聚合分片（DEC-012）。
func (p *Processor) complete(id [16]byte, fs *fecState, meta *frame.MetaData) error {
	if p.done {
		return nil
	}
	data := fs.decoded
	// 播放端补齐到块数整数倍，此处截断到原始大小
	if int64(len(data)) > meta.Size {
		data = data[:int(meta.Size)]
	}

	// 多会话分片：走分片聚合路径，收齐后整体拼接落盘
	if meta.PartTotal > 0 {
		return p.completePart(meta, data)
	}

	if !payload.VerifySHA256(data, meta.Hash) {
		return fmt.Errorf("%w: SHA-256 校验失败，已丢弃损坏数据，请重试", ErrTransfer)
	}
	if p.opts.Hash != "" {
		exp := strings.TrimPrefix(p.opts.Hash, "sha256:")
		if exp != "" && exp != payload.SHA256Hex(data) {
			return fmt.Errorf("%w: 还原结果与 --hash 预期不一致，已丢弃损坏数据", ErrTransfer)
		}
	}

	outPath, err := payload.OutputPath(p.opts.Output, meta.Name)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnv, err)
	}
	if err := payload.WriteFile(outPath, data, payload.WriteOptions{
		Overwrite:    p.opts.Overwrite,
		VerifySHA256: meta.Hash,
	}); err != nil {
		return fmt.Errorf("%w: %v", ErrEnv, err)
	}

	p.result = &Result{
		OutputPath: outPath,
		Name:       meta.Name,
		Size:       int64(len(data)),
		SHA256:     payload.SHA256Hex(data),
	}
	p.done = true

	p.prog.Finish(progress.Event{
		Kind:  progress.Receive,
		Count: p.unique,
		// 进度基准与 Process 中一致：用 TotalSymbols（含冗余），而非 K。
		Total: func() int {
			if meta.TotalSymbols > 0 {
				return meta.TotalSymbols
			}
			return meta.BlockCount
		}(),
		Dedup:   p.dedupTotal,
		Done:    true,
		Message: fmt.Sprintf("还原完成: %s (%d 字节) 校验通过", outPath, len(data)),
	})
	return nil
}

// completePart 多会话分片完成路径（DEC-012）：
// 分片级 SHA-256 校验 → 按 overallHash 聚合缓存 → 收齐 0..PartTotal-1 后拼接 →
// 整体 SHA-256 校验 → 以整体文件名落盘。
func (p *Processor) completePart(meta *frame.MetaData, data []byte) error {
	if !payload.VerifySHA256(data, meta.Hash) {
		return fmt.Errorf("%w: 分片 %d/%d SHA-256 校验失败，已丢弃该分片数据，请重试",
			ErrTransfer, meta.PartIndex+1, meta.PartTotal)
	}

	key := meta.OverallHash
	if key == "" {
		return fmt.Errorf("%w: 分片缺少 overallHash 关联键，无法聚合（播放端版本过低？）", ErrTransfer)
	}
	as := p.assemblies[key]
	if as == nil {
		as = &partAssembly{
			overallHash: meta.OverallHash,
			partTotal:   meta.PartTotal,
			parts:       make(map[int][]byte, meta.PartTotal),
		}
		p.assemblies[key] = as
	}
	// part 0 或任何分片携带的整体信息都补全（防御：part 0 未先到达）
	if meta.OverallName != "" {
		as.overallName = meta.OverallName
	}
	if meta.OverallSize > 0 {
		as.overallSize = meta.OverallSize
	}
	if as.done {
		return nil
	}
	if _, exists := as.parts[meta.PartIndex]; exists {
		return nil // 重复分片，忽略
	}
	as.parts[meta.PartIndex] = append([]byte(nil), data...)
	as.got++

	p.prog.Report(progress.Event{
		Kind:  progress.Receive,
		Count: as.got,
		Total: as.partTotal,
		Dedup: p.dedupTotal,
	})
	if as.got < as.partTotal {
		p.prog.Finish(progress.Event{
			Kind:  progress.Receive,
			Count: as.got,
			Total: as.partTotal,
			Dedup: p.dedupTotal,
			Message: fmt.Sprintf("分片 %d/%d 完成，继续等待剩余分片…", as.got, as.partTotal),
		})
		return nil
	}
	return p.spliceAssembly(as)
}

// spliceAssembly 收齐全部分片后拼接、整体校验、落盘。
func (p *Processor) spliceAssembly(as *partAssembly) error {
	// 按 partIndex 顺序拼接
	full := make([]byte, 0, as.overallSize)
	for i := 0; i < as.partTotal; i++ {
		part, ok := as.parts[i]
		if !ok {
			return fmt.Errorf("%w: 分片 %d/%d 缺失，无法拼接", ErrTransfer, i+1, as.partTotal)
		}
		full = append(full, part...)
	}
	if as.overallSize > 0 && int64(len(full)) != as.overallSize {
		return fmt.Errorf("%w: 拼接长度 %d 与整体大小 %d 不一致，请重试", ErrTransfer, len(full), as.overallSize)
	}
	if !payload.VerifySHA256(full, as.overallHash) {
		return fmt.Errorf("%w: 整体 SHA-256 校验失败，拼接数据损坏，请重试", ErrTransfer)
	}

	name := as.overallName
	if name == "" {
		name = fmt.Sprintf("%s.assembled", as.overallHash[:12])
	}
	outPath, err := payload.OutputPath(p.opts.Output, name)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEnv, err)
	}
	if err := payload.WriteFile(outPath, full, payload.WriteOptions{
		Overwrite:    p.opts.Overwrite,
		VerifySHA256: as.overallHash,
	}); err != nil {
		return fmt.Errorf("%w: %v", ErrEnv, err)
	}

	p.result = &Result{
		OutputPath: outPath,
		Name:       name,
		Size:       int64(len(full)),
		SHA256:     as.overallHash,
	}
	p.done = true
	as.done = true

	p.prog.Finish(progress.Event{
		Kind:  progress.Receive,
		Count: p.unique,
		Total: p.expected,
		Dedup: p.dedupTotal,
		Done:  true,
		Message: fmt.Sprintf("还原完成: %s (%d 字节) 多分片拼接校验通过", outPath, len(full)),
	})
	return nil
}
