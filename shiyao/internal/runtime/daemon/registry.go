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
	"github.com/coffeyvidzro/shiyao/internal/identity/session"
	"github.com/coffeyvidzro/shiyao/internal/identity/users"
	"github.com/coffeyvidzro/shiyao/internal/platform/sandbox"
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
	router.Use(gin.Logger(), gin.Recovery())

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

	sandboxRepository := sandbox.NewRepository(queries)
	sandboxService := sandbox.NewService(sandboxRepository)

	return Modules{
		Auth: AuthModule{
			Repository: authRepository,
			Service:    authService,
			Handler:    auth.NewHandler(authService),
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
