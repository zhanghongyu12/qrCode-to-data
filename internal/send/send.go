package send

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"qrcd/internal/payload"
	"qrcd/internal/progress"
	"qrcd/internal/qrcode"
)

// 错误分类哨兵（供 cmd 层映射退出码）
var (
	// ErrUsage 参数错误（退出码 2）
	ErrUsage = errors.New("send: 参数错误")
	// ErrEnv 环境错误（退出码 3，如文件不可读）
	ErrEnv = errors.New("send: 环境错误")
	// ErrInterrupted 用户中断（Ctrl+C）
	ErrInterrupted = errors.New("send: 传输已停止")
)

// Options 发送编排选项，字段与 docs/04_api.md §1.3 对齐。
type Options struct {
	Text string // 直接发送文本（与 File 二选一）
	File string // 文件路径（与 Text 二选一）

	FPS        int     // 二维码播放帧率（帧/秒），默认 10
	Version    int     // QR 版本上限（1~40），默认 20
	ECC        string  // 纠错级别 L/M/Q/H，默认 L
	Redundancy float64 // 喷泉码冗余度（0~1），默认 0.1
	BlockSize  int     // 源分块大小（字节），默认 1024
	MaxSymbol  int     // 单符号字节上限（0=按 Version 容量自适应）。限制后符号更小、QR 更稀疏易扫
	Terminal   string  // 渲染器：ansi / ascii，默认 ansi
	Invert     bool    // 反色
	Quiet      bool    // 关闭进度，只输出最终结果
	Net        string  // 同网直传 auto/on/off（Phase 2，当前仅 off）
	Addr       string  // 指定监听 IP（Phase 2）

	Out         io.Writer // 二维码渲染输出（默认 os.Stdout）
	ProgressOut io.Writer // 进度/结果输出（默认 os.Stdout）
}

// Result 发送结果。
type Result struct {
	Name        string
	Size        int64
	SHA256      string
	PayloadType string
	Frames      int // 播放的数据帧数（短文本直传为 0）
	ShortText   bool
}

// Send 执行发送编排。
func Send(ctx context.Context, opts Options) (*Result, error) {
	opts = normalize(opts)

	if opts.Net != "" && opts.Net != "off" {
		fmt.Fprintln(os.Stderr, "提示: 同网直传（--net）为 Phase 2 功能，尚未实现，本次按纯光学继续。")
	}

	load, err := loadPayload(opts)
	if err != nil {
		return nil, err
	}

	// F-05 短文本直传：≤200 字节静态标准文本二维码，不组帧不启动动画
	if load.PayloadType == "text" && len(load.Data) <= qrcode.ShortTextLimit {
		if opts.Out != nil {
			fmt.Fprintln(opts.Out, "短文本直传（≤200 字节），显示标准文本二维码（第三方 App 可扫读）")
		}
		return sendShortText(opts, load)
	}
	if load.PayloadType == "text" {
		fmt.Fprintln(os.Stderr, "提示: 文本超过 200 字节，自动切换为二维码帧流传输。")
	}

	streams, err := BuildSessionStreams(load, opts)
	if err != nil {
		return nil, err
	}

	if len(streams) == 1 {
		// 单会话：保持原行为与提示
		estSec := float64(streams[0].TotalData()) / float64(opts.FPS)
		if estSec > 30 {
			fmt.Fprintf(os.Stderr, "提示: 预计播放约 %.0f 秒（%d 帧），如需加速可提高 --fps、增大 --version 或减小 --block-size；同网直传将在 Phase 2 提供。\n",
				estSec, streams[0].TotalData())
		}
		return play(ctx, streams[0], opts, "")
	}

	// 多会话分片（DEC-012）：大文件自动拆为 N 个独立传输会话，逐会话播放
	totalData := 0
	for _, st := range streams {
		totalData += st.TotalData()
	}
	estSec := float64(totalData) / float64(opts.FPS)
	if estSec > 30 {
		fmt.Fprintf(os.Stderr, "提示: 数据较大，已自动拆分为 %d 个会话发送（各会话源块 ≤ %d），预计播放约 %.0f 秒（%d 帧）。可提高 --fps 或 --max-symbol 加速。\n",
			len(streams), maxSourceK, estSec, totalData)
	}
	return playSessions(ctx, streams, opts)
}

// loadPayload 读取文件或文本载荷。
func loadPayload(opts Options) (*payload.Load, error) {
	switch {
	case opts.Text != "" && opts.File != "":
		return nil, fmt.Errorf("%w: --text 与 <file> 二选一，不能同时指定", ErrUsage)
	case opts.Text != "":
		return payload.ReadText(opts.Text)
	case opts.File != "":
		load, err := payload.ReadFile(opts.File)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrEnv, err)
		}
		return load, nil
	default:
		return nil, fmt.Errorf("%w: 请指定 <file> 或 --text", ErrUsage)
	}
}

// sendShortText 短文本直传：静态显示标准文本二维码。
func sendShortText(opts Options, load *payload.Load) (*Result, error) {
	code, err := qrcode.EncodeText(string(load.Data))
	if err != nil {
		return nil, fmt.Errorf("send: 短文本二维码生成失败: %w", err)
	}

	var render string
	if opts.Terminal == "ascii" {
		render = qrcode.RenderASCII(code, opts.Invert)
	} else {
		render = qrcode.RenderANSI(code, opts.Invert)
	}

	if opts.Out != nil {
		fmt.Fprint(opts.Out, qrcode.ANSICursorHide)
		fmt.Fprint(opts.Out, qrcode.ANSICursorHome)
		fmt.Fprint(opts.Out, render)
		fmt.Fprint(opts.Out, qrcode.ANSICursorShow)
	}

	if opts.ProgressOut != nil {
		fmt.Fprintf(opts.ProgressOut, "SHA-256: %s\n", payload.SHA256Hex(load.Data))
	}

	return &Result{
		Name:        load.Name,
		Size:        int64(len(load.Data)),
		SHA256:      payload.SHA256Hex(load.Data),
		PayloadType: load.PayloadType,
		ShortText:   true,
	}, nil
}

// play 逐帧编码 QR 并渲染播放。
// label 非空时用于多会话分片（如 "会话 1/4"），完成消息带会话前缀、不重复打印分片 SHA-256；
// 单会话传 "" 保持原输出。
func play(ctx context.Context, stream *Stream, opts Options, label string) (*Result, error) {
	ph := progress.NewTerminal(opts.ProgressOut, opts.Quiet)
	defer ph.Close()

	var ticker *time.Ticker
	if opts.FPS > 0 {
		ticker = time.NewTicker(time.Second / time.Duration(opts.FPS))
		defer ticker.Stop()
	}

	first := true
	dataCount := 0
	for {
		if err := ctx.Err(); err != nil {
			stopPlayback(opts)
			return nil, ErrInterrupted
		}

		item, ok := stream.Next()
		if !ok {
			break
		}

		code, err := qrcode.Encode(item.Bytes, qrcode.Options{Version: opts.Version, ECC: opts.ECC})
		if err != nil {
			return nil, fmt.Errorf("send: 帧编码失败: %w", err)
		}
		var render string
		if opts.Terminal == "ascii" {
			render = qrcode.RenderASCII(code, opts.Invert)
		} else {
			render = qrcode.RenderANSI(code, opts.Invert)
		}

		if opts.Out != nil {
			if first {
				fmt.Fprint(opts.Out, qrcode.ANSICursorHide)
				first = false
			}
			fmt.Fprint(opts.Out, qrcode.ANSICursorHome)
			fmt.Fprint(opts.Out, render)
		}

		if !item.IsMeta {
			dataCount++
			ph.Report(progress.Event{
				Kind:  progress.Send,
				Count: dataCount,
				Total: stream.TotalData(),
				Done:  dataCount >= stream.TotalData(),
			})
		}

		if ticker != nil {
			select {
			case <-ctx.Done():
				stopPlayback(opts)
				return nil, ErrInterrupted
			case <-ticker.C:
			}
		}
	}

	if opts.Out != nil {
		fmt.Fprint(opts.Out, qrcode.ANSICursorShow)
	}
	msg := fmt.Sprintf("发送完成: %d 帧", dataCount)
	if label != "" {
		msg = fmt.Sprintf("%s 发送完成: %d 帧", label, dataCount)
	}
	ph.Finish(progress.Event{
		Kind:    progress.Send,
		Count:   dataCount,
		Total:   stream.TotalData(),
		Done:    true,
		Message: msg,
	})
	if opts.ProgressOut != nil && label == "" {
		fmt.Fprintf(opts.ProgressOut, "SHA-256: %s\n", stream.Meta().Hash)
	}

	return &Result{
		Name:        stream.Meta().Name,
		Size:        stream.Meta().Size,
		SHA256:      stream.Meta().Hash,
		PayloadType: stream.Meta().PayloadType,
		Frames:      dataCount,
	}, nil
}

// playSessions 多会话分片发送：逐会话播放，返回整体结果（拼接后文件信息）。
func playSessions(ctx context.Context, streams []*Stream, opts Options) (*Result, error) {
	totalFrames := 0
	for i, st := range streams {
		r, err := play(ctx, st, opts, fmt.Sprintf("会话 %d/%d", i+1, len(streams)))
		if err != nil {
			return nil, err
		}
		totalFrames += r.Frames
	}

	m0 := streams[0].Meta()
	res := &Result{
		Name:        m0.OverallName,
		Size:        m0.OverallSize,
		SHA256:      m0.OverallHash,
		PayloadType: m0.PayloadType,
		Frames:      totalFrames,
	}
	if opts.ProgressOut != nil {
		fmt.Fprintf(opts.ProgressOut, "整体 SHA-256: %s\n", res.SHA256)
		fmt.Fprintf(opts.ProgressOut, "传输完成: 共 %d 帧、%d 个会话，接收端将拼接为 %s（%d 字节）\n",
			totalFrames, len(streams), res.Name, res.Size)
	}
	return res, nil
}

func stopPlayback(opts Options) {
	if opts.Out != nil {
		fmt.Fprint(opts.Out, qrcode.ANSICursorShow)
	}
	if opts.ProgressOut != nil {
		fmt.Fprintln(opts.ProgressOut, "传输已停止（Ctrl+C）")
	}
}

// normalize 填充默认值。
func normalize(opts Options) Options {
	if opts.FPS <= 0 {
		opts.FPS = 10
	}
	if opts.Version <= 0 {
		opts.Version = qrcode.DefaultVersion
	}
	if opts.ECC == "" {
		opts.ECC = qrcode.DefaultECC
	}
	if opts.Redundancy < 0 {
		opts.Redundancy = 0
	}
	if opts.BlockSize <= 0 {
		opts.BlockSize = 1024
	}
	if opts.Terminal == "" {
		opts.Terminal = "ansi"
	}
	if opts.Net == "" {
		opts.Net = "auto"
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.ProgressOut == nil {
		opts.ProgressOut = os.Stdout
	}
	return opts
}
