package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	adapter "github.com/coffeyvidzro/shiyao/internal/adapters/nats"
	"github.com/coffeyvidzro/shiyao/internal/adapters/postgres"
	"github.com/coffeyvidzro/shiyao/internal/config"
	"github.com/coffeyvidzro/shiyao/internal/runtime/worker"
)

func main() {
	log.Println("shiyao worker starting...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer db.Close()

	natsClient, err := adapter.New(ctx, cfg.NATSURL)
	if err != nil {
		log.Fatalf("connect to nats: %v", err)
	}
	defer func() { _ = natsClient.Close() }()

	registry, err := worker.New(ctx, cfg, db, natsClient)
	if err != nil {
		log.Fatalf("initialize worker: %v", err)
	}
	defer registry.Close()

	log.Println("shiyao worker running")
	<-ctx.Done()
	log.Println("shiyao worker stopping")
}
