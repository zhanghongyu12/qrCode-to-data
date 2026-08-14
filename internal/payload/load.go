package payload

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
)

// Load 表示读取到的载荷
type Load struct {
	// Data 载荷字节
	Data []byte
	// Name 原始文件名或文本摘要名
	Name string
	// PayloadType "file" 或 "text"
	PayloadType string
	// MimeType 内容类型
	MimeType string
}

// ReadFile 读取文件为载荷（file 类型）
func ReadFile(path string) (*Load, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("payload: 读取文件失败 %q: %w", path, err)
	}
	return &Load{
		Data:        data,
		Name:        filepath.Base(path),
		PayloadType: "file",
		MimeType:    mimeType(filepath.Base(path)),
	}, nil
}

// ReadText 将文本包装为载荷（text 类型），名字为文本摘要名
func ReadText(text string) (*Load, error) {
	return &Load{
		Data:        []byte(text),
		Name:        SummaryName(text),
		PayloadType: "text",
		MimeType:    "text/plain; charset=utf-8",
	}, nil
}

// ReadStdin 从 reader 读取全部字节为载荷（file 类型，默认名 stdin.bin）
func ReadStdin(r io.Reader) (*Load, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("payload: 读取标准输入失败: %w", err)
	}
	return &Load{
		Data:        data,
		Name:        "stdin.bin",
		PayloadType: "file",
		MimeType:    "application/octet-stream",
	}, nil
}

// mimeType 根据文件名后缀猜测 MIME 类型，未知返回 application/octet-stream
func mimeType(name string) string {
	if t := mime.TypeByExtension(filepath.Ext(name)); t != "" {
		return t
	}
	return "application/octet-stream"
}
