package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

const (
	CookieName       = "sus_share_session"
	idleLifetime     = 12 * time.Hour
	absoluteLifetime = 7 * 24 * time.Hour
)

type session struct {
	expiresAt  time.Time
	absoluteAt time.Time
}

type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]session
	now      func() time.Time
}

func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string]session), now: time.Now}
}

func (m *SessionManager) Create() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(data)
	now := m.now()
	m.mu.Lock()
	m.sessions[token] = session{expiresAt: now.Add(idleLifetime), absoluteAt: now.Add(absoluteLifetime)}
	m.mu.Unlock()
	return token, nil
}

func (m *SessionManager) Validate(token string) bool {
	if token == "" {
		return false
	}
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.sessions[token]
	if !ok || now.After(current.expiresAt) || now.After(current.absoluteAt) {
		delete(m.sessions, token)
		return false
	}
	current.expiresAt = now.Add(idleLifetime)
	m.sessions[token] = current
	return true
}

func (m *SessionManager) Delete(token string) {
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
}

func (m *SessionManager) InvalidateAll() {
	m.mu.Lock()
	m.sessions = make(map[string]session)
	m.mu.Unlock()
}

func (m *SessionManager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}
