-- +goose Up
CREATE TABLE account_cleanup_jobs (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    run_after timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 100),
    last_error text NOT NULL DEFAULT '' CHECK (char_length(last_error) <= 500),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE INDEX account_cleanup_due_idx ON account_cleanup_jobs (run_after, updated_at)
    WHERE status IN ('pending', 'running', 'failed');

CREATE TABLE worker_heartbeats (
    worker_name text PRIMARY KEY CHECK (char_length(worker_name) BETWEEN 1 AND 80),
    provider text NOT NULL DEFAULT '' CHECK (char_length(provider) <= 120),
    version text NOT NULL DEFAULT '' CHECK (char_length(version) <= 120),
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_seen_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS worker_heartbeats;
DROP TABLE IF EXISTS account_cleanup_jobs;
