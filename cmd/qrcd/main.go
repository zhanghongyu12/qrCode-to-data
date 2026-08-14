package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "qrcd",
	Short: "二维码数据传输工具",
	Long: `qrcd — 通过二维码实现纯光学数据传输的 CLI 工具。

支持发送（send）与接收（receive）两个子命令：
  qrcd send <file>      发送文件
  qrcd send --text <文本> 发送文本
  qrcd receive           接收数据

纯光学传输无需网络，通过屏幕二维码与摄像头完成数据交换。
Phase 2 将支持同网直传加速与手机离线中转。`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.AddCommand(sendCmd())
	rootCmd.AddCommand(receiveCmd())
}

func sendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send <file>",
		Short: "发送文件或文本",
		Long: `通过二维码流发送文件或文本。

示例：
  qrcd send ./backup.tar.gz
  qrcd send --text "Hello World"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO(TASK-008): 实现发送编排
			fmt.Println("send 命令尚未实现")
			return nil
		},
	}

	cmd.Flags().StringP("text", "t", "", "直接发送文本（与文件二选一）")
	cmd.Flags().Int("fps", 10, "二维码播放帧率（帧/秒）")
	cmd.Flags().IntP("version", "v", 20, "QR 版本上限（1~40）")
	cmd.Flags().String("ecc", "L", "QR 纠错级别 L/M/Q/H")
	cmd.Flags().Float64P("redundancy", "r", 0.1, "喷泉码冗余度（0~1）")
	cmd.Flags().IntP("block-size", "b", 1024, "源分块大小（字节）")
	cmd.Flags().String("terminal", "ansi", "渲染器：ansi（终端半块）")
	cmd.Flags().Bool("invert", false, "反色（浅色终端背景）")
	cmd.Flags().BoolP("quiet", "q", false, "关闭进度，只输出最终结果")
	cmd.Flags().String("net", "auto", "同网直传：auto/on/off（Phase 2）")
	cmd.Flags().String("addr", "", "指定监听 IP（同网直传，Phase 2）")

	return cmd
}

func receiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "receive",
		Short: "接收数据",
		Long: `通过摄像头或文件/标准输入接收二维码流并还原数据。

示例：
  qrcd receive
  qrcd receive --output ./out/
  qrcd receive --source file --output ./out/`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO(TASK-009): 实现接收编排
			fmt.Println("receive 命令尚未实现")
			return nil
		},
	}

	cmd.Flags().StringP("output", "o", ".", "输出目录或完整文件名")
	cmd.Flags().Int("camera", 0, "摄像头设备号")
	cmd.Flags().String("source", "camera", "输入源：camera/file/stdin")
	cmd.Flags().Int("fps", 30, "采集/解码帧率上限")
	cmd.Flags().IntP("version", "v", 20, "期望 QR 版本上限")
	cmd.Flags().Int("expect-size", 0, "预期总字节数（可选）")
	cmd.Flags().String("hash", "", "预期校验值，格式 sha256:<hex>")
	cmd.Flags().Duration("timeout", 0, "无新符号超时（如 30s）")
	cmd.Flags().Bool("overwrite", false, "允许覆盖已存在文件")
	cmd.Flags().BoolP("quiet", "q", false, "关闭进度，只输出最终结果")

	return cmd
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}