package e2e_test

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"qrcd/internal/payload"
	"qrcd/internal/qrcode"
	"qrcd/internal/send"
)

// 端到端测试：CLI 黑盒。TestMain 构建 qrcd 二进制一次，子进程跑各场景，
// 校验退出码（0 成功/1 传输失败/2 参数错误/3 环境错误）与 stdout/stderr 输出。
// 说明：CLI 级 send→receive 真实闭环需摄像头（send 渲染终端 ANSI，不落盘帧图片），
// 故本层退化为 CLI 行为验证；端到端字节级闭环由 tests/integration 用完整 API 覆盖。

var qrcdBin string

func TestMain(m *testing.M) {
	// 二进制落 GOTMPDIR（项目内 .tmp/），避免系统 temp 被 360 拦截
	base := os.Getenv("GOTMPDIR")
	if base == "" {
		base = os.TempDir()
	}
	dir, err := os.MkdirTemp(base, "qrcd-e2e-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup:", err)
		os.Exit(1)
	}
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	qrcdBin = filepath.Join(dir, "qrcd"+exe)
	// 子进程 go build：用绝对项目根，因 go test 的 cwd 是 tests/e2e/
	cmd := exec.Command("go", "build", "-o", qrcdBin, "E:/aiCode/qrCode-to-data/cmd/qrcd")
	cmd.Env = filterGOROOT(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Stderr.Write(out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func filterGOROOT(env []string) []string {
	out := env[:0]
	for _, e := range env {
		if !strings.HasPrefix(e, "GOROOT=") {
			out = append(out, e)
		}
	}
	return out
}

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var so, se bytes.Buffer
	cmd := exec.Command(qrcdBin, args...)
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	code = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("执行 qrcd 失败: %v", err)
		}
	}
	return code, so.String(), se.String()
}

func TestCLI_Help(t *testing.T) {
	code, out, _ := run(t, "--help")
	if code != 0 {
		t.Errorf("--help 退出码 %d, 期望 0", code)
	}
	if !strings.Contains(out, "qrcd") {
		t.Errorf("帮助应含 qrcd: %q", out)
	}
}

func TestCLI_SendShortText(t *testing.T) {
	code, out, _ := run(t, "send", "--text", "Hello QRCD E2E", "--quiet")
	if code != 0 {
		t.Errorf("短文本发送退出码 %d, 期望 0", code)
	}
	if !strings.Contains(out, "SHA-256:") {
		t.Errorf("应输出 SHA-256: %q", out)
	}
	if !strings.Contains(out, "▀") && !strings.Contains(out, "█") {
		t.Errorf("应渲染 QR 半块字符: %q", out)
	}
}

func TestCLI_SendNoArg(t *testing.T) {
	code, _, se := run(t, "send")
	if code != 2 {
		t.Errorf("缺参退出码 %d, 期望 2", code)
	}
	if se == "" {
		t.Error("缺参应输出错误到 stderr")
	}
}

func TestCLI_SendBadECC(t *testing.T) {
	code, _, _ := run(t, "send", "--text", "x", "--ecc", "X")
	if code != 2 {
		t.Errorf("非法 ECC 退出码 %d, 期望 2", code)
	}
}

func TestCLI_SendBadFPS(t *testing.T) {
	code, _, _ := run(t, "send", "--text", "x", "--fps", "0")
	if code != 2 {
		t.Errorf("非法 fps 退出码 %d, 期望 2", code)
	}
}

func TestCLI_ReceiveCameraNoGocv(t *testing.T) {
	code, _, _ := run(t, "receive", "--camera", "0")
	if code != 3 {
		t.Errorf("无 gocv 环境 --camera 退出码 %d, 期望 3", code)
	}
}

func TestCLI_ReceiveFileNoPath(t *testing.T) {
	code, _, _ := run(t, "receive", "--source", "file")
	if code != 2 {
		t.Errorf("--source file 无路径退出码 %d, 期望 2", code)
	}
}

func TestCLI_ReceiveBadSource(t *testing.T) {
	code, _, _ := run(t, "receive", "--source", "bogus")
	if code != 2 {
		t.Errorf("非法 source 退出码 %d, 期望 2", code)
	}
}

// TestCLI_ReceiveBadHash 生成有效帧目录，--hash 预校验不匹配 → 退出码 1。
func TestCLI_ReceiveBadHash(t *testing.T) {
	data := []byte("hash e2e roundtrip data")
	load := &payload.Load{Data: data, Name: "e2e.bin", PayloadType: "file", MimeType: "application/octet-stream"}
	stream, err := send.BuildStream(load, send.Options{BlockSize: 1024, Version: 20, ECC: "L", Redundancy: 0.1})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	i := 0
	for {
		it, ok := stream.Next()
		if !ok {
			break
		}
		code, err := qrcode.Encode(it.Bytes, qrcode.Options{Version: 20, ECC: "L"})
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%04d.png", i)))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, code.Image(8)); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		i++
	}
	outDir := t.TempDir()
	badHash := "sha256:" + strings.Repeat("0", 64)
	code, _, _ := run(t, "receive", "--source", "file", dir, "--output", outDir, "--hash", badHash)
	if code != 1 {
		t.Errorf("hash 预校验失败退出码 %d, 期望 1", code)
	}
}
