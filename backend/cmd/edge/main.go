// Command edge is nevatal-edge, the reverse proxy behind Nevatal Domains. It
// receives the *.<PUBLIC_DOMAIN_ROOT> traffic from cloudflared and proxies
// each hostname to the private address and port of the customer VM that
// published it.
//
// It reads the same database as the API but never migrates or seeds it.
// Its port must only be reachable by cloudflared: the edge trusts the
// Cf-Connecting-Ip and Cf-Visitor headers Cloudflare sets.
//
// Build: go build -o nevatal-edge ./cmd/edge
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/database"
	"github.com/nevatal/cloud/internal/domainroute"
	"github.com/nevatal/cloud/internal/edge"
	"github.com/nevatal/cloud/internal/repository"
)

// The edge only runs one small query every couple of seconds.
const (
	dbMaxOpenConns = 5
	dbMaxIdleConns = 2
)

func main() {
	log.Println("Starting Nevatal Cloud edge proxy (nevatal-edge)...")

	cfg := config.Load()
	for _, w := range cfg.Warnings {
		log.Printf("Config warning: %s", w)
	}
	if err := cfg.ValidateEdge(); err != nil {
		log.Fatalf("Refusing to start: %v", err)
	}
	if len(cfg.EdgeTargetCIDRs) == 0 {
		log.Println("Warning: EDGE_TARGET_CIDRS is empty; every app will answer 502 until it lists the VM network(s).")
	}
	log.Printf("Serving https://<subdomain>.%s on port %s (targets: %v, denied: %v, environment: %s)",
		cfg.PublicDomainRoot, cfg.EdgePort, cfg.EdgeTargetCIDRs, cfg.EdgeDenyIPs, cfg.Env)

	db, err := database.Connect(cfg.DatabaseURL, dbMaxOpenConns, dbMaxIdleConns)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	ads := edge.AdsOptions{
		SDKURL:       cfg.AdSDKURL,
		SiteKey:      cfg.AdSiteKey,
		PlacementKey: cfg.AdPlacementKey,
	}
	if ads.SiteKey != "" {
		log.Printf("AdControl injection: enabled (placement %q, SDK %s)", cfg.AdPlacementKey, cfg.AdSDKURL)
	} else {
		log.Println("AdControl injection: disabled (ADCONTROL_SITE_KEY is empty)")
	}

	server := edge.NewServer(edge.Options{
		Root:   cfg.PublicDomainRoot,
		Policy: domainroute.Policy{TargetCIDRs: cfg.EdgeTargetCIDRs, DenyIPs: cfg.EdgeDenyIPs},
		Source: repository.NewDomainRepository(db),
		Ads:    ads,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Refresh(ctx); err != nil {
		log.Printf("Warning: initial route load failed (retrying every 2s): %v", err)
	}
	go server.Run(ctx)

	// No read/write timeouts: uploads, long polling and WebSockets may run
	// for a long time. The header timeout still stops slowloris clients.
	srv := &http.Server{
		Addr:              ":" + cfg.EdgePort,
		Handler:           server,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("nevatal-edge listening on http://0.0.0.0:%s", cfg.EdgePort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		log.Fatalf("Edge failed to start: %v", err)
	case <-ctx.Done():
		log.Println("Received shutdown signal, finishing in-flight requests...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	log.Println("Shutdown complete.")
}
