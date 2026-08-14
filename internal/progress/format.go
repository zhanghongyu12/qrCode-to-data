package progress

import (
	"fmt"
	"strings"
	"time"
)

// Percent 返回完成百分比（0~100）。total<=0 返回 0，count>=total 返回 100。
func Percent(count, total int) float64 {
	if total <= 0 {
		return 0
	}
	if count >= total {
		return 100
	}
	return float64(count) * 100 / float64(total)
}

// Rate 返回速率（块/秒）。elapsed<=0 返回 0。
func Rate(count int, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	return float64(count) / elapsed.Seconds()
}

// ETA 返回预计剩余秒数（已用 elapsed 播完 count 块，还需 total-count 块）。
func ETA(count, total int, elapsed time.Duration) float64 {
	if count <= 0 || total <= count {
		return 0
	}
	rate := Rate(count, elapsed)
	if rate <= 0 {
		return 0
	}
	return float64(total-count) / rate
}

// FormatBar 生成 ASCII 进度条文本，如 "[====>     ] 45.0%"。width<=0 时默认 20。
func FormatBar(percent float64, width int) string {
	if width <= 0 {
		width = 20
	}
	filled := int(percent * float64(width) / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return fmt.Sprintf("[%s%s] %.1f%%", strings.Repeat("=", filled), strings.Repeat(" ", width-filled), percent)
}
