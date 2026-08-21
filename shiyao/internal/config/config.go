package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string `env:"DATABASE_URL,required"`
	RedisURL       string `env:"REDIS_URL,required"`
	NATSURL        string `env:"NATS_URL,required"`
	AllowedOrigins string `env:"ALLOWED_ORIGINS"`
	Development    bool   `env:"DEVELOPMENT" envDefault:"false"`
}

func (c Config) CORSOrigins() []string {
	if strings.TrimSpace(c.AllowedOrigins) == "" {
		return nil
	}

	parts := strings.Split(c.AllowedOrigins, ",")
	origins := make([]string, 0, len(parts))

	for _, part := range parts {
		if origin := strings.TrimSpace(part); origin != "" {
			origins = append(origins, origin)
		}
	}

	return origins
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("parse environment: %w", err)
	}

	return cfg, nil
}
