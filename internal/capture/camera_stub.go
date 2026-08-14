//go:build !qrcd_camera

package capture

import (
	"fmt"
)

// newCameraSource 在未启用 qrcd_camera 构建标签时返回明确错误。
// 摄像头路径依赖 gocv（OpenCV，CGO），默认构建不引入该依赖。
// 需要摄像头时使用 `go build -tags qrcd_camera` 并先安装 gocv/OpenCV。
func newCameraSource(opts Options) (Source, error) {
	return nil, fmt.Errorf("capture: 摄像头路径需要 gocv（OpenCV）支持，请使用 `go build -tags qrcd_camera` 重新构建，或改用 --source file|stdin")
}
