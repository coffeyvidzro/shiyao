package main

import (
	"context"
	"log"

	adapter "github.com/coffeyvidzro/shiyao/internal/adapters/nats"
	"github.com/coffeyvidzro/shiyao/internal/adapters/postgres"
	"github.com/coffeyvidzro/shiyao/internal/adapters/redis"
	"github.com/coffeyvidzro/shiyao/internal/config"
	daemon "github.com/coffeyvidzro/shiyao/internal/runtime/daemon"
)

func main() {
	log.Println("shiyao daemon starting...")

	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer db.Close()

	redisClient, err := redis.New(ctx, cfg.RedisURL)
	if err != nil {
		db.Close()
		log.Fatalf("connect to redis: %v", err)
	}
	defer redisClient.Close()

	natsClient, err := adapter.New(ctx, cfg.NATSURL)
	if err != nil {
		log.Fatalf("connect to nats: %v", err)
	}
	defer func() { _ = natsClient.Close() }()

	registry, err := daemon.New(ctx, cfg, db, redisClient, natsClient)
	if err != nil {
		log.Fatalf("initialize daemon: %v", err)
	}
	defer registry.Close()

	if err := registry.Router.Run(":8080"); err != nil {
		log.Fatalf("run daemon: %v", err)
	}
}
