package receive

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"qrcd/internal/capture"
	"qrcd/internal/frame"
	"qrcd/internal/qrcode"
)

// Options 还原编排选项，字段与 docs/04_api.md §1.4 对齐。
type Options struct {
	Output     string // 输出目录或完整文件名（默认当前目录）
	Camera     int    // 摄像头设备号
	Source     string // 输入源 camera/file/stdin
	FilePath   string // --source file 时的图片文件或目录
	Stdin      io.Reader
	FPS        int // 采集/解码帧率上限
	Version    int // 期望 QR 版本上限
	ExpectSize int64
	Hash       string        // sha256:<hex>
	Timeout    time.Duration // 无新符号超时
	Overwrite  bool
	Quiet      bool

	Out         io.Writer // 结果输出（默认 os.Stdout）
	ProgressOut io.Writer // 进度输出（默认 os.Stdout）
}

// Receive 执行还原编排。
func Receive(ctx context.Context, opts Options) (*Result, error) {
	opts = normalize(opts)

	src, err := capture.NewSource(opts.Source, capture.Options{
		CameraID: opts.Camera,
		FilePath: opts.FilePath,
		Stdin:    opts.Stdin,
		MaxFPS:   opts.FPS,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEnv, err)
	}
	defer src.Close()

	proc, err := NewProcessor(opts)
	if err != nil {
		return nil, err
	}
	defer proc.Close()

	// 帧率节流（仅 camera/file 实时源；stdin/测试走快速路径）
	var throttle time.Duration
	if opts.Source != "stdin" && opts.FPS > 0 {
		throttle = time.Second / time.Duration(opts.FPS)
	}
	var lastWarn time.Time

	for {
		if err := ctx.Err(); err != nil {
			if opts.ProgressOut != nil {
				fmt.Fprintln(opts.ProgressOut, "还原已停止（Ctrl+C）")
			}
			return nil, ErrInterrupted
		}

		f, err := src.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrEnv, err)
		}

		var data []byte
		if f.Image != nil {
			data, err = qrcode.DecodeImageBytes(f.Image)
			if err != nil {
				continue // 帧内无二维码，跳过
			}
		} else {
			data = f.Data
		}

		frames, err := splitFrames(data)
		if err != nil {
			continue
		}
		for _, fb := range frames {
			if err := proc.Process(fb); err != nil {
				return nil, err
			}
			if proc.Done() {
				res, err := proc.Result()
				if err != nil {
					return nil, err
				}
				printResult(opts, res)
				return res, nil
			}
		}

		if throttle > 0 {
			select {
			case <-ctx.Done():
				return nil, ErrInterrupted
			case <-time.After(throttle):
			}
		}

		// 无新块超时提示（不自动退出，允许 Ctrl+C 终止）
		if terr := proc.CheckTimeout(); terr != nil && time.Since(lastWarn) > opts.Timeout {
			fmt.Fprintln(os.Stderr, "警告:", terr)
			lastWarn = time.Now()
		}
	}

	// 源耗尽（file/stdin 全部帧已处理）
	res, err := proc.Finish()
	if err != nil {
		return nil, err
	}
	printResult(opts, res)
	return res, nil
}

// splitFrames 将字节流拆分为单个帧字节切片。
// 单帧（image 解码结果）原样返回；stdin 多帧串联字节流按帧头 len 依次拆分。
func splitFrames(data []byte) ([][]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("receive: 空字节")
	}
	// 先尝试整体作为单帧
	if _, err := frame.UnmarshalFrame(data); err == nil {
		return [][]byte{data}, nil
	}

	var out [][]byte
	rest := data
	for len(rest) >= frame.HeaderSize+frame.CRC32Size {
		h, err := frame.UnmarshalHeader(rest[:frame.HeaderSize])
		if err != nil {
			break
		}
		total := frame.HeaderSize + int(h.Len) + frame.CRC32Size
		if total < frame.HeaderSize+frame.CRC32Size || total > len(rest) {
			break
		}
		one := rest[:total]
		if _, err := frame.UnmarshalFrame(one); err != nil {
			break
		}
		out = append(out, one)
		rest = rest[total:]
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("receive: 无法解析帧")
	}
	return out, nil
}

// printResult 输出最终结果到 stdout。
func printResult(opts Options, res *Result) {
	if opts.ProgressOut == nil {
		return
	}
	fmt.Fprintf(opts.ProgressOut, "还原完成: %s\n", res.OutputPath)
	fmt.Fprintf(opts.ProgressOut, "大小: %d 字节\n", res.Size)
	fmt.Fprintf(opts.ProgressOut, "SHA-256: %s\n", res.SHA256)
	fmt.Fprintln(opts.ProgressOut, "校验通过")
}

// normalize 填充默认值。
func normalize(opts Options) Options {
	if opts.Output == "" {
		opts.Output = "."
	}
	if opts.Source == "" {
		opts.Source = "camera"
	}
	if opts.FPS <= 0 {
		opts.FPS = 30
	}
	if opts.Version <= 0 {
		opts.Version = qrcode.DefaultVersion
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.ProgressOut == nil {
		opts.ProgressOut = os.Stdout
	}
	return opts
}
