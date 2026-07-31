package store

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type sessionEntry struct {
	expiresAt time.Time
	ttl       time.Duration
}

// SessionManager 内存会话管理器: 重启即失效, 无需落库
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]sessionEntry
}

func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string]sessionEntry)}
}

// Create 生成随机 token 并记录过期时间
func (sm *SessionManager) Create(ttl time.Duration) string {
	if ttl <= 0 {
		ttl = 3 * time.Hour
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)

	sm.mu.Lock()
	sm.sessions[token] = sessionEntry{expiresAt: time.Now().Add(ttl), ttl: ttl}
	sm.mu.Unlock()
	return token
}

// Validate 校验 token 是否有效; 有效则滑动续期, 过期则删除
func (sm *SessionManager) Validate(token string) bool {
	if token == "" {
		return false
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	entry, ok := sm.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(entry.expiresAt) {
		delete(sm.sessions, token)
		return false
	}
	entry.expiresAt = time.Now().Add(entry.ttl)
	sm.sessions[token] = entry
	return true
}

func (sm *SessionManager) Delete(token string) {
	sm.mu.Lock()
	delete(sm.sessions, token)
	sm.mu.Unlock()
}

// Clear 清除全部会话 (修改密码/停用登录时调用)
func (sm *SessionManager) Clear() {
	sm.mu.Lock()
	sm.sessions = make(map[string]sessionEntry)
	sm.mu.Unlock()
}
