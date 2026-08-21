package daemon

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/coffeyvidzro/shiyao/internal/adapters/redis"
	"github.com/coffeyvidzro/shiyao/internal/config"
	"github.com/coffeyvidzro/shiyao/internal/database/sqlc"
	"github.com/coffeyvidzro/shiyao/internal/identity/auth"
	"github.com/coffeyvidzro/shiyao/internal/identity/pat"
	"github.com/coffeyvidzro/shiyao/internal/identity/session"
	"github.com/coffeyvidzro/shiyao/internal/identity/users"
	"github.com/coffeyvidzro/shiyao/internal/network"
	"github.com/coffeyvidzro/shiyao/internal/platform/sandbox"
	"github.com/coffeyvidzro/shiyao/internal/runtime/middleware"
	"github.com/coffeyvidzro/shiyao/internal/vmm"
	"github.com/coffeyvidzro/shiyao/internal/vsock"
)

type Registry struct {
	Config      config.Config
	DB          *pgxpool.Pool
	RedisClient *redis.Client
	Router      *gin.Engine
	Modules     Modules
}

func New(
	ctx context.Context,
	cfg config.Config,
	db *pgxpool.Pool,
	redisClient *redis.Client,
) (*Registry, error) {
	modules, err := NewModules(ctx, cfg, db, redisClient)
	if err != nil {
		return nil, fmt.Errorf("initialize modules: %w", err)
	}

	router := gin.New()
	router.Use(
		gin.Logger(),
		gin.Recovery(),
		middleware.Secure(),
		middleware.CORS(cfg.CORSOrigins, cfg.Development),
	)

	RegisterRoutes(router, modules)

	return &Registry{
		Config:      cfg,
		DB:          db,
		RedisClient: redisClient,
		Router:      router,
		Modules:     modules,
	}, nil
}

func NewModules(
	ctx context.Context,
	cfg config.Config,
	db *pgxpool.Pool,
	redisClient *redis.Client,
) (Modules, error) {
	queries := sqlc.New(db)

	userRepository := users.NewRepository(queries)
	userService := users.NewService(userRepository)

	sessionRepository := session.NewRepository(queries)
	sessionService := session.NewService(sessionRepository)

	authRepository := auth.NewRepository(queries)
	authService := auth.NewService(authRepository, sessionService)

	patRepository := pat.NewRepository(queries)
	patService := pat.NewService(patRepository)

	vmmConfig := vmm.Config{
		KernelPath:     cfg.VMMKernelPath,
		RootfsPath:     cfg.VMMRootfsPath,
		VCPUCount:      cfg.VMMVCPUCount,
		MemSizeMB:      cfg.VMMMemoryMB,
		BootArgs:       cfg.VMMBootArgs,
		GuestAgentPath: cfg.VMMGuestAgentPath,
	}
	if err := vmmConfig.Validate(); err != nil {
		return Modules{}, fmt.Errorf("validate vmm config: %w", err)
	}

	networkConfig := network.DefaultConfig("")
	networkConfig.UplinkInterface = cfg.VMMUplinkInterface
	if err := networkConfig.Validate(); err != nil {
		return Modules{}, fmt.Errorf("validate network config: %w", err)
	}

	vmmManager := vmm.NewManager(
		vmmConfig,
		networkConfig,
		vsock.Config{},
		vmm.SnapshotConfig{},
	)

	sandboxRepository := sandbox.NewRepository(queries)
	sandboxService := sandbox.NewService(sandboxRepository, newVMManager(vmmManager))

	return Modules{
		Auth: AuthModule{
			Repository: authRepository,
			Service:    authService,
			Handler:    auth.NewHandler(authService, sessionService),
		},
		PAT: PATModule{
			Repository: patRepository,
			Service:    patService,
			Handler:    pat.NewHandler(patService),
		},
		Session: SessionModule{
			Repository: sessionRepository,
			Service:    sessionService,
			Handler:    session.NewHandler(sessionService),
		},
		Users: UserModule{
			Repository: userRepository,
			Service:    userService,
			Handler:    users.NewHandler(userService),
		},
		Sandbox: SandboxModule{
			Repository: sandboxRepository,
			Service:    sandboxService,
			Handler:    sandbox.NewHandler(sandboxService),
		},
	}, nil
}

func (r *Registry) Close() {
	if r.RedisClient != nil {
		r.RedisClient.Close()
	}

	if r.DB != nil {
		r.DB.Close()
	}
}
