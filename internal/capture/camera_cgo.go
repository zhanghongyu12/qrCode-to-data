//go:build qrcd_camera

package capture

import (
	"fmt"
	"io"

	"gocv.io/x/gocv"
)

// cameraSource 基于 gocv 的摄像头帧源（仅 -tags qrcd_camera 构建时编译）。
// 本文件默认不参与构建；go.mod 也不 require gocv，需在启用该标签前手动加入。
type cameraSource struct {
	vc   *gocv.VideoCapture
	mat  gocv.Mat
	opts Options
}

// newCameraSource 打开指定设备号的摄像头。
// 设备不存在/权限拒绝时返回中文错误（由 CLI 层映射为退出码 3）。
func newCameraSource(opts Options) (Source, error) {
	vc, err := gocv.OpenVideoCapture(opts.CameraID)
	if err != nil {
		return nil, fmt.Errorf("capture: 打开摄像头 %d 失败（设备不存在或无权限）: %w", opts.CameraID, err)
	}
	if !vc.IsOpened() {
		vc.Close()
		return nil, fmt.Errorf("capture: 打开摄像头 %d 失败（设备不存在或无权限）", opts.CameraID)
	}
	return &cameraSource{
		vc:   vc,
		mat:  gocv.NewMat(),
		opts: opts,
	}, nil
}

// Next 读取下一帧图像；摄像头无更多帧时返回 io.EOF
func (s *cameraSource) Next() (Frame, error) {
	ok := s.vc.Read(&s.mat)
	if !ok || s.mat.Empty() {
		return Frame{}, io.EOF
	}
	img, err := s.mat.ToImage()
	if err != nil {
		return Frame{}, fmt.Errorf("capture: 摄像头帧转图像失败: %w", err)
	}
	return Frame{Image: img}, nil
}

// Close 释放摄像头与矩阵资源
func (s *cameraSource) Close() error {
	s.mat.Close()
	s.vc.Close()
	return nil
}

// 编译期接口断言
var _ Source = (*cameraSource)(nil)
