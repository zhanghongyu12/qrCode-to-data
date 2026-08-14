package payload

import (
	"fmt"
)

// SummaryName 生成文本载荷的摘要文件名：<sha256 前 16 位>.txt。
// 确定性生成，同名文本得到同名文件。
func SummaryName(text string) string {
	sum := SHA256Hex([]byte(text))
	return fmt.Sprintf("%s.txt", sum[:16])
}
