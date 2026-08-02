-- +goose Up
CREATE TABLE products (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 160),
    brand text NOT NULL DEFAULT '' CHECK (char_length(brand) <= 120),
    product_type text NOT NULL DEFAULT 'supplement' CHECK (product_type IN ('supplement', 'otc', 'prescription')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'depleted', 'archived')),
    unit text NOT NULL CHECK (char_length(unit) BETWEEN 1 AND 40),
    dose_quantity numeric(20,6) NOT NULL CHECK (dose_quantity > 0),
    dose_times_per_day integer NOT NULL CHECK (dose_times_per_day BETWEEN 1 AND 8),
    ingredient_serving_quantity numeric(20,6) NOT NULL CHECK (ingredient_serving_quantity > 0),
    with_food boolean,
    restock_threshold_days integer NOT NULL DEFAULT 7 CHECK (restock_threshold_days BETWEEN 0 AND 3650),
    expiry_reminder_days integer NOT NULL DEFAULT 30 CHECK (expiry_reminder_days BETWEEN 0 AND 3650),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE INDEX products_owner_status_idx ON products (user_id, workspace_id, status, created_at DESC);

CREATE TABLE product_ingredients (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    ingredient_key text NOT NULL CHECK (char_length(ingredient_key) BETWEEN 1 AND 120),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    amount numeric(20,6) NOT NULL CHECK (amount >= 0),
    unit text NOT NULL CHECK (char_length(unit) BETWEEN 1 AND 20),
    created_at timestamptz NOT NULL,
    UNIQUE (product_id, ingredient_key, unit)
);

CREATE TABLE product_schedules (
    product_id uuid PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    start_date date NOT NULL,
    weekdays smallint[] NOT NULL,
    day_cycle_enabled boolean NOT NULL DEFAULT false,
    day_cycle_days integer NOT NULL DEFAULT 28 CHECK (day_cycle_days > 0),
    day_take_days integer NOT NULL DEFAULT 28 CHECK (day_take_days > 0 AND day_take_days <= day_cycle_days),
    day_anchor_date date NOT NULL,
    long_cycle_enabled boolean NOT NULL DEFAULT false,
    long_take_weeks integer NOT NULL DEFAULT 6 CHECK (long_take_weeks > 0),
    long_rest_weeks integer NOT NULL DEFAULT 4 CHECK (long_rest_weeks >= 0),
    long_start_date date NOT NULL,
    reminder_times text[] NOT NULL DEFAULT ARRAY['09:00'],
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (cardinality(weekdays) BETWEEN 1 AND 7),
    CHECK (weekdays <@ ARRAY[0,1,2,3,4,5,6]::smallint[])
);

CREATE TABLE day_cycle_versions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    effective_date date NOT NULL,
    enabled boolean NOT NULL,
    cycle_days integer NOT NULL CHECK (cycle_days > 0),
    take_days integer NOT NULL CHECK (take_days > 0 AND take_days <= cycle_days),
    anchor_date date NOT NULL,
    created_at timestamptz NOT NULL,
    UNIQUE (product_id, effective_date)
);

CREATE INDEX day_cycle_versions_lookup_idx ON day_cycle_versions (product_id, effective_date DESC);

CREATE TABLE inventory_batches (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    initial_quantity numeric(20,6) NOT NULL CHECK (initial_quantity > 0),
    current_quantity numeric(20,6) NOT NULL CHECK (current_quantity >= 0 AND current_quantity <= initial_quantity),
    expiry_date date,
    price_cny numeric(20,2) NOT NULL DEFAULT 0 CHECK (price_cny >= 0),
    created_at timestamptz NOT NULL
);

CREATE INDEX inventory_batches_fefo_idx ON inventory_batches
    (user_id, workspace_id, product_id, expiry_date ASC NULLS LAST, created_at ASC)
    WHERE current_quantity > 0;

CREATE TABLE intake_records (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    intake_date date NOT NULL,
    intake_time time,
    quantity numeric(20,6) NOT NULL CHECK (quantity > 0),
    source text NOT NULL CHECK (source IN ('scheduled', 'ad_hoc', 'backfill')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    idempotency_key text,
    note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
    created_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CHECK ((status = 'active' AND revoked_at IS NULL) OR (status = 'revoked' AND revoked_at IS NOT NULL))
);

CREATE UNIQUE INDEX intake_records_idempotency_idx ON intake_records (user_id, workspace_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
CREATE INDEX intake_records_day_idx ON intake_records (user_id, workspace_id, intake_date, created_at);

CREATE TABLE intake_allocations (
    intake_id uuid NOT NULL REFERENCES intake_records(id) ON DELETE CASCADE,
    batch_id uuid NOT NULL REFERENCES inventory_batches(id),
    quantity numeric(20,6) NOT NULL CHECK (quantity > 0),
    unit_cost_cny numeric(20,6) NOT NULL DEFAULT 0 CHECK (unit_cost_cny >= 0),
    PRIMARY KEY (intake_id, batch_id)
);

CREATE TABLE inventory_events (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    batch_id uuid NOT NULL REFERENCES inventory_batches(id) ON DELETE CASCADE,
    intake_id uuid REFERENCES intake_records(id) ON DELETE SET NULL,
    kind text NOT NULL CHECK (kind IN ('opening', 'restock', 'intake', 'undo', 'adjustment')),
    quantity_delta numeric(20,6) NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE INDEX inventory_events_product_idx ON inventory_events (user_id, workspace_id, product_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS inventory_events;
DROP TABLE IF EXISTS intake_allocations;
DROP TABLE IF EXISTS intake_records;
DROP TABLE IF EXISTS inventory_batches;
DROP TABLE IF EXISTS day_cycle_versions;
DROP TABLE IF EXISTS product_schedules;
DROP TABLE IF EXISTS product_ingredients;
DROP TABLE IF EXISTS products;
