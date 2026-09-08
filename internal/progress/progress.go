package progress

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// Kind 进度事件类型
type Kind int

const (
	// Send 播放端进度
	Send Kind = iota
	// Receive 还原端进度
	Receive
)

// String 返回中文标签
func (k Kind) String() string {
	if k == Send {
		return "播放"
	}
	return "还原"
}

// Event 统一进度事件，供 send/receive 编排层调用
type Event struct {
	// Kind 事件类型（播放/还原）
	Kind Kind
	// Count send: 已播块数；receive: 唯一已收块数
	Count int
	// Total send: 总块数；receive: 期望块数
	Total int
	// Dedup 去重丢弃数（仅 receive）
	Dedup int
	// Done 是否完成
	Done bool
	// Message 最终结果消息（Finish 时输出）
	Message string
}

// Handler 统一进度事件接口
type Handler interface {
	// Report 处理进度事件
	Report(e Event)
	// Finish 输出最终结果（quiet 模式下仅输出结果）
	Finish(e Event)
	// Close 释放资源（输出换行等）
	Close()
}

// Terminal 终端进度展示器。
// 进度输出到 w（调用方传入 stdout）；错误由调用方输出到 stderr。
type Terminal struct {
	out     io.Writer
	quiet   bool
	start   time.Time
	lastAt  time.Time
	mu      sync.Mutex
	closed  bool
}

// NewTerminal 创建终端进度展示器。
// quiet=true 时 Report 不输出，仅 Finish 输出最终结果。
func NewTerminal(w io.Writer, quiet bool) *Terminal {
	now := time.Now()
	return &Terminal{out: w, quiet: quiet, start: now, lastAt: now}
}

// Report 输出进度行。
// 使用 \r 复位光标 + 清行，同一行原地刷新，不滚动累积。
func (t *Terminal) Report(e Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastAt = time.Now()
	if t.quiet || t.out == nil {
		return
	}
	elapsed := time.Since(t.start)
	line := FormatEvent(e, elapsed)
	fmt.Fprintf(t.out, "\r\x1b[K%s", line)
}

// Finish 输出最终结果（quiet 模式也输出）。
func (t *Terminal) Finish(e Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.out == nil {
		return
	}
	if t.quiet {
		if e.Message != "" {
			fmt.Fprintln(t.out, e.Message)
		}
		return
	}
	elapsed := time.Since(t.start)
	line := FormatEvent(e, elapsed)
	fmt.Fprintf(t.out, "\r\x1b[K%s\n", line)
	if e.Message != "" {
		fmt.Fprintln(t.out, e.Message)
	}
}

// Close 输出换行，结束进度刷新（幂等）
func (t *Terminal) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.out == nil {
		return
	}
	t.closed = true
	if !t.quiet {
		fmt.Fprintln(t.out)
	}
}

// LastUpdate 返回最近一次 Report 的时间（供超时检测参考）
func (t *Terminal) LastUpdate() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastAt
}

// FormatEvent 将进度事件格式化为一行文本（不含 ANSI 控制序列，便于测试）。
// 播放端：播放: 45/100 块 | 45.0% | 12.3 fps | 剩余约 5s
// 还原端：还原: 45/100 块 | 45.0% | 去重 3 | 12.3 块/s
func FormatEvent(e Event, elapsed time.Duration) string {
	percent := Percent(e.Count, e.Total)
	rate := Rate(e.Count, elapsed)
	var b strings.Builder
	if e.Kind == Send {
		fmt.Fprintf(&b, "播放: %d/%d 块 | %.1f%% | %.1f fps", e.Count, e.Total, percent, rate)
		if e.Total > e.Count {
			fmt.Fprintf(&b, " | 剩余约 %.0fs", ETA(e.Count, e.Total, elapsed))
		}
	} else {
		fmt.Fprintf(&b, "还原: %d/%d 块 | %.1f%% | 去重 %d | %.1f 块/s", e.Count, e.Total, percent, e.Dedup, rate)
	}
	return b.String()
}
