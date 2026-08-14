package frame

import (
	"sync"
)

// Session 表示一个传输会话，按 transfer_id 区分
type Session struct {
	TransferID [16]byte
	Meta       *MetaData
	received   map[uint32]bool
	mu         sync.RWMutex
}

// NewSession 创建新会话
func NewSession(transferID [16]byte) *Session {
	return &Session{
		TransferID: transferID,
		received:   make(map[uint32]bool),
	}
}

// SetMeta 设置会话元数据
func (s *Session) SetMeta(meta *MetaData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Meta = meta
}

// HasMeta 检查是否已收到元数据
func (s *Session) HasMeta() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Meta != nil
}

// MarkReceived 标记 seq 已接收，返回 false 表示重复
func (s *Session) MarkReceived(seq uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.received[seq] {
		return false
	}
	s.received[seq] = true
	return true
}

// IsReceived 检查 seq 是否已接收
func (s *Session) IsReceived(seq uint32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.received[seq]
}

// ReceivedCount 返回已接收的唯一 seq 数量
func (s *Session) ReceivedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.received)
}

// SessionManager 管理多个传输会话，按 transfer_id 区分
type SessionManager struct {
	sessions map[[16]byte]*Session
	mu       sync.RWMutex
}

// NewSessionManager 创建会话管理器
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[[16]byte]*Session),
	}
}

// GetOrCreate 获取或创建指定 transfer_id 的会话
func (sm *SessionManager) GetOrCreate(transferID [16]byte) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.sessions[transferID]
	if !ok {
		s = NewSession(transferID)
		sm.sessions[transferID] = s
	}
	return s
}

// Get 获取指定 transfer_id 的会话，不存在返回 nil
func (sm *SessionManager) Get(transferID [16]byte) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[transferID]
}

// Remove 移除会话
func (sm *SessionManager) Remove(transferID [16]byte) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, transferID)
}