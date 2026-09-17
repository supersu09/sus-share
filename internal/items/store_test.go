package items

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/trixie/sus-share/internal/config"
)

func testOptions(root string, now *time.Time, limits config.LimitsConfig) Options {
	return Options{
		Root: root,
		Now:  func() time.Time { return *now },
		Retention: func() time.Duration {
			return time.Hour
		},
		Limits: func() config.LimitsConfig { return limits },
	}
}

func testLimits() config.LimitsConfig {
	return config.LimitsConfig{MaxTextSize: 16, MaxFileSize: 10, MaxStorageSize: 20}
}

func TestOpenCleansStaging(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".staging")
	if err := os.MkdirAll(filepath.Join(staging, "unfinished"), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	store, err := Open(testOptions(root, &now, testLimits()))
	if err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(staging); len(entries) != 0 {
		t.Fatalf("staging entries = %d, want 0", len(entries))
	}
	if len(store.List()) != 0 {
		t.Fatal("unfinished item became visible")
	}
}

func TestRecoveryQuarantinesCorruptItems(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	id1 := "11111111111111111111111111111111"
	id2 := "22222222222222222222222222222222"
	valid := Metadata{
		ID: id1, Kind: KindText, Title: "good", ContentType: "text/plain; charset=utf-8",
		Preview: "text", Size: 4, CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	writeFinalItem(t, root, id1, valid, []byte("good"))
	writeRawFinalItem(t, root, "33333333333333333333333333333333", []byte("{bad"), []byte("x"))
	duplicate := valid
	duplicateData, err := json.Marshal(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	writeRawFinalItem(t, root, id2, duplicateData, []byte("good"))

	store, err := Open(testOptions(root, &now, testLimits()))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(store.List()); got != 1 {
		t.Fatalf("visible items = %d, want 1", got)
	}
	if _, err := os.Stat(filepath.Join(store.quarantine, id2)); err != nil {
		t.Fatalf("duplicate item was not quarantined: %v", err)
	}
}

func TestCreateTextAndFilesEnforceLimits(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	store, err := Open(testOptions(root, &now, testLimits()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateText("too long", "12345678901234567"); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("text limit error = %v", err)
	}
	if _, err := store.CreateFile("big", "big.bin", "application/octet-stream", 11, bytes.NewReader(make([]byte, 11))); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("file limit error = %v", err)
	}
	if got := len(store.List()); got != 0 {
		t.Fatalf("visible rejected items = %d, want 0", got)
	}
}

func TestConcurrentUploadsCannotExceedStorage(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	store, err := Open(testOptions(root, &now, testLimits()))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.CreateFile(fmt.Sprintf("file-%d", i), "file.bin", "application/octet-stream", 10, bytes.NewReader(make([]byte, 10)))
			if err == nil {
				mu.Lock()
				created++
				mu.Unlock()
			} else if !errors.Is(err, ErrLimitExceeded) {
				t.Errorf("upload error = %v", err)
			}
		}(i)
	}
	wg.Wait()
	if created != 2 {
		t.Fatalf("created = %d, want 2", created)
	}
	entries, _ := os.ReadDir(store.itemsDir)
	if len(entries) != 2 {
		t.Fatalf("final item directories = %d, want 2", len(entries))
	}
}

func TestListReadDeleteAndExpiry(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	store, err := Open(testOptions(root, &now, testLimits()))
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.CreateText("first", "one")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	second, err := store.CreateText("second", "two")
	if err != nil {
		t.Fatal(err)
	}
	list := store.List()
	if list[0].ID != second.ID || list[1].ID != first.ID {
		t.Fatalf("list order = [%s %s]", list[0].ID, list[1].ID)
	}
	meta, file, err := store.Content(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	if meta.Title != "first" {
		t.Fatalf("title = %q", meta.Title)
	}
	if err := store.Delete(first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted get error = %v", err)
	}
	now = now.Add(2 * time.Hour)
	if got := store.List(); len(got) != 0 {
		t.Fatalf("expired visible items = %d", len(got))
	}
	if _, _, err := store.Content(second.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired content error = %v", err)
	}
	if cleaned := store.CleanupExpired(); cleaned != 1 {
		t.Fatalf("cleaned = %d, want 1", cleaned)
	}
	if _, err := os.Stat(filepath.Join(store.itemsDir, second.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired directory still exists: %v", err)
	}
}

func writeFinalItem(t *testing.T, root, id string, meta Metadata, content []byte) {
	t.Helper()
	meta.ID = id
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	writeRawFinalItem(t, root, id, data, content)
}

func writeRawFinalItem(t *testing.T, root, id string, metadata, content []byte) {
	t.Helper()
	dir := filepath.Join(root, "items", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), metadata, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "content"), content, 0o600); err != nil {
		t.Fatal(err)
	}
}
