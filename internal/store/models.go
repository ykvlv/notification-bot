package store

import (
	"database/sql"
	"time"
)

func toNullInt64(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.UTC().Unix(), Valid: true}
}

func fromNullInt64(ns sql.NullInt64) *time.Time {
	if !ns.Valid {
		return nil
	}
	t := time.Unix(ns.Int64, 0).UTC()
	return &t
}
