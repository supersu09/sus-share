package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	MinRetention = time.Hour
	MaxRetention = 7 * 24 * time.Hour
)

var argon2idPattern = regexp.MustCompile(`^\$argon2id\$v=19\$m=(\d+),t=(\d+),p=(\d+)\$([A-Za-z0-9+/=]+)\$([A-Za-z0-9+/=]+)$`)
var byteSizePattern = regexp.MustCompile(`^(\d+)(B|Ki|Mi|Gi|Ti|KiB|MiB|GiB|TiB|KB|MB|GB|TB)$`)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
	Auth    AuthConfig    `yaml:"auth"`
	Limits  LimitsConfig  `yaml:"limits"`
	Web     WebConfig     `yaml:"web"`
	Log     LogConfig     `yaml:"log"`
}

type ServerConfig struct {
	Host     string    `yaml:"host"`
	Port     int       `yaml:"port"`
	BasePath string    `yaml:"base_path"`
	TLS      TLSConfig `yaml:"tls"`
}

func NormalizeBasePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return "", nil
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") ||
		strings.Contains(value, "\\") || strings.Contains(value, ":") {
		return "", errors.New("server.base_path must be an absolute path prefix without a scheme")
	}
	value = strings.TrimSuffix(value, "/")
	if strings.Contains(value, "//") || value == "" {
		return "", errors.New("server.base_path must not contain repeated slashes")
	}
	return value, nil
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type StorageConfig struct {
	Path string        `yaml:"path"`
	TTL  time.Duration `yaml:"ttl"`
}

type AuthConfig struct {
	Enabled      bool   `yaml:"enabled"`
	PasswordHash string `yaml:"password_hash"`
}

type LimitsConfig struct {
	MaxTextSize    ByteSize `yaml:"max_text_size"`
	MaxFileSize    ByteSize `yaml:"max_file_size"`
	MaxStorageSize ByteSize `yaml:"max_storage_size"`
}

type ByteSize uint64

func (size *ByteSize) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return errors.New("must be a byte-size scalar")
	}
	raw := value.Value
	if number, err := strconv.ParseUint(raw, 10, 64); err == nil {
		*size = ByteSize(number)
		return nil
	}
	matches := byteSizePattern.FindStringSubmatch(raw)
	if matches == nil {
		return fmt.Errorf("invalid byte size %q", raw)
	}
	number, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid byte size %q: %w", raw, err)
	}
	multipliers := map[string]uint64{
		"B": 1, "Ki": 1024, "Mi": 1024 * 1024, "Gi": 1024 * 1024 * 1024,
		"Ti": 1024 * 1024 * 1024 * 1024, "KiB": 1024, "MiB": 1024 * 1024,
		"GiB": 1024 * 1024 * 1024, "TiB": 1024 * 1024 * 1024 * 1024,
		"KB": 1000, "MB": 1000 * 1000,
		"GB": 1000 * 1000 * 1000, "TB": 1000 * 1000 * 1000 * 1000,
	}
	if number > ^uint64(0)/multipliers[matches[2]] {
		return fmt.Errorf("byte size %q overflows uint64", raw)
	}
	*size = ByteSize(number * multipliers[matches[2]])
	return nil
}

type WebConfig struct {
	Title string `yaml:"title"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{Host: "0.0.0.0", Port: 19876},
		Storage: StorageConfig{
			Path: "/data",
			TTL:  24 * time.Hour,
		},
		Auth: AuthConfig{Enabled: true},
		Limits: LimitsConfig{
			MaxTextSize:    1024 * 1024,
			MaxFileSize:    5 * 1024 * 1024 * 1024,
			MaxStorageSize: 10 * 1024 * 1024 * 1024,
		},
		Web: WebConfig{Title: "SusShare"},
		Log: LogConfig{Level: "info"},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	cfg, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func Parse(data []byte) (Config, error) {
	cfg := Default()
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		if strings.Contains(err.Error(), "EOF") {
			return cfg, cfg.Validate()
		}
		return Config{}, fmt.Errorf("parse YAML: %w", err)
	}
	basePath, err := NormalizeBasePath(cfg.Server.BasePath)
	if err != nil {
		return Config{}, fmt.Errorf("server.base_path: %w", err)
	}
	cfg.Server.BasePath = basePath
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.Server.Host == "" {
		return errors.New("server.host must not be empty")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return errors.New("server.port must be between 1 and 65535")
	}
	if _, err := NormalizeBasePath(c.Server.BasePath); err != nil {
		return fmt.Errorf("server.base_path: %w", err)
	}
	if c.Server.TLS.Enabled {
		if c.Server.TLS.CertFile == "" || c.Server.TLS.KeyFile == "" {
			return errors.New("server.tls cert_file and key_file are required when TLS is enabled")
		}
	}
	if c.Storage.Path == "" {
		return errors.New("storage.path must not be empty")
	}
	if c.Storage.TTL < MinRetention || c.Storage.TTL > MaxRetention {
		return fmt.Errorf("storage.ttl must be between %s and %s", MinRetention, MaxRetention)
	}
	if c.Auth.Enabled && strings.TrimSpace(c.Auth.PasswordHash) != "" {
		if err := ValidateArgon2idHash(c.Auth.PasswordHash); err != nil {
			return fmt.Errorf("auth.password_hash: %w", err)
		}
	}
	if c.Limits.MaxTextSize == 0 {
		return errors.New("limits.max_text_size must be greater than zero")
	}
	if c.Limits.MaxFileSize == 0 {
		return errors.New("limits.max_file_size must be greater than zero")
	}
	if c.Limits.MaxStorageSize < c.Limits.MaxTextSize || c.Limits.MaxStorageSize < c.Limits.MaxFileSize {
		return errors.New("limits.max_storage_size must cover max_text_size and max_file_size")
	}
	if c.Web.Title == "" {
		return errors.New("web.title must not be empty")
	}
	switch strings.ToLower(c.Log.Level) {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("log.level must be debug, info, warn, or error")
	}
	return nil
}

func ValidateArgon2idHash(hash string) error {
	matches := argon2idPattern.FindStringSubmatch(strings.TrimSpace(hash))
	if matches == nil {
		return errors.New("must be an Argon2id PHC string")
	}
	m, err := strconv.Atoi(matches[1])
	if err != nil || m < 8 {
		return errors.New("Argon2id memory parameter is too weak")
	}
	t, err := strconv.Atoi(matches[2])
	if err != nil || t < 1 {
		return errors.New("Argon2id iterations parameter is too weak")
	}
	p, err := strconv.Atoi(matches[3])
	if err != nil || p < 1 {
		return errors.New("Argon2id parallelism parameter is too weak")
	}
	if _, err := base64.StdEncoding.DecodeString(matches[4]); err != nil {
		return fmt.Errorf("invalid salt: %w", err)
	}
	if key, err := base64.StdEncoding.DecodeString(matches[5]); err != nil || len(key) < 16 {
		return errors.New("invalid hash output")
	}
	return nil
}
