package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"qrcd/internal/qrcode"
	"qrcd/internal/receive"
	"qrcd/internal/send"
	"qrcd/internal/web"
)

// exitErr 携带退出码的错误（0 成功/1 传输失败/2 参数错误/3 环境错误）。
type exitErr struct {
	code int
	err  error
}

func (e *exitErr) Error() string { return e.err.Error() }
func (e *exitErr) Unwrap() error { return e.err }

func paramErr(format string, a ...interface{}) error {
	return &exitErr{code: 2, err: fmt.Errorf(format, a...)}
}

var rootCmd = &cobra.Command{
	Use:   "qrcd",
	Short: "二维码数据传输工具",
	Long: `qrcd — 通过二维码实现纯光学数据传输的 CLI 工具。

支持发送（send）与接收（receive）两个子命令：
  qrcd send <file>        发送文件
  qrcd send --text <文本>  发送文本
  qrcd receive            接收数据（默认摄像头）

纯光学传输无需网络，通过屏幕二维码与摄像头完成数据交换。
Phase 2 将支持同网直传加速与手机离线中转。`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 不带子命令（如双击运行）：自动起 Web 三端并打开浏览器。
		addr, _ := cmd.Flags().GetString("addr")
		if addr == "" {
			addr = ":8080"
		}
		url := "http://localhost" + addr + "/"
		go openBrowser(url)
		srv := web.NewServer(addr)
		if err := srv.Start(cmd.Context()); err != nil && err != http.ErrServerClosed {
			return &exitErr{code: 3, err: err}
		}
		return nil
	},
}

// openBrowser 跨平台打开默认浏览器。
func openBrowser(url string) {
	time.Sleep(400 * time.Millisecond) // 等服务起来
	switch runtime.GOOS {
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}

func init() {
	// 所有 flag 解析错误统一视为参数错误（退出码 2）
	rootCmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return &exitErr{code: 2, err: err}
	})
	rootCmd.AddCommand(sendCmd())
	rootCmd.AddCommand(receiveCmd())
	rootCmd.AddCommand(webCmd())
	rootCmd.Flags().StringP("addr", "a", ":8080", "Web 服务监听地址（双击运行时生效）")
}

func webCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "web",
		Short: "启动三端 Web 产物（发送端/手机中继/接收端）",
		Long: `启动一个本地 Web 服务，浏览器打开即得三个产物：
  /sender   发送端：本机选文件，屏幕逐帧播放二维码
  /relay    手机中继：扫码存下，切换重放给接收端
  /receiver 接收端：扫码探测与计数

手机与电脑需在同一局域网；手机访问 http://<电脑IP>:<端口>/relay`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			srv := web.NewServer(addr)
			if err := srv.Start(ctx); err != nil && err != http.ErrServerClosed {
				return &exitErr{code: 3, err: err}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&addr, "addr", "a", ":8080", "监听地址（如 :8080）")
	return cmd
}

func sendCmd() *cobra.Command {
	var opts send.Options
	cmd := &cobra.Command{
		Use:   "send <file>",
		Short: "发送文件或文本",
		Long: `通过二维码流发送文件或文本。

示例：
  qrcd send ./backup.tar.gz
  qrcd send --text "Hello World"

发送端在终端逐帧播放二维码；接收端用摄像头（或 --source file|stdin）扫描还原。`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return paramErr("send: 最多接受一个 <file> 参数，实际 %d 个", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.File = args[0]
			}
			if err := validateSendOpts(&opts); err != nil {
				_ = cmd.Usage()
				return err
			}

			ctx := cmd.Context()
			if _, err := send.Send(ctx, opts); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.Text, "text", "t", "", "直接发送文本（与 <file> 二选一）")
	cmd.Flags().IntVar(&opts.FPS, "fps", 10, "二维码播放帧率（帧/秒）")
	cmd.Flags().IntVarP(&opts.Version, "version", "v", 20, "QR 版本上限（1~40），载荷不足自动降级")
	cmd.Flags().StringVar(&opts.ECC, "ecc", "L", "QR 纠错级别 L/M/Q/H")
	cmd.Flags().Float64VarP(&opts.Redundancy, "redundancy", "r", 0.1, "喷泉码冗余度（0~1，0.1=多 10% 编码符号）")
	cmd.Flags().IntVarP(&opts.BlockSize, "block-size", "b", 1024, "源分块大小（字节）")
	cmd.Flags().StringVar(&opts.Terminal, "terminal", "ansi", "渲染器：ansi（终端半块）/ ascii（降级）")
	cmd.Flags().BoolVar(&opts.Invert, "invert", false, "反色（浅色终端背景）")
	cmd.Flags().BoolVarP(&opts.Quiet, "quiet", "q", false, "关闭进度，只输出最终结果")
	cmd.Flags().StringVar(&opts.Net, "net", "auto", "同网直传：auto/on/off（Phase 2 未实现）")
	cmd.Flags().StringVar(&opts.Addr, "addr", "", "指定监听 IP（同网直传，Phase 2）")

	return cmd
}

func receiveCmd() *cobra.Command {
	var opts receive.Options
	cmd := &cobra.Command{
		Use:   "receive [路径]",
		Short: "接收数据",
		Long: `通过摄像头（默认）或文件/标准输入接收二维码流并还原数据。

示例：
  qrcd receive
  qrcd receive --output ./out/
  qrcd receive --source file ./frames/ --output ./out/

位置参数 [路径] 仅在 --source file 时指定：为二维码图片文件或含图片的目录。`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return paramErr("receive: 最多接受一个 [路径] 参数，实际 %d 个", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.FilePath = args[0]
			}
			if err := validateReceiveOpts(&opts); err != nil {
				_ = cmd.Usage()
				return err
			}

			ctx := cmd.Context()
			if _, err := receive.Receive(ctx, opts); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.Output, "output", "o", ".", "输出目录或完整文件名")
	cmd.Flags().IntVar(&opts.Camera, "camera", 0, "摄像头设备号")
	cmd.Flags().StringVar(&opts.Source, "source", "camera", "输入源：camera/file/stdin（无摄像头兜底）")
	cmd.Flags().IntVar(&opts.FPS, "fps", 30, "采集/解码帧率上限")
	cmd.Flags().IntVarP(&opts.Version, "version", "v", 20, "期望 QR 版本上限")
	cmd.Flags().Int64Var(&opts.ExpectSize, "expect-size", 0, "预期总字节数（可选，提前校验）")
	cmd.Flags().StringVar(&opts.Hash, "hash", "", "预期校验值，格式 sha256:<hex>")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", 30*time.Second, "无新符号超时（如 30s，提示卡死并允许终止）")
	cmd.Flags().BoolVar(&opts.Overwrite, "overwrite", false, "允许覆盖已存在文件")
	cmd.Flags().BoolVarP(&opts.Quiet, "quiet", "q", false, "关闭进度，只输出最终结果")

	return cmd
}

// validateSendOpts 校验 send 参数，非法返回退出码 2。
func validateSendOpts(o *send.Options) error {
	switch {
	case o.File == "" && o.Text == "":
		return paramErr("send: 请指定 <file> 或 --text（二选一）")
	case o.File != "" && o.Text != "":
		return paramErr("send: <file> 与 --text 二选一，不能同时指定")
	}
	if o.FPS <= 0 {
		return paramErr("send: --fps 必须 ≥ 1，实际 %d", o.FPS)
	}
	if _, err := qrcode.Validate(qrcode.Options{Version: o.Version, ECC: o.ECC}); err != nil {
		return paramErr("send: %v", err)
	}
	if o.Redundancy < 0 || o.Redundancy > 1 {
		return paramErr("send: --redundancy 必须在 [0,1] 区间，实际 %v", o.Redundancy)
	}
	if o.BlockSize < 1 {
		return paramErr("send: --block-size 必须 ≥ 1，实际 %d", o.BlockSize)
	}
	switch o.Terminal {
	case "ansi", "ascii":
	case "window":
		return paramErr("send: --terminal window 为 Phase 2 功能，尚未实现，当前支持 ansi/ascii")
	default:
		return paramErr("send: --terminal 仅支持 ansi/ascii，实际 %q", o.Terminal)
	}
	switch o.Net {
	case "auto", "on", "off":
	default:
		return paramErr("send: --net 仅支持 auto/on/off，实际 %q", o.Net)
	}
	return nil
}

// validateReceiveOpts 校验 receive 参数，非法返回退出码 2。
func validateReceiveOpts(o *receive.Options) error {
	switch o.Source {
	case "camera", "file", "stdin":
	default:
		return paramErr("receive: --source 仅支持 camera/file/stdin，实际 %q", o.Source)
	}
	if o.Source == "file" && o.FilePath == "" {
		return paramErr("receive: --source file 需要指定图片文件或目录路径（位置参数）")
	}
	if o.Source != "file" && o.FilePath != "" {
		return paramErr("receive: 位置参数 [路径] 仅用于 --source file")
	}
	if o.FPS < 0 {
		return paramErr("receive: --fps 必须 ≥ 0，实际 %d", o.FPS)
	}
	if _, err := qrcode.Validate(qrcode.Options{Version: o.Version}); err != nil {
		return paramErr("receive: %v", err)
	}
	if o.ExpectSize < 0 {
		return paramErr("receive: --expect-size 必须 ≥ 0，实际 %d", o.ExpectSize)
	}
	if o.Hash != "" {
		hex := strings.TrimPrefix(o.Hash, "sha256:")
		if hex == o.Hash || len(hex) != 64 {
			return paramErr("receive: --hash 格式应为 sha256:<64位十六进制>，实际 %q", o.Hash)
		}
	}
	if o.Timeout < 0 {
		return paramErr("receive: --timeout 必须 ≥ 0，实际 %v", o.Timeout)
	}
	return nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		code := exitCodeFor(err)
		if !errors.Is(err, send.ErrInterrupted) && !errors.Is(err, receive.ErrInterrupted) {
			fmt.Fprintln(os.Stderr, "错误:", err)
		}
		os.Exit(code)
	}
}

// exitCodeFor 将错误映射为退出码：0 成功/1 传输失败/2 参数错误/3 环境错误。
func exitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var ee *exitErr
	if errors.As(err, &ee) {
		return ee.code
	}
	if errors.Is(err, send.ErrUsage) || errors.Is(err, receive.ErrUsage) {
		return 2
	}
	if errors.Is(err, send.ErrEnv) || errors.Is(err, receive.ErrEnv) {
		return 3
	}
	return 1
}
