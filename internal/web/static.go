package web

import (
	_ "embed"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed static/jsQR.js
var jsQRJS []byte

// handleJSQR 本地提供 jsQR 解码库（手机中继页面使用，避免依赖外网 CDN）。
func (s *Server) handleJSQR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(jsQRJS)
}

// apkCandidates APK 可能所在的固定路径（项目根 / 当前目录 / android 构建输出）。
// 运行时按顺序查找，找到第一个就提供下载。
var apkCandidates = []string{
	"qrcd-app-debug.apk",
	"android/app/build/outputs/apk/debug/app-debug.apk",
	"app-debug.apk",
}

// handleAPKDownload 提供手机中继 App 的 APK 下载。
// 手机访问 http://<本机IP>:8080/dl/app.apk 即可直接下载安装。
func (s *Server) handleAPKDownload(w http.ResponseWriter, r *http.Request) {
	for _, p := range apkCandidates {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		fi, err := os.Stat(abs)
		if err != nil || fi.IsDir() {
			continue
		}
		w.Header().Set("Content-Type", "application/vnd.android.package-archive")
		w.Header().Set("Content-Disposition", `attachment; filename="qrcd-relay.apk"`)
		http.ServeFile(w, r, abs)
		return
	}
	http.NotFound(w, r)
}

