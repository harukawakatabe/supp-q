package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
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

type Checker struct {
	Pool    *pgxpool.Pool
	Timeout time.Duration
}

func (checker Checker) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, checker.Timeout)
	defer cancel()
	return checker.Pool.Ping(pingCtx)
}

type WorkerState struct {
	Provider   string
	Version    string
	LastSeenAt time.Time
}

func (checker Checker) WorkerState(ctx context.Context) (WorkerState, error) {
	checkCtx, cancel := context.WithTimeout(ctx, checker.Timeout)
	defer cancel()
	var state WorkerState
	err := checker.Pool.QueryRow(checkCtx, `SELECT provider,version,last_seen_at FROM worker_heartbeats WHERE worker_name='recognition-cleanup'`).Scan(&state.Provider, &state.Version, &state.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkerState{}, errors.New("worker heartbeat is missing")
	}
	return state, err
}

func (checker Checker) RecognitionQueue(ctx context.Context) (queued, running, failed int, err error) {
	checkCtx, cancel := context.WithTimeout(ctx, checker.Timeout)
	defer cancel()
	err = checker.Pool.QueryRow(checkCtx, `SELECT count(*) FILTER (WHERE status='queued'),count(*) FILTER (WHERE status='running'),count(*) FILTER (WHERE status='failed') FROM recognition_jobs`).Scan(&queued, &running, &failed)
	return
}
