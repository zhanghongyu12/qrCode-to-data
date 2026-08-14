package progress_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"qrcd/internal/progress"
)

// TestPercent 百分比计算
func TestPercent(t *testing.T) {
	if p := progress.Percent(0, 100); p != 0 {
		t.Errorf("0/100 应为 0，实际 %.1f", p)
	}
	if p := progress.Percent(50, 100); p != 50 {
		t.Errorf("50/100 应为 50，实际 %.1f", p)
	}
	if p := progress.Percent(100, 100); p != 100 {
		t.Errorf("100/100 应为 100，实际 %.1f", p)
	}
	if p := progress.Percent(120, 100); p != 100 {
		t.Errorf("超过 100%% 应封顶 100，实际 %.1f", p)
	}
	if p := progress.Percent(5, 0); p != 0 {
		t.Errorf("total=0 应为 0，实际 %.1f", p)
	}
}

// TestRate 速率计算
func TestRate(t *testing.T) {
	if r := progress.Rate(100, 10*time.Second); r != 10 {
		t.Errorf("100/10s 速率应为 10 块/s，实际 %.1f", r)
	}
	if r := progress.Rate(10, 0); r != 0 {
		t.Errorf("elapsed=0 速率应为 0，实际 %.1f", r)
	}
}

// TestETA 预计剩余
func TestETA(t *testing.T) {
	// 10s 播完 10 块 → 速率 1 块/s，剩 90 块 → 90s
	if e := progress.ETA(10, 100, 10*time.Second); e != 90 {
		t.Errorf("ETA 应为 90s，实际 %.1f", e)
	}
	if e := progress.ETA(0, 100, 10*time.Second); e != 0 {
		t.Errorf("count=0 ETA 应为 0，实际 %.1f", e)
	}
	if e := progress.ETA(100, 100, 10*time.Second); e != 0 {
		t.Errorf("已完成 ETA 应为 0，实际 %.1f", e)
	}
}

// TestFormatBar 进度条文本
func TestFormatBar(t *testing.T) {
	bar := progress.FormatBar(50, 10)
	if !strings.HasPrefix(bar, "[") || !strings.HasSuffix(bar, "50.0%") {
		t.Errorf("进度条格式不正确: %q", bar)
	}
	if !strings.Contains(bar, "=====") {
		t.Errorf("进度条应含 5 个 '='（50%% 宽度 10），实际: %q", bar)
	}
}

// TestTerminalReport 进度行含百分比/块数/速率，且 quiet 下无输出
func TestTerminalReport(t *testing.T) {
	var buf bytes.Buffer
	term := progress.NewTerminal(&buf, false)

	// 人为构造 elapsed：用小的 start 时间差较难，直接用 FormatEvent 验证内容
	line := progress.FormatEvent(progress.Event{Kind: progress.Receive, Count: 45, Total: 100, Dedup: 3}, 5*time.Second)
	for _, want := range []string{"接收:", "45/100", "45.0%", "去重 3", "块/s"} {
		if !strings.Contains(line, want) {
			t.Errorf("接收进度行缺少 %q: %s", want, line)
		}
	}
	line = progress.FormatEvent(progress.Event{Kind: progress.Send, Count: 10, Total: 100}, 1*time.Second)
	for _, want := range []string{"发送:", "10/100", "10.0%", "fps", "剩余约"} {
		if !strings.Contains(line, want) {
			t.Errorf("发送进度行缺少 %q: %s", want, line)
		}
	}

	term.Report(progress.Event{Kind: progress.Receive, Count: 45, Total: 100, Dedup: 3})
	out := buf.String()
	if out == "" {
		t.Fatal("非 quiet 模式 Report 应有输出")
	}
	if !strings.Contains(out, "45/100") || !strings.Contains(out, "%") {
		t.Errorf("Report 输出缺少进度字段: %q", out)
	}
	if !strings.Contains(out, "\r") {
		t.Errorf("Report 应含光标复位 \\r（不滚动累积）: %q", out)
	}
}

// TestTerminalQuiet quiet 模式：Report 无输出，Finish 仅输出最终结果
func TestTerminalQuiet(t *testing.T) {
	var buf bytes.Buffer
	term := progress.NewTerminal(&buf, true)
	term.Report(progress.Event{Kind: progress.Send, Count: 50, Total: 100})
	if buf.Len() != 0 {
		t.Errorf("quiet 模式 Report 不应输出，实际 %q", buf.String())
	}
	term.Finish(progress.Event{Kind: progress.Receive, Count: 100, Total: 100, Done: true, Message: "校验通过: /out/a.bin"})
	out := buf.String()
	if out == "" {
		t.Fatal("quiet 模式 Finish 应输出最终结果")
	}
	if !strings.Contains(out, "校验通过") {
		t.Errorf("quiet 模式应输出最终结果消息: %q", out)
	}
}

// TestTerminalFinish 非 quiet 模式 Finish 输出 100% 与最终结果
func TestTerminalFinish(t *testing.T) {
	var buf bytes.Buffer
	term := progress.NewTerminal(&buf, false)
	term.Finish(progress.Event{Kind: progress.Receive, Count: 100, Total: 100, Done: true, Message: "校验通过"})
	out := buf.String()
	if !strings.Contains(out, "100/100") || !strings.Contains(out, "100.0%") {
		t.Errorf("Finish 应输出 100%% 进度: %q", out)
	}
	if !strings.Contains(out, "校验通过") {
		t.Errorf("Finish 应输出最终结果: %q", out)
	}
}

// TestTerminalClose 非 quiet 模式 Close 输出换行
func TestTerminalClose(t *testing.T) {
	var buf bytes.Buffer
	term := progress.NewTerminal(&buf, false)
	term.Close()
	if !strings.Contains(buf.String(), "\n") {
		t.Error("非 quiet Close 应输出换行结束进度行")
	}
	// 幂等
	term.Close()
}

// TestMonitor 无新块超时
func TestMonitor(t *testing.T) {
	m := progress.NewMonitor(50 * time.Millisecond)
	if err := m.Check(); err != nil {
		t.Fatalf("初始不应超时: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if err := m.Check(); err != progress.ErrTimeout {
		t.Errorf("超时后应返回 ErrTimeout，实际 %v", err)
	}

	// Notify 后刷新
	m = progress.NewMonitor(100 * time.Millisecond)
	m.Notify()
	time.Sleep(30 * time.Millisecond)
	if err := m.Check(); err != nil {
		t.Errorf("Notify 后不应超时: %v", err)
	}
}

// TestMonitorDisabled timeout<=0 不启用
func TestMonitorDisabled(t *testing.T) {
	m := progress.NewMonitor(0)
	if err := m.Check(); err != nil {
		t.Errorf("timeout<=0 不应超时: %v", err)
	}
}
