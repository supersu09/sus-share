package router

import (
	"net/http"
	"strings"
)

func New(basePath string, next http.Handler) http.Handler {
	if basePath == "" || basePath == "/" {
		return next
	}
	prefix := strings.TrimSuffix(basePath, "/")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == prefix {
			target := prefix + "/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusPermanentRedirect)
			return
		}
		if strings.HasPrefix(r.URL.Path, prefix+"/") {
			scoped := r.Clone(r.Context())
			scoped.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
			if scoped.URL.Path == "" {
				scoped.URL.Path = "/"
			}
			scoped.URL.RawPath = ""
			next.ServeHTTP(w, scoped)
			return
		}
		next.ServeHTTP(w, r)
	})
}
