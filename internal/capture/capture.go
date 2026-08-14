package capture

import (
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Frame 一帧数据
type Frame struct {
	// Image 图像帧（camera/file 输入源提供），供 qrcode 解码
	Image image.Image
	// Data 原始字节（stdin 输入源提供），供 frame 直接拆帧
	Data []byte
}

// Source 抽象帧源接口，解码层只依赖本接口，不直接依赖 gocv。
// camera/file/stdin 三种实现可插拔。
type Source interface {
	// Next 返回下一帧；返回 io.EOF 表示正常结束。
	Next() (Frame, error)
	// Close 释放资源
	Close() error
}

// Options 帧源选项
type Options struct {
	// CameraID 摄像头设备号（--camera）
	CameraID int
	// FilePath 文件/目录路径（--source file）
	FilePath string
	// Stdin 标准输入（--source stdin），nil 时使用 os.Stdin
	Stdin io.Reader
	// MaxFPS 采集/解码帧率上限
	MaxFPS int
}

// NewSource 创建指定类型的帧源。
// sourceType: "camera" / "file" / "stdin"。
func NewSource(sourceType string, opts Options) (Source, error) {
	switch sourceType {
	case "camera":
		return newCameraSource(opts)
	case "file":
		return newFileSource(opts.FilePath)
	case "stdin":
		return newStdinSource(opts.Stdin)
	default:
		return nil, fmt.Errorf("capture: 未知输入源 %q，支持 camera/file/stdin", sourceType)
	}
}

// videoExts 常见视频扩展名（默认构建不支持解码，给出明确提示）
var videoExts = map[string]bool{
	".mp4": true, ".avi": true, ".mkv": true, ".mov": true,
	".webm": true, ".flv": true, ".m4v": true, ".ts": true,
	".mpg": true, ".mpeg": true, ".wmv": true, ".ogv": true,
}

// fileSource 从图片文件或图片目录逐帧读取（普通文件，默认编译）。
// 视频文件解码需要 gocv 构建（-tags qrcd_camera），默认构建给出明确错误。
type fileSource struct {
	frames []Frame
	index  int
}

// newFileSource 创建文件帧源。
// path 为单个图片文件 → 一帧；为目录 → 按文件名排序逐张读取目录内图片。
func newFileSource(path string) (Source, error) {
	if path == "" {
		return nil, fmt.Errorf("capture: --source file 需要指定文件或目录路径")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("capture: 访问路径 %q 失败: %w", path, err)
	}

	var images []Frame
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("capture: 读取目录 %q 失败: %w", path, err)
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			img, err := decodeImageFile(filepath.Join(path, n))
			if err == nil {
				images = append(images, Frame{Image: img})
			}
		}
	} else {
		img, err := decodeImageFile(path)
		if err != nil {
			return nil, err
		}
		images = append(images, Frame{Image: img})
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("capture: %q 中没有可解码的图片（PNG/JPEG/GIF/BMP）", path)
	}
	return &fileSource{frames: images}, nil
}

// Next 返回下一帧图像；耗尽后返回 io.EOF
func (s *fileSource) Next() (Frame, error) {
	if s.index >= len(s.frames) {
		return Frame{}, io.EOF
	}
	f := s.frames[s.index]
	s.index++
	return f, nil
}

// Close 文件帧源无资源需要释放
func (s *fileSource) Close() error { return nil }

// decodeImageFile 解码单个图片文件；视频文件给出明确的中文错误提示。
func decodeImageFile(path string) (image.Image, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if videoExts[ext] {
		return nil, fmt.Errorf("capture: 视频文件 %q 解码需要 gocv（OpenCV）构建（-tags qrcd_camera），默认构建仅支持图片文件/目录", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("capture: 打开文件 %q 失败: %w", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("capture: 解码图片 %q 失败（不支持格式或文件损坏）: %w", path, err)
	}
	return img, nil
}

// stdinSource 从标准输入读取字节流（普通文件，默认编译）
type stdinSource struct {
	r io.Reader
}

// newStdinSource 创建标准输入帧源，r 为 nil 时使用 os.Stdin
func newStdinSource(r io.Reader) (Source, error) {
	if r == nil {
		r = os.Stdin
	}
	return &stdinSource{r: r}, nil
}

// Next 读取全部字节作为一帧；读取完成后返回 io.EOF
func (s *stdinSource) Next() (Frame, error) {
	data, err := io.ReadAll(s.r)
	if err != nil {
		return Frame{}, fmt.Errorf("capture: 读取标准输入失败: %w", err)
	}
	if len(data) == 0 {
		return Frame{}, io.EOF
	}
	return Frame{Data: data}, nil
}

// Close 标准输入帧源无资源需要释放
func (s *stdinSource) Close() error { return nil }

// 编译期接口断言
var (
	_ Source = (*fileSource)(nil)
	_ Source = (*stdinSource)(nil)
)
