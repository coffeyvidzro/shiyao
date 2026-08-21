package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL        string   `env:"DATABASE_URL,required"`
	RedisURL           string   `env:"REDIS_URL,required"`
	NATSURL            string   `env:"NATS_URL,required"`
	CORSOrigins        []string `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000,http://127.0.0.1:3000"`
	Development        bool     `env:"DEVELOPMENT" envDefault:"false"`
	VMMKernelPath      string   `env:"VMM_KERNEL_PATH,required"`
	VMMRootfsPath      string   `env:"VMM_ROOTFS_PATH,required"`
	VMMGuestAgentPath  string   `env:"VMM_GUEST_AGENT_PATH" envDefault:"/usr/local/bin/shiyao-agent"`
	VMMVCPUCount       int      `env:"VMM_VCPU_COUNT" envDefault:"2"`
	VMMMemoryMB        int      `env:"VMM_MEMORY_MB" envDefault:"512"`
	VMMBootArgs        string   `env:"VMM_BOOT_ARGS" envDefault:"console=ttyS0 reboot=k panic=1 pci=off"`
	VMMUplinkInterface string   `env:"VMM_UPLINK_INTERFACE" envDefault:"eth0"`
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
