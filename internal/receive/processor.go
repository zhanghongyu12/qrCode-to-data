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
	// ErrTransfer 传输/解码失败（退出码 1，如校验失败、符号不足、超时）
	ErrTransfer = errors.New("receive: 传输失败")
	// ErrInterrupted 用户中断（Ctrl+C）
	ErrInterrupted = errors.New("receive: 接收已停止")
)

// Result 接收结果。
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

// fecState 单个传输会话的 FEC 解码状态。
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
	// gofountain 要求 messageLength 为块数的整数倍（发送端已补齐），
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

// Processor 接收帧处理器（白盒可测核心）：
// 逐帧接收字节 → 拆帧（CRC/去重/元数据初始化）→ FEC 解码 → 校验 → 落盘。
type Processor struct {
	opts       Options
	sm         *frame.SessionManager
	fecStates  map[[16]byte]*fecState
	prog       *progress.Terminal
	monitor    *progress.Monitor
	dedupTotal int
	unique     int
	expected   int // 期望块数（来自元数据 BlockCount）
	result     *Result
	done       bool
	mu         sync.Mutex
}

// NewProcessor 创建接收帧处理器。
func NewProcessor(opts Options) (*Processor, error) {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.ProgressOut == nil {
		opts.ProgressOut = io.Discard
	}
	return &Processor{
		opts:      opts,
		sm:        frame.NewSessionManager(),
		fecStates: make(map[[16]byte]*fecState),
		prog:      progress.NewTerminal(opts.ProgressOut, opts.Quiet),
		monitor:   progress.NewMonitor(opts.Timeout),
	}, nil
}

// Process 处理一帧字节。校验失败丢帧（返回 nil）；校验/校验失败等致命错误返回 error。
func (p *Processor) Process(frameBytes []byte) error {
	if p.done {
		return nil
	}

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
		p.expected = meta.BlockCount
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

	// 数据帧：按 seq 去重
	if !sess.MarkReceived(f.Header.Seq) {
		p.dedupTotal++
		return nil
	}
	p.unique++

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
	return nil, fmt.Errorf("%w: 符号不足，未能还原数据，请调整角度重扫或重新发送", ErrTransfer)
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
func (p *Processor) checkMeta(meta *frame.MetaData) error {
	if p.opts.ExpectSize > 0 && meta.Size != p.opts.ExpectSize {
		return fmt.Errorf("%w: 预期大小 %d 与实际 %d 不匹配，请确认发送端", ErrTransfer, p.opts.ExpectSize, meta.Size)
	}
	if p.opts.Hash != "" {
		exp := strings.TrimPrefix(p.opts.Hash, "sha256:")
		if exp != "" && exp != meta.Hash {
			return fmt.Errorf("%w: 预期 SHA-256 与元数据不匹配，请确认发送端", ErrTransfer)
		}
	}
	return nil
}

// complete 校验通过后落盘。
func (p *Processor) complete(id [16]byte, fs *fecState, meta *frame.MetaData) error {
	if p.done {
		return nil
	}
	data := fs.decoded
	// 发送端补齐到块数整数倍，此处截断到原始大小
	if int64(len(data)) > meta.Size {
		data = data[:int(meta.Size)]
	}

	if !payload.VerifySHA256(data, meta.Hash) {
		return fmt.Errorf("%w: SHA-256 校验失败，已丢弃损坏数据，请重新发送", ErrTransfer)
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
		Kind:    progress.Receive,
		Count:   p.unique,
		Total:   meta.BlockCount,
		Dedup:   p.dedupTotal,
		Done:    true,
		Message: fmt.Sprintf("接收完成: %s (%d 字节) 校验通过", outPath, len(data)),
	})
	return nil
}
