package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/trixie/sus-share/internal/api"
	"github.com/trixie/sus-share/internal/auth"
	"github.com/trixie/sus-share/internal/config"
	"github.com/trixie/sus-share/internal/items"
	"github.com/trixie/sus-share/internal/router"
	"github.com/trixie/sus-share/internal/web"
)

func main() {
	configPath := flag.String("config", "/config/config.yaml", "path to YAML configuration")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sus-share: invalid configuration: %v\n", err)
		os.Exit(1)
	}
	manager := config.NewManager(*configPath, cfg)
	itemStore, err := items.Open(items.Options{
		Root:      cfg.Storage.Path,
		Retention: func() time.Duration { return manager.Config().Storage.TTL },
		Limits:    func() config.LimitsConfig { return manager.Config().Limits },
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "sus-share: cannot open storage: %v\n", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", api.New(itemStore, auth.NewSessionManager(), manager).Handler())
	mux.Handle("/", web.Handler())
	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:           router.New(cfg.Server.BasePath, mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("SusShare listening on %s", server.Addr)
	if cfg.Server.TLS.Enabled {
		log.Fatal(server.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile))
	} else {
		log.Fatal(server.ListenAndServe())
	}
}
