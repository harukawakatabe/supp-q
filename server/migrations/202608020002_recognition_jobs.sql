-- +goose Up
ALTER TABLE products ADD COLUMN source_recognition_set_id uuid;

CREATE TABLE files (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    object_key text NOT NULL UNIQUE CHECK (char_length(object_key) BETWEEN 1 AND 500),
    original_name text NOT NULL DEFAULT '' CHECK (char_length(original_name) <= 180),
    mime_type text NOT NULL CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/webp')),
    byte_size bigint NOT NULL CHECK (byte_size BETWEEN 1 AND 10485760),
    sha256_hex text NOT NULL CHECK (sha256_hex ~ '^[0-9a-f]{64}$'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deleted')),
    created_at timestamptz NOT NULL,
    deleted_at timestamptz
);

CREATE INDEX files_owner_created_idx ON files (user_id, workspace_id, created_at DESC);

CREATE TABLE recognition_sets (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'processing', 'awaiting_confirmation', 'confirmed', 'cancelled')),
    product_id uuid REFERENCES products(id) ON DELETE SET NULL,
    confirmed_payload jsonb,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    confirmed_at timestamptz
);

ALTER TABLE products
    ADD CONSTRAINT products_source_recognition_set_fk
    FOREIGN KEY (source_recognition_set_id) REFERENCES recognition_sets(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX products_source_recognition_set_uidx ON products (source_recognition_set_id)
    WHERE source_recognition_set_id IS NOT NULL;

CREATE INDEX recognition_sets_owner_status_idx ON recognition_sets (user_id, workspace_id, status, created_at DESC);

CREATE TABLE recognition_files (
    recognition_set_id uuid NOT NULL REFERENCES recognition_sets(id) ON DELETE CASCADE,
    file_id uuid NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('front', 'facts', 'expiry')),
    PRIMARY KEY (recognition_set_id, role),
    UNIQUE (file_id)
);

CREATE TABLE recognition_jobs (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    recognition_set_id uuid NOT NULL REFERENCES recognition_sets(id) ON DELETE CASCADE,
    file_id uuid NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('front', 'facts', 'expiry')),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'partial', 'failed', 'cancelled')),
    provider text NOT NULL DEFAULT 'pending' CHECK (char_length(provider) BETWEEN 1 AND 120),
    attempt integer NOT NULL DEFAULT 0 CHECK (attempt BETWEEN 0 AND 20),
    max_attempts integer NOT NULL DEFAULT 3 CHECK (max_attempts BETWEEN 1 AND 20),
    confidence numeric(5,4) NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 1),
    result jsonb,
    error_code text NOT NULL DEFAULT '' CHECK (char_length(error_code) <= 120),
    error_message text NOT NULL DEFAULT '' CHECK (char_length(error_message) <= 500),
    run_after timestamptz NOT NULL,
    lease_until timestamptz,
    created_at timestamptz NOT NULL,
    started_at timestamptz,
    completed_at timestamptz,
    updated_at timestamptz NOT NULL,
    UNIQUE (recognition_set_id, role)
);

CREATE INDEX recognition_jobs_claim_idx ON recognition_jobs (status, run_after, created_at)
    WHERE status IN ('queued', 'running');
CREATE INDEX recognition_jobs_owner_set_idx ON recognition_jobs (user_id, workspace_id, recognition_set_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS recognition_jobs;
DROP TABLE IF EXISTS recognition_files;
DROP INDEX IF EXISTS products_source_recognition_set_uidx;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_source_recognition_set_fk;
DROP TABLE IF EXISTS recognition_sets;
DROP TABLE IF EXISTS files;
ALTER TABLE products DROP COLUMN IF EXISTS source_recognition_set_id;
