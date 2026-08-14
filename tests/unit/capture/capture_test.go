package capture_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qrcd/internal/capture"
)

// makePNG 生成一张纯色 PNG 图片，写入 path 并返回其尺寸
func makePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建测试图片失败: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("编码测试图片失败: %v", err)
	}
}

// TestFileSourceImage 从单个图片文件读取一帧
func TestFileSourceImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "frame1.png")
	makePNG(t, path, 16, 16)

	src, err := capture.NewSource("file", capture.Options{FilePath: path})
	if err != nil {
		t.Fatalf("NewSource(file) 失败: %v", err)
	}
	defer src.Close()

	f, err := src.Next()
	if err != nil {
		t.Fatalf("Next 失败: %v", err)
	}
	if f.Image == nil {
		t.Fatal("file 帧应提供 Image")
	}
	if b := f.Image.Bounds(); b.Dx() != 16 || b.Dy() != 16 {
		t.Errorf("图片尺寸不正确: %v", b)
	}
	// 耗尽
	if _, err := src.Next(); err != io.EOF {
		t.Errorf("第二帧应返回 io.EOF，实际 %v", err)
	}
}

// TestFileSourceDir 从目录逐帧读取多张图片（按文件名排序）
func TestFileSourceDir(t *testing.T) {
	dir := t.TempDir()
	makePNG(t, filepath.Join(dir, "02.png"), 8, 8)
	makePNG(t, filepath.Join(dir, "01.png"), 8, 8)

	src, err := capture.NewSource("file", capture.Options{FilePath: dir})
	if err != nil {
		t.Fatalf("NewSource(file 目录) 失败: %v", err)
	}
	defer src.Close()

	frames := 0
	for {
		f, err := src.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next 失败: %v", err)
		}
		if f.Image == nil {
			t.Error("目录帧应提供 Image")
		}
		frames++
	}
	if frames != 2 {
		t.Errorf("目录应读取 2 帧，实际 %d", frames)
	}
}

// TestFileSourceNotExist 路径不存在 → 中文报错
func TestFileSourceNotExist(t *testing.T) {
	_, err := capture.NewSource("file", capture.Options{FilePath: filepath.Join(t.TempDir(), "missing.png")})
	if err == nil {
		t.Fatal("不存在路径应报错")
	}
	if !strings.Contains(err.Error(), "失败") {
		t.Errorf("错误应含中文上下文: %v", err)
	}
}

// TestFileSourceVideo 视频文件 → 明确中文错误（默认构建不支持）
func TestFileSourceVideo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.mp4")
	// 写一个非图片内容的假 mp4
	if err := os.WriteFile(path, []byte("not a real video"), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	_, err := capture.NewSource("file", capture.Options{FilePath: path})
	if err == nil {
		t.Fatal("视频文件应报错")
	}
	if !strings.Contains(err.Error(), "gocv") {
		t.Errorf("视频文件错误应提示需要 gocv，实际: %v", err)
	}
}

// TestStdinSource 从 reader 读取字节流
func TestStdinSource(t *testing.T) {
	data := []byte("raw frame bytes stream")
	src, err := capture.NewSource("stdin", capture.Options{Stdin: bytes.NewReader(data)})
	if err != nil {
		t.Fatalf("NewSource(stdin) 失败: %v", err)
	}
	defer src.Close()

	f, err := src.Next()
	if err != nil {
		t.Fatalf("Next 失败: %v", err)
	}
	if !bytes.Equal(f.Data, data) {
		t.Errorf("stdin 帧字节不一致")
	}
	if _, err := src.Next(); err != io.EOF {
		t.Errorf("读取完应返回 io.EOF，实际 %v", err)
	}
}

// TestStdinSourceEmpty 空 stdin → io.EOF
func TestStdinSourceEmpty(t *testing.T) {
	src, err := capture.NewSource("stdin", capture.Options{Stdin: bytes.NewReader(nil)})
	if err != nil {
		t.Fatalf("NewSource(stdin) 失败: %v", err)
	}
	defer src.Close()
	if _, err := src.Next(); err != io.EOF {
		t.Errorf("空 stdin 应返回 io.EOF，实际 %v", err)
	}
}

// TestNewSourceUnknown 未知输入源 → 报错
func TestNewSourceUnknown(t *testing.T) {
	_, err := capture.NewSource("bogus", capture.Options{})
	if err == nil {
		t.Fatal("未知输入源应报错")
	}
}

// TestNewSourceCameraDefault 默认构建下 camera 路径返回明确中文错误（无 gocv）
func TestNewSourceCameraDefault(t *testing.T) {
	_, err := capture.NewSource("camera", capture.Options{CameraID: 0})
	if err == nil {
		t.Fatal("默认构建（无 gocv）camera 应返回错误")
	}
	if !strings.Contains(err.Error(), "gocv") {
		t.Errorf("camera 错误应提示需要 gocv 构建，实际: %v", err)
	}
}

