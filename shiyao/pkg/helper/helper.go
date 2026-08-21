package helper

import (
	"net/http"

	"github.com/coffeyvidzro/shiyao/pkg/pgconv"
	"github.com/jackc/pgx/v5/pgtype"
)

func FormatTime(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}

	t := pgconv.TimestamptzToTime(value)
	if t.IsZero() {
		return ""
	}

	return t.Format(http.TimeFormat)
}
