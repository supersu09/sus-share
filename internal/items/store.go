package items

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/trixie/sus-share/internal/config"
	"github.com/trixie/sus-share/internal/preview"
)

var (
	ErrNotFound      = errors.New("item not found")
	ErrLimitExceeded = errors.New("limit exceeded")
	idPattern        = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

type Kind string

const (
	KindText Kind = "text"
	KindFile Kind = "file"
)

type Metadata struct {
	ID          string    `json:"id"`
	Kind        Kind      `json:"kind"`
	Title       string    `json:"title"`
	Filename    string    `json:"filename,omitempty"`
	ContentType string    `json:"content_type"`
	Preview     string    `json:"preview"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (m Metadata) Clone() Metadata { return m }

type Store struct {
	mu         sync.RWMutex
	root       string
	itemsDir   string
	staging    string
	quarantine string
	entries    map[string]Metadata
	used       int64
	reserved   int64
	now        func() time.Time
	retention  func() time.Duration
	limits     func() config.LimitsConfig
}

type Options struct {
	Root      string
	Now       func() time.Time
	Retention func() time.Duration
	Limits    func() config.LimitsConfig
}

func (s *Store) Root() string {
	return s.root
}

func Open(options Options) (*Store, error) {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Retention == nil {
		return nil, errors.New("items: retention provider is required")
	}
	if options.Limits == nil {
		return nil, errors.New("items: limits provider is required")
	}
	store := &Store{
		root: options.Root, itemsDir: filepath.Join(options.Root, "items"),
		staging: filepath.Join(options.Root, ".staging"), quarantine: filepath.Join(options.Root, "quarantine"),
		entries: make(map[string]Metadata), now: options.Now, retention: options.Retention, limits: options.Limits,
	}
	for _, dir := range []string{store.itemsDir, store.staging, store.quarantine} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("items: create directory: %w", err)
		}
	}
	if err := store.recover(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) recover() error {
	stagingEntries, err := os.ReadDir(s.staging)
	if err != nil {
		return fmt.Errorf("items: read staging: %w", err)
	}
	for _, entry := range stagingEntries {
		if err := os.RemoveAll(filepath.Join(s.staging, entry.Name())); err != nil {
			return fmt.Errorf("items: clean staging: %w", err)
		}
	}
	finalEntries, err := os.ReadDir(s.itemsDir)
	if err != nil {
		return fmt.Errorf("items: read final items: %w", err)
	}
	seen := make(map[string]struct{})
	for _, dirEntry := range finalEntries {
		name := dirEntry.Name()
		path := filepath.Join(s.itemsDir, name)
		if !dirEntry.IsDir() {
			if err := s.quarantineItem(path, "not-directory"); err != nil {
				return err
			}
			continue
		}
		meta, contentSize, reason, err := readFinalItem(path)
		if err != nil {
			reason = fmt.Sprintf("read item: %v", err)
		} else if _, duplicate := seen[meta.ID]; duplicate {
			reason = "duplicate id"
		} else if meta.ID != name {
			reason = "directory id mismatch"
		}
		if reason != "" {
			if err := s.quarantineItem(path, reason); err != nil {
				return err
			}
			continue
		}
		seen[meta.ID] = struct{}{}
		s.entries[meta.ID] = meta
		s.used += contentSize
	}
	return nil
}

func readFinalItem(path string) (Metadata, int64, string, error) {
	metaPath := filepath.Join(path, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return Metadata{}, 0, "missing metadata", err
	}
	var meta Metadata
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&meta); err != nil {
		return Metadata{}, 0, "invalid metadata JSON", err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Metadata{}, 0, "trailing metadata JSON", err
	}
	info, err := os.Stat(filepath.Join(path, "content"))
	if err != nil {
		return Metadata{}, 0, "missing content", err
	}
	if !info.Mode().IsRegular() || info.Size() != meta.Size || meta.Size < 0 {
		return Metadata{}, 0, "invalid content", nil
	}
	if !idPattern.MatchString(meta.ID) || (meta.Kind != KindText && meta.Kind != KindFile) {
		return Metadata{}, 0, "invalid metadata", nil
	}
	if meta.CreatedAt.IsZero() || meta.ExpiresAt.Before(meta.CreatedAt) || meta.ContentType == "" || meta.Preview == "" {
		return Metadata{}, 0, "invalid metadata", nil
	}
	return meta, info.Size(), "", nil
}

func (s *Store) quarantineItem(path, reason string) error {
	if err := os.MkdirAll(s.quarantine, 0o700); err != nil {
		return err
	}
	base := filepath.Base(path)
	target := filepath.Join(s.quarantine, base)
	for suffix := 1; ; suffix++ {
		if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
			break
		} else if err != nil {
			return err
		}
		target = filepath.Join(s.quarantine, fmt.Sprintf("%s.%d", base, suffix))
	}
	if err := os.Rename(path, target); err != nil {
		return fmt.Errorf("items: quarantine %s: %w", reason, err)
	}
	return nil
}

func (s *Store) CreateText(title, content string) (Metadata, error) {
	if !utf8.ValidString(title) || !utf8.ValidString(content) {
		return Metadata{}, errors.New("items: text must be valid UTF-8")
	}
	title = cleanTitle(title)
	if title == "" {
		title = "Text"
	}
	data := []byte(content)
	if int64(len(data)) > int64(s.limits().MaxTextSize) {
		return Metadata{}, ErrLimitExceeded
	}
	created := s.now()
	meta := Metadata{
		Kind: KindText, Title: title, ContentType: "text/plain; charset=utf-8", Preview: string(preview.Text),
		Size: int64(len(data)), CreatedAt: created.UTC(), ExpiresAt: created.Add(s.retention()).UTC(),
	}
	return s.create(meta, bytes.NewReader(data))
}

func (s *Store) CreateFile(title, filename, contentType string, size int64, content io.Reader) (Metadata, error) {
	if size < 0 {
		return Metadata{}, errors.New("items: file size is invalid")
	}
	if size > int64(s.limits().MaxFileSize) {
		return Metadata{}, ErrLimitExceeded
	}
	filename = cleanFilename(filename)
	title = cleanTitle(title)
	if title == "" {
		title = filename
	}
	head := make([]byte, 512)
	headSize, err := io.ReadFull(content, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return Metadata{}, err
	}
	head = head[:headSize]
	if int64(headSize) > size {
		return Metadata{}, errors.New("items: file is larger than declared size")
	}
	classification := preview.Classify(filename, contentType, head)
	created := s.now()
	meta := Metadata{
		Kind: KindFile, Title: title, Filename: filename, ContentType: classification.ContentType,
		Preview: string(classification.Kind), Size: size, CreatedAt: created.UTC(),
		ExpiresAt: created.Add(s.retention()).UTC(),
	}
	return s.create(meta, io.MultiReader(bytes.NewReader(head), content))
}

func (s *Store) CreateFileStream(title, filename, contentType string, content io.Reader) (Metadata, error) {
	filename = cleanFilename(filename)
	title = cleanTitle(title)
	if title == "" {
		title = filename
	}
	head := make([]byte, 512)
	headSize, err := io.ReadFull(content, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return Metadata{}, err
	}
	head = head[:headSize]
	limits := s.limits()
	classification := preview.Classify(filename, contentType, head)
	created := s.now()
	id, err := newID()
	if err != nil {
		return Metadata{}, err
	}
	if err := s.reserve(int64(limits.MaxFileSize)); err != nil {
		return Metadata{}, err
	}
	stagingPath := filepath.Join(s.staging, id)
	if err := os.MkdirAll(stagingPath, 0o700); err != nil {
		s.release(int64(limits.MaxFileSize))
		return Metadata{}, err
	}
	contentFile, err := os.OpenFile(filepath.Join(stagingPath, "content"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		_ = os.RemoveAll(stagingPath)
		s.release(int64(limits.MaxFileSize))
		return Metadata{}, err
	}
	written, copyErr := io.Copy(contentFile, io.MultiReader(bytes.NewReader(head), content))
	if copyErr == nil && written > int64(limits.MaxFileSize) {
		copyErr = ErrLimitExceeded
	}
	if copyErr == nil {
		copyErr = contentFile.Sync()
	}
	if closeErr := contentFile.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		_ = os.RemoveAll(stagingPath)
		s.release(int64(limits.MaxFileSize))
		return Metadata{}, copyErr
	}
	meta := Metadata{
		ID: id, Kind: KindFile, Title: title, Filename: filename, ContentType: classification.ContentType,
		Preview: string(classification.Kind), Size: written, CreatedAt: created.UTC(),
		ExpiresAt: created.Add(s.retention()).UTC(),
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err == nil {
		err = os.WriteFile(filepath.Join(stagingPath, "meta.json"), data, 0o600)
	}
	if err == nil {
		syncDir(stagingPath)
		err = os.Rename(stagingPath, filepath.Join(s.itemsDir, id))
	}
	if err != nil {
		_ = os.RemoveAll(stagingPath)
		s.release(int64(limits.MaxFileSize))
		return Metadata{}, err
	}
	s.release(int64(limits.MaxFileSize))
	s.mu.Lock()
	s.entries[id] = meta
	s.used += written
	s.mu.Unlock()
	return meta.Clone(), nil
}

func (s *Store) create(meta Metadata, content io.Reader) (Metadata, error) {
	id, err := newID()
	if err != nil {
		return Metadata{}, err
	}
	meta.ID = id
	if err := s.reserve(meta.Size); err != nil {
		return Metadata{}, err
	}
	stagingPath := filepath.Join(s.staging, id)
	if err := os.MkdirAll(stagingPath, 0o700); err != nil {
		s.release(meta.Size)
		return Metadata{}, fmt.Errorf("items: create staging: %w", err)
	}
	if err := writeContentAndMetadata(stagingPath, meta, content); err != nil {
		_ = os.RemoveAll(stagingPath)
		s.release(meta.Size)
		return Metadata{}, err
	}
	finalPath := filepath.Join(s.itemsDir, id)
	if err := os.Rename(stagingPath, finalPath); err != nil {
		_ = os.RemoveAll(stagingPath)
		s.release(meta.Size)
		return Metadata{}, fmt.Errorf("items: promote item: %w", err)
	}
	syncDir(s.itemsDir)
	s.mu.Lock()
	s.entries[id] = meta
	s.used += meta.Size
	s.mu.Unlock()
	return meta.Clone(), nil
}

func writeContentAndMetadata(path string, meta Metadata, content io.Reader) error {
	contentFile, err := os.OpenFile(filepath.Join(path, "content"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("items: create content: %w", err)
	}
	written, err := io.Copy(contentFile, content)
	if err == nil && written != meta.Size {
		err = fmt.Errorf("items: wrote %d bytes, expected %d", written, meta.Size)
	}
	if err == nil {
		err = contentFile.Sync()
	}
	if closeErr := contentFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	metaTemp := filepath.Join(path, ".meta.json.tmp")
	metaFile, err := os.OpenFile(metaTemp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("items: write metadata: %w", err)
	}
	if _, err := metaFile.Write(data); err == nil {
		err = metaFile.Sync()
	}
	if closeErr := metaFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("items: write metadata: %w", err)
	}
	if err := os.Rename(metaTemp, filepath.Join(path, "meta.json")); err != nil {
		return fmt.Errorf("items: finalize metadata: %w", err)
	}
	syncDir(path)
	return nil
}

func (s *Store) reserve(size int64) error {
	limits := s.limits()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used+s.reserved+size > int64(limits.MaxStorageSize) {
		return ErrLimitExceeded
	}
	s.reserved += size
	return nil
}

func (s *Store) release(size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reserved -= size
	if s.reserved < 0 {
		s.reserved = 0
	}
}

func (s *Store) List() []Metadata {
	now := s.now()
	s.mu.RLock()
	result := make([]Metadata, 0, len(s.entries))
	for _, meta := range s.entries {
		if meta.ExpiresAt.After(now) {
			result = append(result, meta.Clone())
		}
	}
	s.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID > result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

func (s *Store) Get(id string) (Metadata, error) {
	if !idPattern.MatchString(id) {
		return Metadata{}, ErrNotFound
	}
	s.mu.RLock()
	meta, ok := s.entries[id]
	s.mu.RUnlock()
	if !ok || !meta.ExpiresAt.After(s.now()) {
		return Metadata{}, ErrNotFound
	}
	return meta.Clone(), nil
}

func (s *Store) Content(id string) (Metadata, *os.File, error) {
	meta, err := s.Get(id)
	if err != nil {
		return Metadata{}, nil, err
	}
	file, err := os.Open(filepath.Join(s.itemsDir, id, "content"))
	if err != nil {
		return Metadata{}, nil, err
	}
	return meta, file, nil
}

func (s *Store) Delete(id string) error {
	if !idPattern.MatchString(id) {
		return ErrNotFound
	}
	s.mu.Lock()
	meta, ok := s.entries[id]
	if !ok || !meta.ExpiresAt.After(s.now()) {
		s.mu.Unlock()
		return ErrNotFound
	}
	delete(s.entries, id)
	s.used -= meta.Size
	if s.used < 0 {
		s.used = 0
	}
	s.mu.Unlock()
	if err := os.RemoveAll(filepath.Join(s.itemsDir, id)); err != nil {
		s.mu.Lock()
		s.entries[id] = meta
		s.used += meta.Size
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *Store) CleanupExpired() int {
	now := s.now()
	expired := make([]string, 0)
	s.mu.Lock()
	for id, meta := range s.entries {
		if !meta.ExpiresAt.After(now) {
			expired = append(expired, id)
			delete(s.entries, id)
			s.used -= meta.Size
		}
	}
	s.mu.Unlock()
	for _, id := range expired {
		_ = os.RemoveAll(filepath.Join(s.itemsDir, id))
	}
	if s.used < 0 {
		s.used = 0
	}
	return len(expired)
}

func (s *Store) RunCleanup(ctx interface {
	Done() <-chan struct{}
}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CleanupExpired()
		}
	}
}

func newID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("items: generate id: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func cleanTitle(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, strings.TrimSpace(value))
	if runes := []rune(value); len(runes) > 200 {
		value = strings.TrimSpace(string(runes[:200]))
	}
	return value
}

func cleanFilename(value string) string {
	value = filepath.Base(strings.ReplaceAll(value, "\\", "/"))
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '/' || r == '\\' {
			return '-'
		}
		return r
	}, strings.TrimSpace(value))
	if value == "." || value == ".." || value == "" {
		return "file"
	}
	if runes := []rune(value); len(runes) > 255 {
		extension := filepath.Ext(value)
		base := strings.TrimSuffix(value, extension)
		maxBase := len(runes) - len([]rune(extension))
		if maxBase < 1 {
			return string(runes[:255])
		}
		value = string([]rune(base)[:maxBase]) + extension
	}
	return value
}

func syncDir(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	_ = file.Sync()
	_ = file.Close()
}
