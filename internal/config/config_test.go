package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const validHash = "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHRzYWx0c2FsdA==$aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGFzaGhhc2g="

func baseYAML(t *testing.T) string {
	t.Helper()
	return `server:
  host: 127.0.0.1
  port: 19876
storage:
  path: /tmp/sus-data
  ttl: 24h
auth:
  enabled: true
  password_hash: "` + validHash + `"
limits:
  max_text_size: 1Mi
  max_file_size: 10Mi
  max_storage_size: 20Mi
`
}

func TestParseRejectsInvalidConfig(t *testing.T) {
	cases := []struct {
		name string
		mut  func(string) string
	}{
		{"bad hash", func(s string) string { return strings.Replace(s, validHash, "123456", 1) }},
		{"zero ttl", func(s string) string { return strings.Replace(s, "ttl: 24h", "ttl: 0s", 1) }},
		{"too long ttl", func(s string) string { return strings.Replace(s, "ttl: 24h", "ttl: 720h", 1) }},
		{"oversized text", func(s string) string { return strings.Replace(s, "max_storage_size: 20Mi", "max_storage_size: 1Mi", 1) }},
		{"unknown field", func(s string) string { return s + "\nunknown: true\n" }},
		{"repeated slash in base path", func(s string) string {
			return strings.Replace(s, "port: 19876", "port: 19876\n  base_path: //share", 1)
		}},
		{"backslash in base path", func(s string) string {
			return strings.Replace(s, "port: 19876", "port: 19876\n  base_path: /share\\", 1)
		}},
		{"full URL base path", func(s string) string {
			return strings.Replace(s, "port: 19876", "port: 19876\n  base_path: https://www.suscode.cn/share", 1)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.mut(baseYAML(t)))); err == nil {
				t.Fatal("expected invalid config to fail")
			}
		})
	}
}

func TestNormalizeBasePath(t *testing.T) {
	cases := []struct {
		input  string
		output string
	}{
		{input: "", output: ""},
		{input: "/", output: ""},
		{input: "/share", output: "/share"},
		{input: "/share/", output: "/share"},
		{input: " /share/ ", output: "/share"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := NormalizeBasePath(tc.input)
			if err != nil {
				t.Fatalf("NormalizeBasePath() = %v", err)
			}
			if got != tc.output {
				t.Fatalf("NormalizeBasePath() = %q, want %q", got, tc.output)
			}
		})
	}
}

func TestAuthEnabledAllowsEmptyPasswordHash(t *testing.T) {
	raw := strings.Replace(baseYAML(t), validHash, "", 1)
	cfg, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse() = %v", err)
	}
	if !cfg.Auth.Enabled || cfg.Auth.PasswordHash != "" {
		t.Fatalf("auth config = %+v", cfg.Auth)
	}
}

func TestManagerPersistsAndReloads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("# admin comment\n"+baseYAML(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(path, cfg)
	if err := manager.UpdateRetention(72 * time.Hour); err != nil {
		t.Fatal(err)
	}
	if manager.Config().Storage.TTL != 72*time.Hour {
		t.Fatalf("retention = %s, want 72h", manager.Config().Storage.TTL)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Storage.TTL != 72*time.Hour {
		t.Fatalf("persisted retention = %s, want 72h", reloaded.Storage.TTL)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "admin comment") {
		t.Fatal("expected structured rewrite to discard comments")
	}
}

func TestManagerWriteFailureKeepsOldConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing", "config.yaml")
	cfg, err := Parse([]byte(baseYAML(t)))
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(path, cfg)
	if err := manager.UpdateRetention(7 * 24 * time.Hour); err == nil {
		t.Fatal("expected write failure")
	}
	if manager.Config().Storage.TTL != 24*time.Hour {
		t.Fatalf("active retention changed to %s", manager.Config().Storage.TTL)
	}
}
