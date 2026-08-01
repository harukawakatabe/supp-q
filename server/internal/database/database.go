package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.MinConns = 0
	config.MaxConns = 10
	config.MaxConnLifetime = 30 * time.Minute
	return pgxpool.NewWithConfig(ctx, config)
}

type Pinger interface {
	Ping(context.Context) error
}

type Checker struct {
	Pool    Pinger
	Timeout time.Duration
}

func (checker Checker) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, checker.Timeout)
	defer cancel()
	return checker.Pool.Ping(pingCtx)
}
