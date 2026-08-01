-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('registered', 'demo_ephemeral')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'pending_deletion', 'deleted')),
    role text NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
    last_activity_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    deleted_at timestamptz
);

CREATE TABLE auth_identities (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider text NOT NULL CHECK (provider IN ('email_code', 'email_password', 'wechat')),
    provider_subject text NOT NULL,
    secret_hash text,
    verified_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (provider, provider_subject),
    UNIQUE (user_id, provider, provider_subject)
);

CREATE TABLE workspaces (
    id uuid PRIMARY KEY,
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('registered', 'demo')),
    name text NOT NULL,
    last_activity_at timestamptz NOT NULL,
    expires_at timestamptz,
    created_at timestamptz NOT NULL,
    CHECK ((kind = 'demo' AND expires_at IS NOT NULL) OR (kind = 'registered' AND expires_at IS NULL))
);

CREATE TABLE workspace_members (
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL DEFAULT 'owner' CHECK (role IN ('owner', 'member')),
    created_at timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    token_digest char(64) NOT NULL UNIQUE,
    kind text NOT NULL CHECK (kind IN ('registered', 'demo')),
    expires_at timestamptz NOT NULL,
    last_activity_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    invalidated_at timestamptz
);

CREATE INDEX sessions_active_lookup_idx ON sessions (token_digest, expires_at)
    WHERE invalidated_at IS NULL;

CREATE TABLE invitations (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('generic_code', 'email_bound')),
    secret_digest char(64) NOT NULL UNIQUE,
    email_normalized text,
    max_uses integer NOT NULL CHECK (max_uses > 0),
    use_count integer NOT NULL DEFAULT 0 CHECK (use_count >= 0),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL,
    CHECK ((kind = 'email_bound' AND email_normalized IS NOT NULL AND max_uses = 1)
        OR (kind = 'generic_code' AND email_normalized IS NULL))
);

CREATE TABLE invitation_acceptances (
    id uuid PRIMARY KEY,
    invitation_id uuid NOT NULL REFERENCES invitations(id),
    user_id uuid NOT NULL REFERENCES users(id),
    email_normalized text NOT NULL,
    accepted_at timestamptz NOT NULL,
    UNIQUE (invitation_id, user_id)
);

CREATE TABLE email_challenges (
    id uuid PRIMARY KEY,
    email_normalized text NOT NULL,
    purpose text NOT NULL CHECK (purpose IN ('sign_in', 'reset_password')),
    code_digest char(64) NOT NULL,
    invitation_id uuid REFERENCES invitations(id),
    expires_at timestamptz NOT NULL,
    attempts integer NOT NULL DEFAULT 0,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL
);

CREATE INDEX email_challenges_active_idx
    ON email_challenges (email_normalized, purpose, created_at DESC)
    WHERE consumed_at IS NULL;

CREATE TABLE demo_cleanup_jobs (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    run_after timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'failed')),
    attempts integer NOT NULL DEFAULT 0,
    last_error text,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE INDEX demo_cleanup_due_idx ON demo_cleanup_jobs (run_after)
    WHERE status IN ('pending', 'failed');

-- Future identity binding deliberately has schema support but no shipped WeChat UI.

-- +goose Down
DROP TABLE IF EXISTS demo_cleanup_jobs;
DROP TABLE IF EXISTS email_challenges;
DROP TABLE IF EXISTS invitation_acceptances;
DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS auth_identities;
DROP TABLE IF EXISTS users;
