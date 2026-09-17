package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/trixie/sus-share/internal/auth"
	"github.com/trixie/sus-share/internal/config"
	"github.com/trixie/sus-share/internal/items"
)

const maxHeadBytes = 1 << 20

type Server struct {
	items    *items.Store
	sessions *auth.SessionManager
	config   *config.Manager
	maxHead  int64
}

func New(itemStore *items.Store, sessions *auth.SessionManager, configManager *config.Manager) *Server {
	return &Server{items: itemStore, sessions: sessions, config: configManager, maxHead: maxHeadBytes}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", s.handleLogin)
	mux.Handle("/api/v1/auth/logout", s.protected(http.MethodPost, s.handleLogout))
	mux.Handle("/api/v1/items", s.protected("", s.handleItems))
	mux.Handle("/api/v1/items/", s.protected("", s.handleItem))
	mux.Handle("/api/v1/settings", s.protected("", s.handleSettings))
	mux.Handle("/api/v1/settings/password", s.protected(http.MethodPost, s.handlePassword))
	return mux
}

func cookiePath(cfg config.Config) string {
	if cfg.Server.BasePath == "" {
		return "/"
	}
	return cfg.Server.BasePath
}

func (s *Server) protected(method string, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.CookieName)
		if err != nil || !s.sessions.Validate(cookie.Value) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		if method != "" && r.Method != method {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.Header.Get("X-SusShare-Request") != "1" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request origin"})
				return
			}
		}
		next(w, r)
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var request struct {
		Password string `json:"password"`
	}
	if !readJSON(w, r, &request) {
		return
	}
	cfg := s.config.Config()
	if cfg.Auth.Enabled {
		ok, err := auth.VerifyPassword(request.Password, auth.EffectivePasswordHash(cfg.Auth.PasswordHash))
		if err != nil || !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication failed"})
			return
		}
	}
	token, err := s.sessions.Create()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create session"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: auth.CookieName, Value: token, Path: cookiePath(cfg), HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: cfg.Server.TLS.Enabled,
		Expires: time.Now().Add(7 * 24 * time.Hour),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cfg := s.config.Config()
	if cookie, err := r.Cookie(auth.CookieName); err == nil {
		s.sessions.Delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: auth.CookieName, Value: "", Path: cookiePath(cfg), MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: cfg.Server.TLS.Enabled,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		list := s.items.List()
		writeJSON(w, http.StatusOK, map[string]any{"items": list})
	case http.MethodPost:
		s.publish(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) publish(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(contentType, "application/json"):
		s.publishText(w, r)
	case strings.HasPrefix(contentType, "multipart/form-data"):
		s.publishFiles(w, r)
	default:
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "unsupported content type"})
	}
}

func (s *Server) publishText(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(s.config.Config().Limits.MaxTextSize)+1024)
	var request struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if !readJSON(w, r, &request) {
		return
	}
	meta, err := s.items.CreateText(request.Title, request.Content)
	writeItemResult(w, meta, err)
}

func (s *Server) publishFiles(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
	reader, err := r.MultipartReader()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart request"})
		return
	}
	results := make([]map[string]any, 0)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart body"})
			return
		}
		if part.FormName() != "files" {
			part.Close()
			continue
		}
		meta, itemErr := s.createFileFromPart(part)
		part.Close()
		result := map[string]any{"filename": part.FileName()}
		if itemErr != nil {
			result["error"] = errorText(itemErr)
			result["status"] = statusForError(itemErr)
		} else {
			result["item"] = meta
			result["status"] = "ok"
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no files supplied"})
		return
	}
	writeJSON(w, http.StatusMultiStatus, map[string]any{"results": results})
}

func (s *Server) createFileFromPart(part *multipart.Part) (items.Metadata, error) {
	head := make([]byte, 512)
	headSize, err := io.ReadFull(part, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return items.Metadata{}, err
	}
	head = head[:headSize]
	meta, err := s.items.CreateFileStream(part.FileName(), part.FileName(), part.Header.Get("Content-Type"), io.MultiReader(strings.NewReader(string(head)), part))
	return meta, err
}

func (s *Server) handleItem(w http.ResponseWriter, r *http.Request) {
	remaining := strings.TrimPrefix(r.URL.Path, "/api/v1/items/")
	parts := strings.Split(remaining, "/")
	id := parts[0]
	if id == "" || len(parts) > 2 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		meta, err := s.items.Get(id)
		if err != nil {
			writeItemError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, meta)
	case len(parts) == 1 && r.Method == http.MethodDelete:
		if err := s.items.Delete(id); err != nil {
			writeItemError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	case len(parts) == 2 && parts[1] == "content" && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		meta, file, err := s.items.Content(id)
		if err != nil {
			writeItemError(w, err)
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", meta.ContentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+urlPathEscape(meta.Filename))
		w.Header().Set("Referrer-Policy", "no-referrer")
		http.ServeContent(w, r, meta.Filename, meta.CreatedAt, file)
	case len(parts) == 2 && parts[1] == "preview" && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		meta, file, err := s.items.Content(id)
		if err != nil {
			writeItemError(w, err)
			return
		}
		defer file.Close()
		disposition := "attachment"
		if meta.Preview == "inline-image" || meta.Preview == "inline-audio" || meta.Preview == "inline-video" || meta.Preview == "inline-pdf" {
			disposition = "inline"
		}
		w.Header().Set("Content-Type", meta.ContentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", disposition)
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		http.ServeContent(w, r, meta.Filename, meta.CreatedAt, file)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := s.config.Config()
		writeJSON(w, http.StatusOK, map[string]any{"retention": canonicalRetention(cfg.Storage.TTL), "retention_choices": []string{"1h", "6h", "12h", "24h", "72h", "168h"}})
	case http.MethodPut:
		var request struct {
			Retention string `json:"retention"`
		}
		if !readJSON(w, r, &request) {
			return
		}
		ttl, err := time.ParseDuration(request.Retention)
		if err != nil || ttl < config.MinRetention || ttl > config.MaxRetention {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid retention"})
			return
		}
		if err := s.config.UpdateRetention(ttl); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not persist settings"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"retention": canonicalRetention(s.config.Config().Storage.TTL)})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handlePassword(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !readJSON(w, r, &request) {
		return
	}
	cfg := s.config.Config()
	ok, err := auth.VerifyPassword(request.CurrentPassword, auth.EffectivePasswordHash(cfg.Auth.PasswordHash))
	if err != nil || !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication failed"})
		return
	}
	hash, err := auth.GenerateHash(request.NewPassword, auth.DefaultArgon2Params())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": errorText(err)})
		return
	}
	if err := s.config.UpdatePasswordHash(hash); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not persist password"})
		return
	}
	s.sessions.InvalidateAll()
	writeJSON(w, http.StatusOK, map[string]string{"status": "password_changed"})
}

func readJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "expected JSON"})
		return false
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(status)
	if status != http.StatusNoContent {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func writeItemResult(w http.ResponseWriter, meta items.Metadata, err error) {
	if err != nil {
		writeJSON(w, statusForError(err), map[string]string{"error": errorText(err)})
		return
	}
	writeJSON(w, http.StatusCreated, meta)
}

func writeItemError(w http.ResponseWriter, err error) {
	if errors.Is(err, items.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if errors.Is(err, items.ErrLimitExceeded) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "limit exceeded"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, items.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, items.ErrLimitExceeded):
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, items.ErrLimitExceeded) {
		return "limit exceeded"
	}
	return err.Error()
}

func urlPathEscape(value string) string {
	return strings.ReplaceAll(value, `"`, "%22")
}

func canonicalRetention(ttl time.Duration) string {
	if hours := int64(ttl.Hours()); ttl == time.Duration(hours)*time.Hour && hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return ttl.String()
}
