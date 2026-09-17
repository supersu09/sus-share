package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/trixie/sus-share/internal/auth"
	"github.com/trixie/sus-share/internal/config"
	"github.com/trixie/sus-share/internal/items"
)

func newTestServer(t *testing.T, password string) (http.Handler, *auth.SessionManager) {
	t.Helper()
	hash, err := auth.GenerateHash(password, auth.Argon2Params{Memory: 8, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Auth.PasswordHash = hash
	cfg.Limits.MaxTextSize = 1024
	cfg.Limits.MaxFileSize = 1024 * 1024
	cfg.Limits.MaxStorageSize = 2 * 1024 * 1024
	manager := config.NewManager(t.TempDir()+"/config.yaml", cfg)
	store, err := items.Open(items.Options{
		Root: t.TempDir(),
		Retention: func() time.Duration {
			return time.Hour
		},
		Limits: func() config.LimitsConfig { return cfg.Limits },
	})
	if err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessionManager()
	return New(store, sessions, manager).Handler(), sessions
}

func login(t *testing.T, handler http.Handler, password string) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"password": password})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Result()
}

func clientCookie(t *testing.T, response *http.Response) string {
	t.Helper()
	for _, cookie := range response.Cookies() {
		if cookie.Name == auth.CookieName {
			return cookie.Name + "=" + cookie.Value
		}
	}
	t.Fatal("login did not issue session cookie")
	return ""
}

func TestLoginLogoutAndProtection(t *testing.T) {
	handler, sessions := newTestServer(t, "password123")
	if response := login(t, handler, "wrong-password"); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d", response.StatusCode)
	}
	first := login(t, handler, "password123")
	if first.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", first.StatusCode)
	}
	firstCookie := clientCookie(t, first)
	second := login(t, handler, "password123")
	secondCookie := clientCookie(t, second)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	request.Header.Set("Cookie", firstCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("protected status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.Header.Set("Cookie", firstCookie)
	request.Header.Set("X-SusShare-Request", "1")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("logout status = %d", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	request.Header.Set("Cookie", firstCookie)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("logged-out status = %d", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	request.Header.Set("Cookie", secondCookie)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("other-session status = %d", response.Code)
	}
	if sessions.Len() != 1 {
		t.Fatalf("sessions = %d, want 1", sessions.Len())
	}
}

func TestCookiePathFollowsBasePath(t *testing.T) {
	cfg := config.Default()
	cfg.Auth.Enabled = false
	cfg.Server.BasePath = "/share"
	cfg.Limits.MaxTextSize = 1024
	cfg.Limits.MaxFileSize = 1024 * 1024
	cfg.Limits.MaxStorageSize = 2 * 1024 * 1024
	manager := config.NewManager(t.TempDir()+"/config.yaml", cfg)
	store, err := items.Open(items.Options{
		Root: t.TempDir(),
		Retention: func() time.Duration {
			return time.Hour
		},
		Limits: func() config.LimitsConfig { return cfg.Limits },
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(store, auth.NewSessionManager(), manager).Handler()

	loginResponse := login(t, handler, "")
	var loginCookie *http.Cookie
	for _, cookie := range loginResponse.Cookies() {
		if cookie.Name == auth.CookieName {
			loginCookie = cookie
			break
		}
	}
	if loginCookie == nil {
		t.Fatal("login did not issue a session cookie")
	}
	if loginCookie.Path != "/share" || !loginCookie.HttpOnly || loginCookie.SameSite != http.SameSiteStrictMode || loginCookie.Secure {
		t.Fatalf("login cookie = %+v", loginCookie)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.Header.Set("Cookie", clientCookie(t, loginResponse))
	request.Header.Set("X-SusShare-Request", "1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var logoutCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == auth.CookieName {
			logoutCookie = cookie
			break
		}
	}
	if logoutCookie == nil || logoutCookie.Path != "/share" || !logoutCookie.HttpOnly ||
		logoutCookie.SameSite != http.SameSiteStrictMode || logoutCookie.Secure || logoutCookie.MaxAge != -1 {
		t.Fatalf("logout cookie = %+v", logoutCookie)
	}
}

func TestPasswordPolicyAllowsShortAndEmptyPasswords(t *testing.T) {
	oldPassword := "old-password"
	hash, err := auth.GenerateHash(oldPassword, auth.Argon2Params{Memory: 8, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Auth.PasswordHash = hash
	cfg.Limits.MaxTextSize = 1024
	cfg.Limits.MaxFileSize = 1024 * 1024
	cfg.Limits.MaxStorageSize = 2 * 1024 * 1024
	configPath := t.TempDir() + "/config.yaml"
	manager := config.NewManager(configPath, cfg)
	store, err := items.Open(items.Options{
		Root: t.TempDir(),
		Retention: func() time.Duration {
			return time.Hour
		},
		Limits: func() config.LimitsConfig { return cfg.Limits },
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(store, auth.NewSessionManager(), manager).Handler()

	changePassword := func(currentPassword, newPassword string) int {
		t.Helper()
		loginResponse := login(t, handler, currentPassword)
		body, _ := json.Marshal(map[string]string{
			"current_password": currentPassword,
			"new_password":     newPassword,
		})
		request := httptest.NewRequest(http.MethodPost, "/api/v1/settings/password", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Cookie", clientCookie(t, loginResponse))
		request.Header.Set("X-SusShare-Request", "1")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response.Code
	}

	if status := changePassword(oldPassword, "x"); status != http.StatusOK {
		t.Fatalf("short password change status = %d", status)
	}
	if response := login(t, handler, oldPassword); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old password status = %d", response.StatusCode)
	}
	if response := login(t, handler, "x"); response.StatusCode != http.StatusOK {
		t.Fatalf("short password login status = %d", response.StatusCode)
	}
	if status := changePassword("x", ""); status != http.StatusOK {
		t.Fatalf("empty password change status = %d", status)
	}
	if response := login(t, handler, "x"); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("previous password status = %d", response.StatusCode)
	}
	if response := login(t, handler, ""); response.StatusCode != http.StatusOK {
		t.Fatalf("empty password login status = %d", response.StatusCode)
	}
}

func TestPublishTextAndMultiPartFiles(t *testing.T) {
	handler, _ := newTestServer(t, "password123")
	response := login(t, handler, "password123")
	cookie := clientCookie(t, response)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/items", strings.NewReader(`{"title":"SQL","content":"SELECT 1"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cookie", cookie)
	request.Header.Set("X-SusShare-Request", "1")
	published := httptest.NewRecorder()
	handler.ServeHTTP(published, request)
	if published.Code != http.StatusCreated {
		t.Fatalf("publish status = %d, body=%s", published.Code, published.Body.String())
	}

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	for i := 0; i < 2; i++ {
		file, _ := writer.CreateFormFile("files", fmt.Sprintf("file-%d.png", i))
		_, _ = file.Write(append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, bytes.Repeat([]byte{0}, 600)...))
	}
	writer.Close()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/items", &requestBody)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Cookie", cookie)
	request.Header.Set("X-SusShare-Request", "1")
	upload := httptest.NewRecorder()
	handler.ServeHTTP(upload, request)
	if upload.Code != http.StatusMultiStatus {
		t.Fatalf("upload status = %d, body=%s", upload.Code, upload.Body.String())
	}
	var result struct {
		Results []struct {
			Item   items.Metadata
			Status string
		}
	}
	if err := json.NewDecoder(upload.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 2 || result.Results[0].Item.Size != 608 {
		t.Fatalf("upload result = %+v", result)
	}
	for _, fileResult := range result.Results {
		previewRequest := httptest.NewRequest(http.MethodGet, "/api/v1/items/"+fileResult.Item.ID+"/preview", nil)
		previewRequest.Header.Set("Cookie", cookie)
		previewResponse := httptest.NewRecorder()
		handler.ServeHTTP(previewResponse, previewRequest)
		if previewResponse.Header().Get("Content-Security-Policy") != "sandbox; default-src 'none'" || previewResponse.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("preview response lacks required security headers")
		}
	}
}

func TestCrossSiteStyleMutationRejected(t *testing.T) {
	handler, _ := newTestServer(t, "password123")
	response := login(t, handler, "password123")
	cookie := clientCookie(t, response)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/items", strings.NewReader(`{"title":"bad","content":"bad"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cookie", cookie)
	mutation := httptest.NewRecorder()
	handler.ServeHTTP(mutation, request)
	if mutation.Code != http.StatusBadRequest {
		t.Fatalf("missing origin header status = %d", mutation.Code)
	}
	itemsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	itemsRequest.Header.Set("Cookie", cookie)
	itemsResponse := httptest.NewRecorder()
	handler.ServeHTTP(itemsResponse, itemsRequest)
	var payload struct {
		Items []items.Metadata `json:"items"`
	}
	_ = json.NewDecoder(itemsResponse.Body).Decode(&payload)
	if len(payload.Items) != 0 {
		t.Fatalf("cross-site mutation created %d items", len(payload.Items))
	}
}

func TestSettingsAndPasswordRestartInvalidatesSessions(t *testing.T) {
	password := "old-password"
	hash, err := auth.GenerateHash(password, auth.Argon2Params{Memory: 8, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Auth.PasswordHash = hash
	cfg.Limits.MaxTextSize = 1024
	cfg.Limits.MaxFileSize = 1024 * 1024
	cfg.Limits.MaxStorageSize = 2 * 1024 * 1024
	configPath := t.TempDir() + "/config.yaml"
	manager := config.NewManager(configPath, cfg)
	store, err := items.Open(items.Options{
		Root: t.TempDir(),
		Retention: func() time.Duration {
			return manager.Config().Storage.TTL
		},
		Limits: func() config.LimitsConfig { return manager.Config().Limits },
	})
	if err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessionManager()
	handler := New(store, sessions, manager).Handler()
	response := login(t, handler, password)
	cookie := clientCookie(t, response)

	settingsBody := strings.NewReader(`{"retention":"72h"}`)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/settings", settingsBody)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cookie", cookie)
	request.Header.Set("X-SusShare-Request", "1")
	settingsResponse := httptest.NewRecorder()
	handler.ServeHTTP(settingsResponse, request)
	if settingsResponse.Code != http.StatusOK {
		t.Fatalf("settings status = %d, body=%s", settingsResponse.Code, settingsResponse.Body.String())
	}

	passwordBody := strings.NewReader(`{"current_password":"old-password","new_password":"new-password"}`)
	request = httptest.NewRequest(http.MethodPost, "/api/v1/settings/password", passwordBody)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cookie", cookie)
	request.Header.Set("X-SusShare-Request", "1")
	passwordResponse := httptest.NewRecorder()
	handler.ServeHTTP(passwordResponse, request)
	if passwordResponse.Code != http.StatusOK {
		t.Fatalf("password status = %d, body=%s", passwordResponse.Code, passwordResponse.Body.String())
	}
	if sessions.Len() != 0 {
		t.Fatalf("sessions after password change = %d, want 0", sessions.Len())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	request.Header.Set("Cookie", cookie)
	authResponse := httptest.NewRecorder()
	handler.ServeHTTP(authResponse, request)
	if authResponse.Code != http.StatusUnauthorized {
		t.Fatalf("old session status = %d", authResponse.Code)
	}

	reloadedConfig, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if reloadedConfig.Storage.TTL != 72*time.Hour {
		t.Fatalf("persisted retention = %s", reloadedConfig.Storage.TTL)
	}
	reloadedManager := config.NewManager(configPath, reloadedConfig)
	reloadedStore, err := items.Open(items.Options{
		Root: store.Root(),
		Retention: func() time.Duration {
			return reloadedManager.Config().Storage.TTL
		},
		Limits: func() config.LimitsConfig { return reloadedManager.Config().Limits },
	})
	if err != nil {
		t.Fatal(err)
	}
	reloadedHandler := New(reloadedStore, auth.NewSessionManager(), reloadedManager).Handler()
	if response := login(t, reloadedHandler, password); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old password login = %d", response.StatusCode)
	}
	if response := login(t, reloadedHandler, "new-password"); response.StatusCode != http.StatusOK {
		t.Fatalf("new password login = %d", response.StatusCode)
	}
}
