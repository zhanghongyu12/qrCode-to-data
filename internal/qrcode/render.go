package qrcode

import (
	"image"
	"image/color"
	"strings"
)

// ANSI 控制序列（用于逐帧刷新，不滚动累积）
const (
	// ANSICursorHome 光标复位到左上角（配合隐藏光标实现原地刷新）
	ANSICursorHome = "\r\x1b[H"
	// ANSICursorHide 隐藏光标
	ANSICursorHide = "\x1b[?25l"
	// ANSICursorShow 显示光标
	ANSICursorShow = "\x1b[?25h"
)

// halfBlock 根据上下两个模块的黑/白返回半块字符
func halfBlock(topBlack, bottomBlack bool) rune {
	switch {
	case topBlack && bottomBlack:
		return '█' // U+2588 全块
	case topBlack && !bottomBlack:
		return '▀' // U+2580 上半块
	case !topBlack && bottomBlack:
		return '▄' // U+2584 下半块
	default:
		return ' '
	}
}

// moduleAt 返回位图在 (col,row) 处是否为黑；越界（静区）视为白
func moduleAt(m [][]bool, col, row int) bool {
	if row < 0 || row >= len(m) || col < 0 || col >= len(m) {
		return false
	}
	return m[row][col]
}

// RenderANSI 返回 ANSI 半块字符渲染文本。
// Bitmap 已含 ≥4 模块静区；输出仅含半块字符（▀▄█）与空格，无彩色。
func RenderANSI(c *Code, invert bool) string {
	m := c.Bitmap
	var sb strings.Builder
	for row := 0; row < len(m); row += 2 {
		for col := 0; col < len(m); col++ {
			top := moduleAt(m, col, row)
			bottom := moduleAt(m, col, row+1)
			if invert {
				top, bottom = !top, !bottom
			}
			sb.WriteRune(halfBlock(top, bottom))
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// RenderASCII 返回 ASCII 降级渲染文本（黑='#'，白=' '），含静区。
func RenderASCII(c *Code, invert bool) string {
	m := c.Bitmap
	var sb strings.Builder
	for _, row := range m {
		for _, v := range row {
			black := v
			if invert {
				black = !black
			}
			if black {
				sb.WriteByte('#')
			} else {
				sb.WriteByte(' ')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// Image 将位图渲染为灰度图像（含静区）。
// scale 为每个模块的像素数（≤0 时按 1 处理）。
func (c *Code) Image(scale int) *image.Gray {
	n := len(c.Bitmap)
	if scale <= 0 {
		scale = 1
	}
	img := image.NewGray(image.Rect(0, 0, n*scale, n*scale))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			black := c.Bitmap[y][x]
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					if black {
						img.SetGray(x*scale+dx, y*scale+dy, color.Gray{Y: 0})
					} else {
						img.SetGray(x*scale+dx, y*scale+dy, color.Gray{Y: 255})
					}
				}
			}
		}
	}
	return img
}
