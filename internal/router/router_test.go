package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRootPathIsUnchanged(t *testing.T) {
	var served string
	handler := New("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = r.URL.Path
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || served != "/api/v1/items" {
		t.Fatalf("root response = %d, path = %q", response.Code, served)
	}
}

func TestSubpathRedirectAndPrefixStripping(t *testing.T) {
	var served string
	handler := New("/share", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = r.URL.Path
	}))

	request := httptest.NewRequest(http.MethodGet, "/share?refresh=1", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusPermanentRedirect || response.Header().Get("Location") != "/share/?refresh=1" {
		t.Fatalf("redirect = %d, location = %q", response.Code, response.Header().Get("Location"))
	}

	for _, path := range []string{"/share/", "/share/api/v1/items", "/api/v1/items"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		want := strings.TrimPrefix(path, "/share")
		if want == "" {
			want = "/"
		}
		if response.Code != http.StatusOK || served != want {
			t.Fatalf("path %s: response = %d, served = %q, want %q", path, response.Code, served, want)
		}
	}
}
