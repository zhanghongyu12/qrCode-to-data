package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"qrcd/internal/web"
)

// usbPort USB 直连固定端口，与还原端默认监听端口一致。
// 手机 App 选「USB 直连」后 POST 到 localhost:<usbPort>，经 adb reverse 走 USB 到本机。
const usbPort = 8080

// setupUSBReverse 后台循环维护手机 USB 直连隧道（还原端专用）。
// 每 2s 探测设备，出现即执行 adb reverse；掉线重插自动重建。
func setupUSBReverse(ctx context.Context) {
	adb := findADB()
	if adb == "" {
		web.SetUSBStatus(web.USBStatus{Available: false, Detail: "未找到 adb"})
		return
	}
	wasUp := false
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		c := exec.Command(adb, "devices")
		hideWindow(c)
		out, _ := c.Output()
		up, serial := detectDevice(string(out))
		reverseOk := false
		if up {
			rc := exec.Command(adb, "reverse",
				fmt.Sprintf("tcp:%d", usbPort),
				fmt.Sprintf("tcp:%d", usbPort))
			hideWindow(rc)
			co, err := rc.CombinedOutput()
			reverseOk = err == nil
			if err != nil {
				fmt.Printf("  ⚠ USB 直连建立失败: %v %s\n", err, strings.TrimSpace(string(co)))
			}
		}
		detail := "手机未连接"
		if up && reverseOk {
			detail = "手机已连接，USB 直连就绪"
		} else if up {
			detail = "手机已连接，但隧道建立失败"
		}
		web.SetUSBStatus(web.USBStatus{Available: true, Device: up, Reverse: reverseOk, Serial: serial, Detail: detail})
		if up != wasUp {
			if up {
				fmt.Printf("  ✓ 手机已连接：USB 直连就绪（App 内选「USB 直连」即可，无需填 IP）\n")
			} else {
				fmt.Printf("  ⚠ 手机断开：USB 直连暂停，重新插上会自动恢复\n")
			}
			wasUp = up
		}
	}
}

// findADB 定位 adb 可执行文件（PATH 优先，其次 Windows SDK 常见路径）。
func findADB() string {
	if p, err := exec.LookPath("adb"); err == nil {
		return p
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		p := filepath.Join(la, "Android", "Sdk", "platform-tools", "adb.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if ah := os.Getenv("ANDROID_HOME"); ah != "" {
		p := filepath.Join(ah, "platform-tools", "adb.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// detectDevice 判断 adb devices 输出里是否有 online 设备，返回其序列号。
func detectDevice(out string) (bool, string) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, "device") {
			if parts := strings.Fields(line); len(parts) >= 1 {
				return true, parts[0]
			}
			return true, ""
		}
	}
	return false, ""
}

// hideWindow 隐藏子进程控制台窗口，避免 adb 每次调用弹出黑窗闪烁。
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
