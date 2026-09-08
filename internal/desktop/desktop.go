// Package desktop 提供桌面端：WebView2 原生窗口承载播放端/还原端页面。
// 复用现有 Web 页面（/desktop 壳），无需浏览器，窗口关闭即退出。
package desktop

import (
	"runtime"

	"github.com/jchv/go-webview2"
)

// Open 打开桌面窗口并阻塞，直到用户关闭窗口。
// 返回 false 表示当前环境无 WebView2 运行时（应回退到浏览器）。
func Open(url, title string) bool {
	// 锁定 OS 线程：Win32 窗口的消息循环必须与其创建线程一致。
	// 本函数在 goroutine 中调用，若不锁线程，goroutine 迁移线程会导致窗口假死。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  title,
			Width:  1280,
			Height: 820,
		},
	})
	if w == nil {
		return false
	}
	defer w.Destroy()
	w.Navigate(url)
	w.Run()
	return true
}
