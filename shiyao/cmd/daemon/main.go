package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	adapter "github.com/coffeyvidzro/shiyao/internal/adapters/nats"
	"github.com/coffeyvidzro/shiyao/internal/adapters/postgres"
	"github.com/coffeyvidzro/shiyao/internal/adapters/redis"
	"github.com/coffeyvidzro/shiyao/internal/config"
	daemon "github.com/coffeyvidzro/shiyao/internal/runtime/daemon"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("shiyao-daemon fatal: %v", err)
	}
}

func run() error {
	// 1. Create a root context that cancels on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("shiyao-daemon starting...")

	// 2. Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 3. Connect to PostgreSQL.
	db, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer db.Close()

	// 4. Connect to Redis.
	redisClient, err := redis.New(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}
	defer redisClient.Close()

	// 5. Connect to NATS JetStream.
	natsClient, err := adapter.New(ctx, cfg.NATSURL, "shiyao-daemon")
	if err != nil {
		return fmt.Errorf("connect to nats: %w", err)
	}
	defer func() { _ = natsClient.Close() }()

	// 6. Build the daemon registry (wires modules, routes, middleware).
	registry, err := daemon.New(ctx, cfg, db, redisClient, natsClient)
	if err != nil {
		return fmt.Errorf("initialize daemon: %w", err)
	}
	defer registry.Close()

	// 7. Configure the HTTP server with safe timeouts.
	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           registry.Router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second, // Long enough for most REST calls
		IdleTimeout:       120 * time.Second,
	}

	// 8. Start the server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		log.Printf("shiyao-daemon listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	// 9. Block until shutdown signal or server error.
	select {
	case <-ctx.Done():
		log.Println("shutdown signal received, draining connections...")
	case err := <-errCh:
		return fmt.Errorf("http server error: %w", err)
	}

	// 10. Graceful shutdown: stop accepting new requests, wait for in-flight.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	log.Println("shiyao-daemon stopped cleanly")
	return nil
}
