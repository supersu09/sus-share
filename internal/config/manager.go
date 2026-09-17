package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Manager struct {
	mu     sync.RWMutex
	cfg    Config
	path   string
	fsysis func(*os.File) error
}

func NewManager(path string, cfg Config) *Manager {
	return &Manager{cfg: cfg, path: path, fsysis: fileSync}
}

func (m *Manager) Config() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) UpdateRetention(ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.cfg
	next.Storage.TTL = ttl
	return m.persist(next)
}

func (m *Manager) UpdatePasswordHash(hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ValidateArgon2idHash(hash); err != nil {
		return err
	}
	next := m.cfg
	next.Auth.PasswordHash = hash
	return m.persist(next)
}

func (m *Manager) persist(next Config) error {
	if err := next.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(next)
	if err != nil {
		return fmt.Errorf("serialize config: %w", err)
	}
	dir := filepath.Dir(m.path)
	temp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := m.fsysis(temp); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tempName, m.path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	if dirFile, err := os.Open(dir); err == nil {
		_ = m.fsysis(dirFile)
		_ = dirFile.Close()
	}
	m.cfg = next
	return nil
}

func fileSync(file *os.File) error {
	return file.Sync()
}
