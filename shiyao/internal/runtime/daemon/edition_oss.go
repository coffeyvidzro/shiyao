//go:build !cloud

package daemon

import "github.com/coffeyvidzro/shiyao/internal/database/sqlc"

func configureEditionModules(_ *sqlc.Queries, _ *Modules) {}
