package daemon

// import (
// 	"context"
// 	"fmt"

// // 	"github.com/coffeyvidzro/shiyao/internal/identity/auth"
// // 	"github.com/coffeyvidzro/shiyao/internal/identity/session"
// // 	"github.com/coffeyvidzro/shiyao/internal/identity/users"
// )

// type Registry struct {
// 	Config  config.Config
// 	DB      *pgxpool.Pool
// 	Echo    *echo.Echo
// 	Modules Modules
// }

// func New(ctx context.Context, cfg config.Config) (*Registry, error) {
// 	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
// 	if err != nil {
// 		return nil, fmt.Errorf("connect database: %w", err)
// 	}

// 	modules, err := NewModules(ctx, cfg, db)
// 	if err != nil {
// 		db.Close()
// 		return nil, fmt.Errorf("initialize modules: %w", err)
// 	}

// 	e := echo.New()
// 	RegisterRoutes(e, modules)

// 	return &Registry{Config: cfg, DB: db, Echo: e, Modules: modules}, nil
// }

// func NewModules(ctx context.Context, cfg config.Config, db *pgxpool.Pool) (Modules, error) {
// 	userRepository := users.NewRepository(db)
// 	userService := users.NewService(userRepository)

// 	sessionRepository := session.NewRepository(db)
// 	sessionService := session.NewService(sessionRepository)

// 	authRepository := auth.NewRepository(db)
// 	authService := auth.NewService(authRepository, sessionService)

// 	return Modules{
// 		Auth: AuthModule{
// 			Repository: authRepository,
// 			Service:    authService,
// 			Handler:    auth.NewHandler(authService),
// 		},
// 		Session: SessionModule{
// 			Repository: sessionRepository,
// 			Service:    sessionService,
// 			Handler:    session.NewHandler(sessionService),
// 		},
// 		Users: UserModule{
// 			Repository: userRepository,
// 			Service:    userService,
// 			Handler:    users.NewHandler(userService),
// 		},
// 	}, nil
// }

// func (r *Registry) Close() {
// 	if r.DB != nil {
// 		r.DB.Close()
// 	}
// }
