-- +goose Up
CREATE INDEX recognition_jobs_lease_idx ON recognition_jobs (lease_until)
    WHERE status = 'running';

-- +goose Down
DROP INDEX IF EXISTS recognition_jobs_lease_idx;
