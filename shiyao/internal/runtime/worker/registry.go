package worker

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	adapter "github.com/coffeyvidzro/shiyao/internal/adapters/nats"
	"github.com/coffeyvidzro/shiyao/internal/config"
	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/coffeyvidzro/shiyao/internal/network"
	"github.com/coffeyvidzro/shiyao/internal/platform/sandbox"
	"github.com/coffeyvidzro/shiyao/internal/vmm"
	"github.com/coffeyvidzro/shiyao/internal/vsock"
)

type Registry struct {
	Config     config.Config
	DB         *pgxpool.Pool
	NATS       *adapter.Client
	Queries    *sqlc.Queries
	Repository *sandbox.Repository
	VMM        *vmm.Manager
	Consumers  []adapter.ConsumerHandle
}

func New(ctx context.Context, cfg config.Config, db *pgxpool.Pool, natsClient *adapter.Client) (*Registry, error) {
	queries := sqlc.New(db)
	repository := sandbox.NewRepository(queries)

	vmmConfig := vmm.Config{
		KernelPath:     cfg.VMMKernelPath,
		RootfsPath:     cfg.VMMRootfsPath,
		VCPUCount:      cfg.VMMVCPUCount,
		MemSizeMB:      cfg.VMMMemoryMB,
		BootArgs:       cfg.VMMBootArgs,
		GuestAgentPath: cfg.VMMGuestAgentPath,
	}
	if err := vmmConfig.Validate(); err != nil {
		return nil, fmt.Errorf("validate vmm config: %w", err)
	}

	networkConfig := network.DefaultConfig("")
	networkConfig.UplinkInterface = cfg.VMMUplinkInterface

	registry := &Registry{
		Config:     cfg,
		DB:         db,
		NATS:       natsClient,
		Queries:    queries,
		Repository: repository,
		VMM: vmm.NewManager(
			vmmConfig,
			networkConfig,
			vsock.Config{},
			vmm.SnapshotConfig{},
		),
	}

	if err := registry.subscribe(ctx); err != nil {
		registry.Close()
		return nil, err
	}
	return registry, nil
}

func (r *Registry) Close() {
	for _, consumer := range r.Consumers {
		consumer.Stop()
	}
	if r.NATS != nil {
		_ = r.NATS.Close()
	}
	if r.DB != nil {
		r.DB.Close()
	}
}
