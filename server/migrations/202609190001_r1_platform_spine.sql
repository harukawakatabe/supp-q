-- +goose Up
CREATE TABLE workspace_timezone_versions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    business_version integer NOT NULL CHECK (business_version > 0),
    iana_timezone text NOT NULL CHECK (char_length(iana_timezone) BETWEEN 1 AND 120),
    confirmation_state text NOT NULL CHECK (confirmation_state IN ('needs_confirmation', 'confirmed')),
    source text NOT NULL CHECK (source IN ('legacy_unspecified', 'user_confirmed', 'system_default')),
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    created_at timestamptz NOT NULL,
    CHECK (effective_to IS NULL OR effective_to > effective_from),
    UNIQUE (workspace_id, business_version)
);

CREATE UNIQUE INDEX workspace_timezone_versions_current_uidx
    ON workspace_timezone_versions (workspace_id)
    WHERE effective_to IS NULL;
CREATE INDEX workspace_timezone_versions_owner_idx
    ON workspace_timezone_versions (user_id, workspace_id, business_version DESC);

ALTER TABLE workspaces ADD COLUMN current_timezone_version_id uuid;

INSERT INTO workspace_timezone_versions (
    id,
    user_id,
    workspace_id,
    business_version,
    iana_timezone,
    confirmation_state,
    source,
    effective_from,
    created_at
)
SELECT
    gen_random_uuid(),
    owner_user_id,
    id,
    1,
    'Etc/UTC',
    'needs_confirmation',
    'legacy_unspecified',
    created_at,
    created_at
FROM workspaces;

UPDATE workspaces w
SET current_timezone_version_id = tz.id
FROM workspace_timezone_versions tz
WHERE tz.workspace_id = w.id
  AND tz.effective_to IS NULL;

ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_current_timezone_version_fk
    FOREIGN KEY (current_timezone_version_id)
    REFERENCES workspace_timezone_versions(id)
    ON DELETE RESTRICT;

CREATE TABLE client_actions (
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    client_action_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action_type text NOT NULL CHECK (char_length(action_type) BETWEEN 1 AND 120),
    request_hash char(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    action_state text NOT NULL CHECK (action_state IN ('received', 'processing', 'succeeded', 'rejected', 'failed')),
    result_resource_type text CHECK (result_resource_type IS NULL OR char_length(result_resource_type) BETWEEN 1 AND 80),
    result_resource_id uuid,
    result_aggregate_version bigint CHECK (result_aggregate_version IS NULL OR result_aggregate_version > 0),
    error_code text NOT NULL DEFAULT '' CHECK (char_length(error_code) <= 120),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    completed_at timestamptz,
    PRIMARY KEY (workspace_id, client_action_id),
    CHECK (expires_at > created_at),
    CHECK ((action_state IN ('succeeded', 'rejected', 'failed') AND completed_at IS NOT NULL)
        OR (action_state IN ('received', 'processing') AND completed_at IS NULL)),
    CHECK ((result_resource_type IS NULL AND result_resource_id IS NULL)
        OR (result_resource_type IS NOT NULL AND result_resource_id IS NOT NULL))
);

CREATE INDEX client_actions_owner_created_idx
    ON client_actions (user_id, workspace_id, created_at DESC);
CREATE INDEX client_actions_expiry_idx
    ON client_actions (expires_at)
    WHERE action_state IN ('succeeded', 'rejected', 'failed');

CREATE TABLE domain_changes (
    event_id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    aggregate_type text NOT NULL CHECK (char_length(aggregate_type) BETWEEN 1 AND 80),
    aggregate_id uuid NOT NULL,
    aggregate_version bigint NOT NULL CHECK (aggregate_version > 0),
    change_type text NOT NULL CHECK (char_length(change_type) BETWEEN 1 AND 120),
    actor_type text NOT NULL CHECK (actor_type IN ('user', 'system', 'worker', 'migration')),
    client_action_id uuid,
    correlation_id text NOT NULL DEFAULT '' CHECK (char_length(correlation_id) <= 160),
    payload_schema_version integer NOT NULL DEFAULT 1 CHECK (payload_schema_version > 0),
    minimal_payload jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(minimal_payload) = 'object'),
    occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE INDEX domain_changes_workspace_order_idx
    ON domain_changes (workspace_id, occurred_at, event_id);
CREATE INDEX domain_changes_aggregate_idx
    ON domain_changes (workspace_id, aggregate_type, aggregate_id, aggregate_version DESC);
CREATE INDEX domain_changes_client_action_idx
    ON domain_changes (workspace_id, client_action_id)
    WHERE client_action_id IS NOT NULL;

CREATE TABLE projection_revisions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    projection_type text NOT NULL CHECK (char_length(projection_type) BETWEEN 1 AND 100),
    scope_type text NOT NULL CHECK (char_length(scope_type) BETWEEN 1 AND 80),
    scope_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    projection_state text NOT NULL CHECK (projection_state IN ('building', 'active', 'failed', 'superseded')),
    source_revision_vector jsonb NOT NULL CHECK (jsonb_typeof(source_revision_vector) = 'object'),
    error_code text NOT NULL DEFAULT '' CHECK (char_length(error_code) <= 120),
    calculated_at timestamptz,
    activated_at timestamptz,
    failed_at timestamptz,
    created_at timestamptz NOT NULL,
    UNIQUE (workspace_id, projection_type, scope_type, scope_id, revision),
    CHECK ((projection_state = 'active' AND activated_at IS NOT NULL AND failed_at IS NULL)
        OR (projection_state = 'failed' AND failed_at IS NOT NULL AND activated_at IS NULL)
        OR (projection_state IN ('building', 'superseded') AND failed_at IS NULL))
);

CREATE UNIQUE INDEX projection_revisions_active_uidx
    ON projection_revisions (workspace_id, projection_type, scope_type, scope_id)
    WHERE projection_state = 'active';
CREATE INDEX projection_revisions_owner_idx
    ON projection_revisions (user_id, workspace_id, projection_type, created_at DESC);

CREATE TABLE domain_change_consumer_receipts (
    event_id uuid NOT NULL REFERENCES domain_changes(event_id) ON DELETE CASCADE,
    consumer_name text NOT NULL CHECK (char_length(consumer_name) BETWEEN 1 AND 100),
    receipt_state text NOT NULL CHECK (receipt_state IN ('processing', 'succeeded', 'failed')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count BETWEEN 0 AND 1000),
    lease_until timestamptz,
    next_attempt_at timestamptz,
    error_code text NOT NULL DEFAULT '' CHECK (char_length(error_code) <= 120),
    result_projection_revision_id uuid REFERENCES projection_revisions(id) ON DELETE SET NULL,
    processed_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (event_id, consumer_name),
    CHECK ((receipt_state = 'succeeded' AND processed_at IS NOT NULL)
        OR (receipt_state IN ('processing', 'failed') AND processed_at IS NULL))
);

CREATE INDEX domain_change_consumer_retry_idx
    ON domain_change_consumer_receipts (consumer_name, next_attempt_at, updated_at)
    WHERE receipt_state IN ('processing', 'failed');

CREATE TABLE migration_quarantines (
    source_table text NOT NULL CHECK (char_length(source_table) BETWEEN 1 AND 80),
    source_id uuid NOT NULL,
    reason_code text NOT NULL CHECK (char_length(reason_code) BETWEEN 1 AND 120),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    state text NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'resolved', 'discarded')),
    resolution_code text NOT NULL DEFAULT '' CHECK (char_length(resolution_code) <= 120),
    discovered_at timestamptz NOT NULL,
    resolved_at timestamptz,
    PRIMARY KEY (source_table, source_id, reason_code),
    CHECK ((state = 'open' AND resolved_at IS NULL)
        OR (state IN ('resolved', 'discarded') AND resolved_at IS NOT NULL))
);

CREATE INDEX migration_quarantines_workspace_state_idx
    ON migration_quarantines (workspace_id, state, reason_code, discovered_at);

INSERT INTO migration_quarantines (
    source_table,
    source_id,
    reason_code,
    user_id,
    workspace_id,
    state,
    discovered_at
)
SELECT
    'products',
    id,
    'unsupported_product_type',
    user_id,
    workspace_id,
    'open',
    CURRENT_TIMESTAMP
FROM products
WHERE product_type IN ('otc', 'prescription')
ON CONFLICT (source_table, source_id, reason_code) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS migration_quarantines;
DROP TABLE IF EXISTS domain_change_consumer_receipts;
DROP TABLE IF EXISTS projection_revisions;
DROP TABLE IF EXISTS domain_changes;
DROP TABLE IF EXISTS client_actions;
ALTER TABLE workspaces DROP CONSTRAINT IF EXISTS workspaces_current_timezone_version_fk;
ALTER TABLE workspaces DROP COLUMN IF EXISTS current_timezone_version_id;
DROP TABLE IF EXISTS workspace_timezone_versions;
