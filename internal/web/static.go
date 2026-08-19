package web

import (
	_ "embed"
	"net/http"
)

//go:embed static/jsQR.js
var jsQRJS []byte

// handleJSQR 本地提供 jsQR 解码库（手机中继页面使用，避免依赖外网 CDN）。
func (s *Server) handleJSQR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(jsQRJS)
}
