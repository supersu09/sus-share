package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/trixie/sus-share/internal/api"
	"github.com/trixie/sus-share/internal/auth"
	"github.com/trixie/sus-share/internal/config"
	"github.com/trixie/sus-share/internal/items"
	"github.com/trixie/sus-share/internal/router"
	"gopkg.in/yaml.v3"
)

var pngFile = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func TestEndToEnd(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	configPath := filepath.Join(temp, "config.yaml")
	hash, err := auth.GenerateHash("correct-password", auth.Argon2Params{
		Memory: 8, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16,
	})
	if err != nil {
		t.Fatalf("generate hash: %v", err)
	}
	raw := config.Config{
		Server:  config.ServerConfig{Host: "127.0.0.1", Port: 19876},
		Storage: config.StorageConfig{Path: filepath.Join(temp, "data"), TTL: time.Hour},
		Auth:    config.AuthConfig{Enabled: true, PasswordHash: hash},
		Limits: config.LimitsConfig{
			MaxTextSize:    1024,
			MaxFileSize:    64 * 1024,
			MaxStorageSize: 128 * 1024,
		},
		Web: config.WebConfig{Title: "SusShare"},
		Log: config.LogConfig{Level: "info"},
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := config.Load(configPath); err != nil {
		t.Fatalf("load config: %v", err)
	}

	service, _ := start(t, configPath)
	first := request(t, service, httptest.NewRequest("GET", "/api/v1/items", nil), "")
	if first.Code != 401 {
		t.Fatalf("unauthenticated status = %d, want 401", first.Code)
	}

	firstSession := login(t, service, "correct-password")
	text := publishText(t, service, firstSession, "SELECT id FROM users;", "SELECT id FROM users;")
	file := uploadFile(t, service, firstSession)

	list := getList(t, service, firstSession)
	if len(list) != 2 || list[0].ID != file.ID || list[1].ID != text.ID {
		t.Fatalf("list = %#v, want newest file first then text", list)
	}

	assertPreview(t, service, firstSession, file.ID)
	assertDownload(t, service, firstSession, file.ID, pngFile)

	deleted := request(t, service, httptest.NewRequest("DELETE", "/api/v1/items/"+file.ID, nil), firstSession)
	if deleted.Code != 200 {
		t.Fatalf("delete status = %d, body = %s", deleted.Code, deleted.Body.String())
	}
	missing := request(t, service, httptest.NewRequest("GET", "/api/v1/items/"+file.ID+"/content", nil), firstSession)
	if missing.Code != 404 {
		t.Fatalf("deleted content status = %d, want 404", missing.Code)
	}

	service, _ = start(t, configPath)
	if status := request(t, service, httptest.NewRequest("GET", "/api/v1/items", nil), firstSession).Code; status != 401 {
		t.Fatalf("old session status = %d, want 401", status)
	}
	secondSession := login(t, service, "correct-password")
	list = getList(t, service, secondSession)
	if len(list) != 1 || list[0].ID != text.ID {
		t.Fatalf("recovered list = %#v, want text item only", list)
	}

	textPath := filepath.Join(raw.Storage.Path, "items", text.ID, "meta.json")
	metaData, err := os.ReadFile(textPath)
	if err != nil {
		t.Fatalf("read item metadata: %v", err)
	}
	var storedMeta items.Metadata
	if err := json.Unmarshal(metaData, &storedMeta); err != nil {
		t.Fatalf("unmarshal item metadata: %v", err)
	}
	storedMeta.CreatedAt = storedMeta.CreatedAt.Add(-2 * time.Hour)
	storedMeta.ExpiresAt = storedMeta.CreatedAt.Add(time.Hour)
	metaData, err = json.Marshal(storedMeta)
	if err != nil {
		t.Fatalf("marshal expired metadata: %v", err)
	}
	if err := os.WriteFile(textPath, metaData, 0o600); err != nil {
		t.Fatalf("write expired metadata: %v", err)
	}

	service, itemStore := start(t, configPath)
	thirdSession := login(t, service, "correct-password")
	if list := getList(t, service, thirdSession); len(list) != 0 {
		t.Fatalf("expired list = %#v, want empty", list)
	}
	if count := itemStore.CleanupExpired(); count != 1 {
		t.Fatalf("cleanup removed %d items, want 1", count)
	}
	if _, err := os.Stat(textPath); !os.IsNotExist(err) {
		t.Fatalf("expired item still exists: %v", err)
	}
}

func TestSubpathDefaultPasswordAndCreationResponses(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	configPath := filepath.Join(temp, "config.yaml")
	raw := config.Config{
		Server:  config.ServerConfig{Host: "127.0.0.1", Port: 19876, BasePath: "/share"},
		Storage: config.StorageConfig{Path: filepath.Join(temp, "data"), TTL: time.Hour},
		Auth:    config.AuthConfig{Enabled: true},
		Limits: config.LimitsConfig{
			MaxTextSize:    1024,
			MaxFileSize:    64 * 1024,
			MaxStorageSize: 128 * 1024,
		},
		Web: config.WebConfig{Title: "SusShare"},
		Log: config.LogConfig{Level: "info"},
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	service, _ := start(t, configPath)
	handler := router.New(cfg.Server.BasePath, service.Handler())

	send := func(method, path, body, contentType, session string) *httptest.ResponseRecorder {
		t.Helper()
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		request := httptest.NewRequest(method, path, reader)
		if contentType != "" {
			request.Header.Set("Content-Type", contentType)
		}
		return requestHandler(t, handler, request, session)
	}

	redirect := send(http.MethodGet, "/share", "", "", "")
	if redirect.Code != http.StatusPermanentRedirect || redirect.Header().Get("Location") != "/share/" {
		t.Fatalf("subpath redirect = %d, location = %q", redirect.Code, redirect.Header().Get("Location"))
	}

	defaultLogin := send(http.MethodPost, "/share/api/v1/auth/login", `{"password":"password"}`, "application/json", "")
	if defaultLogin.Code != http.StatusOK {
		t.Fatalf("default password login status = %d, body = %s", defaultLogin.Code, defaultLogin.Body.String())
	}
	defaultSession := cookieValue(t, defaultLogin.Result())

	changeBody, err := json.Marshal(map[string]string{"current_password": "password", "new_password": "x"})
	if err != nil {
		t.Fatalf("marshal password change: %v", err)
	}
	changed := send(http.MethodPost, "/share/api/v1/settings/password", string(changeBody), "application/json", defaultSession)
	if changed.Code != http.StatusOK {
		t.Fatalf("short password change status = %d, body = %s", changed.Code, changed.Body.String())
	}
	if oldPassword := send(http.MethodPost, "/share/api/v1/auth/login", `{"password":"password"}`, "application/json", ""); oldPassword.Code != http.StatusUnauthorized {
		t.Fatalf("old password status = %d, want 401", oldPassword.Code)
	}
	shortLogin := send(http.MethodPost, "/share/api/v1/auth/login", `{"password":"x"}`, "application/json", "")
	if shortLogin.Code != http.StatusOK {
		t.Fatalf("short password login status = %d, body = %s", shortLogin.Code, shortLogin.Body.String())
	}
	session := cookieValue(t, shortLogin.Result())

	published := send(http.MethodPost, "/share/api/v1/items", `{"title":"子路径文本","content":"content"}`, "application/json", session)
	if published.Code != http.StatusCreated {
		t.Fatalf("subpath publish status = %d, body = %s", published.Code, published.Body.String())
	}
	var text items.Metadata
	if err := json.Unmarshal(published.Body.Bytes(), &text); err != nil {
		t.Fatalf("unmarshal publish response: %v", err)
	}
	if text.ID == "" || text.Title != "子路径文本" || text.Size != int64(len("content")) {
		t.Fatalf("publish response metadata = %#v", text)
	}

	uploadBody := &bytes.Buffer{}
	form := multipart.NewWriter(uploadBody)
	file, err := form.CreateFormFile("files", "test.png")
	if err != nil {
		t.Fatalf("create upload form: %v", err)
	}
	if _, err := file.Write(pngFile); err != nil {
		t.Fatalf("write upload form: %v", err)
	}
	if err := form.Close(); err != nil {
		t.Fatalf("close upload form: %v", err)
	}
	uploadRequest := httptest.NewRequest(http.MethodPost, "/share/api/v1/items", uploadBody)
	uploadRequest.Header.Set("Content-Type", form.FormDataContentType())
	upload := requestHandler(t, handler, uploadRequest, session)
	if upload.Code != http.StatusMultiStatus {
		t.Fatalf("subpath upload status = %d, body = %s", upload.Code, upload.Body.String())
	}
	var uploadPayload struct {
		Results []struct {
			Status string         `json:"status"`
			Item   items.Metadata `json:"item"`
		} `json:"results"`
	}
	if err := json.Unmarshal(upload.Body.Bytes(), &uploadPayload); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	if len(uploadPayload.Results) != 1 || uploadPayload.Results[0].Status != "ok" || uploadPayload.Results[0].Item.ID == "" {
		t.Fatalf("upload response = %#v", uploadPayload.Results)
	}

	list := send(http.MethodGet, "/share/api/v1/items", "", "", session)
	if list.Code != http.StatusOK {
		t.Fatalf("subpath list status = %d, body = %s", list.Code, list.Body.String())
	}
	var listPayload struct {
		Items []items.Metadata `json:"items"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listPayload.Items) != 2 ||
		listPayload.Items[0].ID != uploadPayload.Results[0].Item.ID ||
		listPayload.Items[1].ID != text.ID {
		t.Fatalf("list response = %#v, want uploaded item and text item", listPayload.Items)
	}
}

func start(t *testing.T, configPath string) (*api.Server, *items.Store) {
	t.Helper()

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	manager := config.NewManager(configPath, cfg)
	itemStore, err := items.Open(items.Options{
		Root: cfg.Storage.Path,
		Retention: func() time.Duration {
			return manager.Config().Storage.TTL
		},
		Limits: func() config.LimitsConfig {
			return manager.Config().Limits
		},
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return api.New(itemStore, auth.NewSessionManager(), manager), itemStore
}

func request(t *testing.T, service *api.Server, req *http.Request, session string) *httptest.ResponseRecorder {
	t.Helper()

	return requestHandler(t, service.Handler(), req, session)
}

func requestHandler(t *testing.T, handler http.Handler, req *http.Request, session string) *httptest.ResponseRecorder {
	t.Helper()

	if session != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: session})
	}
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		req.Header.Set("X-SusShare-Request", "1")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func cookieValue(t *testing.T, response *http.Response) string {
	t.Helper()
	for _, cookie := range response.Cookies() {
		if cookie.Name == auth.CookieName {
			return cookie.Value
		}
	}
	t.Fatal("response did not include a session cookie")
	return ""
}

func login(t *testing.T, service *api.Server, password string) string {
	t.Helper()

	body := strings.NewReader(`{"password":"` + password + `"}`)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := request(t, service, req, "")
	if recorder.Code != 200 {
		t.Fatalf("login status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == auth.CookieName {
			return cookie.Value
		}
	}
	t.Fatal("login did not return session cookie")
	return ""
}

func publishText(t *testing.T, service *api.Server, session, title, content string) items.Metadata {
	t.Helper()

	payload, err := json.Marshal(map[string]string{"title": title, "content": content})
	if err != nil {
		t.Fatalf("marshal text: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/v1/items", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	recorder := request(t, service, req, session)
	if recorder.Code != 201 {
		t.Fatalf("publish text status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var meta items.Metadata
	if err := json.Unmarshal(recorder.Body.Bytes(), &meta); err != nil {
		t.Fatalf("unmarshal text metadata: %v", err)
	}
	if meta.Kind != items.KindText || meta.Size != int64(len(content)) {
		t.Fatalf("text metadata = %#v", meta)
	}
	return meta
}

func uploadFile(t *testing.T, service *api.Server, session string) items.Metadata {
	t.Helper()

	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="files"; filename="test.png"`)
	header.Set("Content-Type", "application/octet-stream")
	part, err := form.CreatePart(header)
	if err != nil {
		t.Fatalf("create form part: %v", err)
	}
	if _, err := part.Write(pngFile); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := form.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/v1/items", body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	recorder := request(t, service, req, session)
	if recorder.Code != 207 {
		t.Fatalf("upload status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Results []struct {
			Filename string         `json:"filename"`
			Status   string         `json:"status"`
			Item     items.Metadata `json:"item"`
		} `json:"results"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	if len(payload.Results) != 1 || payload.Results[0].Status != "ok" {
		t.Fatalf("upload response = %#v", payload.Results)
	}
	meta := payload.Results[0].Item
	if meta.Kind != items.KindFile || meta.Preview != "inline-image" {
		t.Fatalf("file metadata = %#v", meta)
	}
	return meta
}

func getList(t *testing.T, service *api.Server, session string) []items.Metadata {
	t.Helper()

	recorder := request(t, service, httptest.NewRequest("GET", "/api/v1/items", nil), session)
	if recorder.Code != 200 {
		t.Fatalf("list status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Items []items.Metadata `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	return payload.Items
}

func assertPreview(t *testing.T, service *api.Server, session, id string) {
	t.Helper()

	recorder := request(t, service, httptest.NewRequest("GET", "/api/v1/items/"+id+"/preview", nil), session)
	if recorder.Code != 200 {
		t.Fatalf("preview status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	headers := recorder.Header()
	if got := headers.Get("Content-Security-Policy"); got != "sandbox; default-src 'none'" {
		t.Fatalf("preview CSP = %q", got)
	}
	if got := headers.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("preview nosniff = %q", got)
	}
	if got := headers.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("preview referrer = %q", got)
	}
	if disposition := headers.Get("Content-Disposition"); !strings.HasPrefix(disposition, "inline") {
		t.Fatalf("preview disposition = %q", disposition)
	}
}

func assertDownload(t *testing.T, service *api.Server, session, id string, want []byte) {
	t.Helper()

	recorder := request(t, service, httptest.NewRequest("GET", "/api/v1/items/"+id+"/content", nil), session)
	if recorder.Code != 200 {
		t.Fatalf("download status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if disposition := recorder.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, "attachment") {
		t.Fatalf("download disposition = %q", disposition)
	}
	got, err := io.ReadAll(recorder.Body)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("download bytes changed: got %d bytes, want %d bytes", len(got), len(want))
	}
}
