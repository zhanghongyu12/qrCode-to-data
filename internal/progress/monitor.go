package progress

import (
	"errors"
	"sync"
	"time"
)

// ErrTimeout 无新块超时
var ErrTimeout = errors.New("progress: 无新块超时，交换可能卡死（可 Ctrl+C 终止）")

// Monitor 无新块超时监控器。
// timeout<=0 表示不启用超时检测。
type Monitor struct {
	timeout time.Duration
	last    time.Time
	mu      sync.Mutex
}

// NewMonitor 创建监控器，起始时间记为 now
func NewMonitor(timeout time.Duration) *Monitor {
	return &Monitor{timeout: timeout, last: time.Now()}
}

// Notify 收到新块时调用，刷新最后活动时间
func (m *Monitor) Notify() {
	m.mu.Lock()
	m.last = time.Now()
	m.mu.Unlock()
}

// Check 检查是否超时；超时返回 ErrTimeout，否则返回 nil。
func (m *Monitor) Check() error {
	if m.timeout <= 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Since(m.last) > m.timeout {
		return ErrTimeout
	}
	return nil
}

// LastUpdate 返回最后活动时间
func (m *Monitor) LastUpdate() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.last
}
