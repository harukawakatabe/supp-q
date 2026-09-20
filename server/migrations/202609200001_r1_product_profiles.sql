-- +goose Up
CREATE UNIQUE INDEX products_tenant_identity_uidx
    ON products (id, user_id, workspace_id);
CREATE UNIQUE INDEX files_tenant_identity_uidx
    ON files (id, user_id, workspace_id);

ALTER TABLE products
    ADD COLUMN catalog_state text
        CONSTRAINT products_catalog_state_check
        CHECK (catalog_state IS NULL OR catalog_state IN ('in_cabinet', 'archived', 'deleting', 'deleted')),
    ADD COLUMN aggregate_version bigint
        CONSTRAINT products_aggregate_version_check
        CHECK (aggregate_version IS NULL OR aggregate_version > 0),
    ADD COLUMN current_product_profile_version_id uuid,
    ADD COLUMN current_ingredient_profile_version_id uuid;

CREATE TABLE product_profile_versions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    business_version integer NOT NULL
        CONSTRAINT product_profile_versions_business_version_check CHECK (business_version > 0),
    name text NOT NULL
        CONSTRAINT product_profile_versions_name_check CHECK (char_length(name) BETWEEN 1 AND 160),
    brand text NOT NULL DEFAULT ''
        CONSTRAINT product_profile_versions_brand_check CHECK (char_length(brand) <= 120),
    product_form text NOT NULL DEFAULT ''
        CONSTRAINT product_profile_versions_form_check CHECK (char_length(product_form) <= 120),
    management_unit text NOT NULL
        CONSTRAINT product_profile_versions_unit_check CHECK (char_length(management_unit) BETWEEN 1 AND 40),
    source text NOT NULL
        CONSTRAINT product_profile_versions_source_check
        CHECK (source IN ('legacy_current', 'manual', 'capture_confirmed')),
    history_completeness text NOT NULL
        CONSTRAINT product_profile_versions_history_check
        CHECK (history_completeness IN ('complete', 'legacy_current_only')),
    source_updated_at timestamptz NOT NULL,
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    created_at timestamptz NOT NULL,
    CONSTRAINT product_profile_versions_interval_check
        CHECK (effective_to IS NULL OR effective_to > effective_from),
    CONSTRAINT product_profile_versions_product_fk
        FOREIGN KEY (product_id, user_id, workspace_id)
        REFERENCES products (id, user_id, workspace_id) ON DELETE CASCADE,
    UNIQUE (product_id, business_version),
    UNIQUE (id, product_id, user_id, workspace_id)
);

CREATE UNIQUE INDEX product_profile_versions_current_uidx
    ON product_profile_versions (product_id)
    WHERE effective_to IS NULL;
CREATE INDEX product_profile_versions_owner_idx
    ON product_profile_versions (user_id, workspace_id, product_id, business_version DESC);

CREATE TABLE ingredient_profile_versions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    business_version integer NOT NULL
        CONSTRAINT ingredient_profile_versions_business_version_check CHECK (business_version > 0),
    serving_quantity numeric(20,6) NOT NULL
        CONSTRAINT ingredient_profile_versions_serving_quantity_check CHECK (serving_quantity > 0),
    serving_unit text NOT NULL
        CONSTRAINT ingredient_profile_versions_serving_unit_check CHECK (char_length(serving_unit) BETWEEN 1 AND 40),
    serving_relation_state text NOT NULL
        CONSTRAINT ingredient_profile_versions_serving_relation_check
        CHECK (serving_relation_state IN ('confirmed', 'legacy_assumed_same_unit', 'unknown')),
    profile_status text NOT NULL
        CONSTRAINT ingredient_profile_versions_status_check
        CHECK (profile_status IN ('complete', 'partial', 'confirmed_empty')),
    change_kind text NOT NULL
        CONSTRAINT ingredient_profile_versions_change_kind_check
        CHECK (change_kind IN ('initial', 'legacy_import', 'new_formula', 'correction')),
    history_completeness text NOT NULL
        CONSTRAINT ingredient_profile_versions_history_check
        CHECK (history_completeness IN ('complete', 'legacy_current_only')),
    source_updated_at timestamptz NOT NULL,
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    created_at timestamptz NOT NULL,
    CONSTRAINT ingredient_profile_versions_interval_check
        CHECK (effective_to IS NULL OR effective_to > effective_from),
    CONSTRAINT ingredient_profile_versions_product_fk
        FOREIGN KEY (product_id, user_id, workspace_id)
        REFERENCES products (id, user_id, workspace_id) ON DELETE CASCADE,
    UNIQUE (product_id, business_version),
    UNIQUE (id, product_id, user_id, workspace_id)
);

CREATE UNIQUE INDEX ingredient_profile_versions_current_uidx
    ON ingredient_profile_versions (product_id)
    WHERE effective_to IS NULL;
CREATE INDEX ingredient_profile_versions_owner_idx
    ON ingredient_profile_versions (user_id, workspace_id, product_id, business_version DESC);

CREATE TABLE ingredient_profile_items (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    ingredient_profile_version_id uuid NOT NULL,
    item_order integer NOT NULL
        CONSTRAINT ingredient_profile_items_order_check CHECK (item_order >= 0),
    legacy_ingredient_id uuid,
    original_key text NOT NULL
        CONSTRAINT ingredient_profile_items_key_check CHECK (char_length(original_key) BETWEEN 1 AND 120),
    original_name text NOT NULL
        CONSTRAINT ingredient_profile_items_name_check CHECK (char_length(original_name) BETWEEN 1 AND 120),
    translated_name text,
    label_amount numeric(20,6),
    label_unit text,
    ingredient_definition_id uuid,
    mapping_status text NOT NULL DEFAULT 'unmapped'
        CONSTRAINT ingredient_profile_items_mapping_check
        CHECK (mapping_status IN ('exact', 'alias_matched', 'user_confirmed', 'ambiguous', 'unmapped')),
    form_original text NOT NULL DEFAULT ''
        CONSTRAINT ingredient_profile_items_form_check CHECK (char_length(form_original) <= 120),
    daily_value_percent numeric(12,6),
    normalization_source_version text NOT NULL DEFAULT ''
        CONSTRAINT ingredient_profile_items_source_version_check
        CHECK (char_length(normalization_source_version) <= 120),
    created_at timestamptz NOT NULL,
    CONSTRAINT ingredient_profile_items_amount_check CHECK (label_amount IS NULL OR label_amount >= 0),
    CONSTRAINT ingredient_profile_items_unit_check CHECK (label_unit IS NULL OR char_length(label_unit) BETWEEN 1 AND 40),
    CONSTRAINT ingredient_profile_items_translation_check CHECK (translated_name IS NULL OR char_length(translated_name) <= 160),
    CONSTRAINT ingredient_profile_items_daily_value_check CHECK (daily_value_percent IS NULL OR daily_value_percent >= 0),
    CONSTRAINT ingredient_profile_items_profile_fk
        FOREIGN KEY (ingredient_profile_version_id, product_id, user_id, workspace_id)
        REFERENCES ingredient_profile_versions (id, product_id, user_id, workspace_id) ON DELETE CASCADE,
    UNIQUE (ingredient_profile_version_id, item_order),
    UNIQUE (ingredient_profile_version_id, legacy_ingredient_id)
);

CREATE INDEX ingredient_profile_items_owner_idx
    ON ingredient_profile_items (user_id, workspace_id, product_id, ingredient_profile_version_id, item_order);

CREATE TABLE product_media_links (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    file_id uuid NOT NULL,
    purpose text NOT NULL
        CONSTRAINT product_media_links_purpose_check CHECK (purpose IN ('front', 'facts', 'supporting')),
    sort_order integer NOT NULL DEFAULT 0
        CONSTRAINT product_media_links_order_check CHECK (sort_order >= 0),
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    created_at timestamptz NOT NULL,
    CONSTRAINT product_media_links_interval_check CHECK (effective_to IS NULL OR effective_to > effective_from),
    CONSTRAINT product_media_links_product_fk
        FOREIGN KEY (product_id, user_id, workspace_id)
        REFERENCES products (id, user_id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT product_media_links_file_fk
        FOREIGN KEY (file_id, user_id, workspace_id)
        REFERENCES files (id, user_id, workspace_id) ON DELETE RESTRICT,
    UNIQUE (product_id, purpose, sort_order, effective_from)
);

CREATE UNIQUE INDEX product_media_links_current_front_uidx
    ON product_media_links (product_id)
    WHERE purpose = 'front' AND effective_to IS NULL;
CREATE INDEX product_media_links_owner_idx
    ON product_media_links (user_id, workspace_id, product_id, purpose, sort_order);

CREATE TABLE product_deletion_jobs (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    deletion_state text NOT NULL DEFAULT 'pending'
        CONSTRAINT product_deletion_jobs_state_check
        CHECK (deletion_state IN ('pending', 'running', 'failed', 'succeeded')),
    phase text NOT NULL DEFAULT 'mark_deleting'
        CONSTRAINT product_deletion_jobs_phase_check
        CHECK (phase IN ('mark_deleting', 'delete_objects', 'delete_business_data', 'finalize')),
    cursor jsonb NOT NULL DEFAULT '{}'::jsonb
        CONSTRAINT product_deletion_jobs_cursor_check CHECK (jsonb_typeof(cursor) = 'object'),
    attempt_count integer NOT NULL DEFAULT 0
        CONSTRAINT product_deletion_jobs_attempt_check CHECK (attempt_count BETWEEN 0 AND 1000),
    run_after timestamptz NOT NULL,
    lease_until timestamptz,
    error_code text NOT NULL DEFAULT ''
        CONSTRAINT product_deletion_jobs_error_check CHECK (char_length(error_code) <= 120),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    completed_at timestamptz,
    CONSTRAINT product_deletion_jobs_completion_check
        CHECK ((deletion_state = 'succeeded' AND completed_at IS NOT NULL)
            OR (deletion_state <> 'succeeded' AND completed_at IS NULL)),
    CONSTRAINT product_deletion_jobs_product_fk
        FOREIGN KEY (product_id, user_id, workspace_id)
        REFERENCES products (id, user_id, workspace_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX product_deletion_jobs_active_uidx
    ON product_deletion_jobs (product_id)
    WHERE deletion_state IN ('pending', 'running', 'failed');
CREATE INDEX product_deletion_jobs_claim_idx
    ON product_deletion_jobs (deletion_state, run_after, updated_at)
    WHERE deletion_state IN ('pending', 'running', 'failed');

CREATE TABLE r1_product_profile_backfill_state (
    singleton boolean PRIMARY KEY DEFAULT true
        CONSTRAINT r1_product_profile_backfill_state_singleton_check CHECK (singleton),
    cycle_id uuid,
    cycle_state text NOT NULL DEFAULT 'idle'
        CONSTRAINT r1_product_profile_backfill_state_cycle_check CHECK (cycle_state IN ('idle', 'running')),
    cursor_product_id uuid,
    processed_count bigint NOT NULL DEFAULT 0
        CONSTRAINT r1_product_profile_backfill_state_processed_check CHECK (processed_count >= 0),
    cycle_started_at timestamptz,
    last_completed_at timestamptz,
    updated_at timestamptz NOT NULL,
    CONSTRAINT r1_product_profile_backfill_state_running_check
        CHECK ((cycle_state = 'running' AND cycle_id IS NOT NULL AND cycle_started_at IS NOT NULL)
            OR (cycle_state = 'idle' AND cursor_product_id IS NULL))
);

INSERT INTO r1_product_profile_backfill_state (singleton, updated_at)
VALUES (true, CURRENT_TIMESTAMP);

ALTER TABLE inventory_batches ADD COLUMN ingredient_profile_version_id uuid;

-- +goose StatementBegin
CREATE FUNCTION r1_validate_product_profile_version() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.product_id::text, 0));
    IF TG_OP = 'UPDATE' THEN
        IF (to_jsonb(NEW) - 'effective_to') IS DISTINCT FROM (to_jsonb(OLD) - 'effective_to')
           OR OLD.effective_to IS NOT NULL
           OR NEW.effective_to IS NULL THEN
            RAISE EXCEPTION 'product profile versions are immutable after activation'
                USING ERRCODE = '23514';
        END IF;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM product_profile_versions existing
        WHERE existing.product_id = NEW.product_id
          AND existing.id <> NEW.id
          AND tstzrange(existing.effective_from, existing.effective_to, '[)')
              && tstzrange(NEW.effective_from, NEW.effective_to, '[)')
    ) THEN
        RAISE EXCEPTION 'product profile effective intervals overlap'
            USING ERRCODE = '23P01';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_validate_ingredient_profile_version() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.product_id::text, 1));
    IF TG_OP = 'UPDATE' THEN
        IF (to_jsonb(NEW) - 'effective_to') IS DISTINCT FROM (to_jsonb(OLD) - 'effective_to')
           OR OLD.effective_to IS NOT NULL
           OR NEW.effective_to IS NULL THEN
            RAISE EXCEPTION 'ingredient profile versions are immutable after activation'
                USING ERRCODE = '23514';
        END IF;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM ingredient_profile_versions existing
        WHERE existing.product_id = NEW.product_id
          AND existing.id <> NEW.id
          AND tstzrange(existing.effective_from, existing.effective_to, '[)')
              && tstzrange(NEW.effective_from, NEW.effective_to, '[)')
    ) THEN
        RAISE EXCEPTION 'ingredient profile effective intervals overlap'
            USING ERRCODE = '23P01';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_block_immutable_version_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' AND pg_trigger_depth() > 1 THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'activated profile facts are immutable'
        USING ERRCODE = '23514';
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_validate_ingredient_profile_item() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    parent_product_id uuid;
    parent_effective_to timestamptz;
BEGIN
    IF TG_OP = 'DELETE' AND pg_trigger_depth() > 1 THEN
        RETURN OLD;
    END IF;
    IF TG_OP IN ('UPDATE', 'DELETE') THEN
        RAISE EXCEPTION 'activated profile facts are immutable'
            USING ERRCODE = '23514';
    END IF;

    SELECT product_id, effective_to
    INTO parent_product_id, parent_effective_to
    FROM ingredient_profile_versions
    WHERE id = NEW.ingredient_profile_version_id;
    IF parent_product_id IS NULL THEN
        RETURN NEW;
    END IF;

    PERFORM pg_advisory_xact_lock(hashtextextended(parent_product_id::text, 1));
    IF parent_effective_to IS NOT NULL
       OR EXISTS (
           SELECT 1
           FROM products
           WHERE id = parent_product_id
             AND current_ingredient_profile_version_id = NEW.ingredient_profile_version_id
       ) THEN
        RAISE EXCEPTION 'ingredient profile items cannot be appended after activation'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER product_profile_versions_validate
    BEFORE INSERT OR UPDATE ON product_profile_versions
    FOR EACH ROW EXECUTE FUNCTION r1_validate_product_profile_version();
CREATE TRIGGER product_profile_versions_block_delete
    BEFORE DELETE ON product_profile_versions
    FOR EACH ROW EXECUTE FUNCTION r1_block_immutable_version_change();
CREATE TRIGGER ingredient_profile_versions_validate
    BEFORE INSERT OR UPDATE ON ingredient_profile_versions
    FOR EACH ROW EXECUTE FUNCTION r1_validate_ingredient_profile_version();
CREATE TRIGGER ingredient_profile_versions_block_delete
    BEFORE DELETE ON ingredient_profile_versions
    FOR EACH ROW EXECUTE FUNCTION r1_block_immutable_version_change();
CREATE TRIGGER ingredient_profile_items_validate
    BEFORE INSERT OR UPDATE OR DELETE ON ingredient_profile_items
    FOR EACH ROW EXECUTE FUNCTION r1_validate_ingredient_profile_item();

-- +goose StatementBegin
CREATE FUNCTION r1_backfill_product_profiles_batch(batch_size integer DEFAULT 100) RETURNS boolean
LANGUAGE plpgsql AS $$
DECLARE
    state_cycle text;
    state_cursor uuid;
    product_row record;
    processed_in_batch integer := 0;
    last_product_id uuid;
    current_product_profile_id uuid;
    current_product_business_version integer;
    current_product_effective_from timestamptz;
    current_product_name text;
    current_product_brand text;
    current_product_unit text;
    current_ingredient_profile_id uuid;
    current_ingredient_business_version integer;
    current_ingredient_effective_from timestamptz;
    current_serving_quantity numeric(20,6);
    current_serving_unit text;
    source_items jsonb;
    target_items jsonb;
    next_profile_id uuid;
    next_business_version integer;
    next_effective_from timestamptz;
    product_profile_changed boolean;
    ingredient_profile_changed boolean;
    target_fact_changed boolean;
BEGIN
    IF batch_size < 1 OR batch_size > 10000 THEN
        RAISE EXCEPTION 'product profile backfill batch size must be between 1 and 10000'
            USING ERRCODE = '22023';
    END IF;

    PERFORM pg_advisory_xact_lock(hashtextextended('r1_product_profile_backfill', 0));

    INSERT INTO migration_quarantines (
        source_table, source_id, reason_code, user_id, workspace_id, state, discovered_at
    )
    SELECT 'products', id, 'unsupported_product_type', user_id, workspace_id, 'open', CURRENT_TIMESTAMP
    FROM products
    WHERE product_type IN ('otc', 'prescription')
    ON CONFLICT (source_table, source_id, reason_code) DO NOTHING;

    SELECT cycle_state, cursor_product_id
    INTO state_cycle, state_cursor
    FROM r1_product_profile_backfill_state
    WHERE singleton
    FOR UPDATE;

    IF state_cycle = 'idle' THEN
        state_cursor := NULL;
        UPDATE r1_product_profile_backfill_state
        SET cycle_id = gen_random_uuid(),
            cycle_state = 'running',
            cursor_product_id = NULL,
            processed_count = 0,
            cycle_started_at = CURRENT_TIMESTAMP,
            updated_at = CURRENT_TIMESTAMP
        WHERE singleton;
    END IF;

    FOR product_row IN
        SELECT p.*
        FROM products p
        WHERE p.product_type = 'supplement'
          AND (state_cursor IS NULL OR p.id > state_cursor)
        ORDER BY p.id
        LIMIT batch_size
    LOOP
        processed_in_batch := processed_in_batch + 1;
        last_product_id := product_row.id;
        target_fact_changed := false;

        current_product_profile_id := NULL;
        current_product_business_version := NULL;
        current_product_effective_from := NULL;
        current_product_name := NULL;
        current_product_brand := NULL;
        current_product_unit := NULL;
        SELECT id, business_version, effective_from, name, brand, management_unit
        INTO current_product_profile_id, current_product_business_version,
             current_product_effective_from, current_product_name,
             current_product_brand, current_product_unit
        FROM product_profile_versions
        WHERE id = product_row.current_product_profile_version_id
          AND product_id = product_row.id
          AND user_id = product_row.user_id
          AND workspace_id = product_row.workspace_id;

        product_profile_changed := current_product_profile_id IS NULL
            OR current_product_name IS DISTINCT FROM product_row.name
            OR current_product_brand IS DISTINCT FROM product_row.brand
            OR current_product_unit IS DISTINCT FROM product_row.unit;

        IF product_profile_changed THEN
            IF current_product_profile_id IS NOT NULL THEN
                next_effective_from := GREATEST(
                    CURRENT_TIMESTAMP,
                    product_row.updated_at,
                    current_product_effective_from + interval '1 microsecond'
                );
                UPDATE product_profile_versions
                SET effective_to = next_effective_from
                WHERE id = current_product_profile_id
                  AND product_id = product_row.id
                  AND effective_to IS NULL;
                target_fact_changed := true;
            ELSE
                next_effective_from := product_row.updated_at;
            END IF;

            SELECT COALESCE(max(business_version), 0) + 1
            INTO next_business_version
            FROM product_profile_versions
            WHERE product_id = product_row.id;
            next_profile_id := gen_random_uuid();
            INSERT INTO product_profile_versions (
                id, user_id, workspace_id, product_id, business_version,
                name, brand, product_form, management_unit, source, history_completeness,
                source_updated_at, effective_from, created_at
            ) VALUES (
                next_profile_id, product_row.user_id, product_row.workspace_id,
                product_row.id, next_business_version, product_row.name,
                product_row.brand, '', product_row.unit, 'legacy_current',
                'legacy_current_only', product_row.updated_at,
                next_effective_from, CURRENT_TIMESTAMP
            );
            current_product_profile_id := next_profile_id;
        END IF;

        current_ingredient_profile_id := NULL;
        current_ingredient_business_version := NULL;
        current_ingredient_effective_from := NULL;
        current_serving_quantity := NULL;
        current_serving_unit := NULL;
        SELECT id, business_version, effective_from, serving_quantity, serving_unit
        INTO current_ingredient_profile_id, current_ingredient_business_version,
             current_ingredient_effective_from, current_serving_quantity,
             current_serving_unit
        FROM ingredient_profile_versions
        WHERE id = product_row.current_ingredient_profile_version_id
          AND product_id = product_row.id
          AND user_id = product_row.user_id
          AND workspace_id = product_row.workspace_id;

        SELECT COALESCE(
            jsonb_agg(
                jsonb_build_array(id::text, ingredient_key, name, amount::text, unit)
                ORDER BY created_at, id
            ),
            '[]'::jsonb
        )
        INTO source_items
        FROM product_ingredients
        WHERE product_id = product_row.id
          AND user_id = product_row.user_id
          AND workspace_id = product_row.workspace_id;

        SELECT COALESCE(
            jsonb_agg(
                jsonb_build_array(legacy_ingredient_id::text, original_key, original_name, label_amount::text, label_unit)
                ORDER BY item_order
            ),
            '[]'::jsonb
        )
        INTO target_items
        FROM ingredient_profile_items
        WHERE ingredient_profile_version_id = current_ingredient_profile_id
          AND product_id = product_row.id
          AND user_id = product_row.user_id
          AND workspace_id = product_row.workspace_id;

        ingredient_profile_changed := current_ingredient_profile_id IS NULL
            OR current_serving_quantity IS DISTINCT FROM product_row.ingredient_serving_quantity
            OR current_serving_unit IS DISTINCT FROM product_row.unit
            OR target_items IS DISTINCT FROM source_items;

        IF ingredient_profile_changed THEN
            IF current_ingredient_profile_id IS NOT NULL THEN
                next_effective_from := GREATEST(
                    CURRENT_TIMESTAMP,
                    product_row.updated_at,
                    current_ingredient_effective_from + interval '1 microsecond'
                );
                UPDATE ingredient_profile_versions
                SET effective_to = next_effective_from
                WHERE id = current_ingredient_profile_id
                  AND product_id = product_row.id
                  AND effective_to IS NULL;
                target_fact_changed := true;
            ELSE
                next_effective_from := product_row.updated_at;
            END IF;

            SELECT COALESCE(max(business_version), 0) + 1
            INTO next_business_version
            FROM ingredient_profile_versions
            WHERE product_id = product_row.id;
            next_profile_id := gen_random_uuid();
            INSERT INTO ingredient_profile_versions (
                id, user_id, workspace_id, product_id, business_version,
                serving_quantity, serving_unit, serving_relation_state, profile_status,
                change_kind, history_completeness, source_updated_at, effective_from, created_at
            ) VALUES (
                next_profile_id, product_row.user_id, product_row.workspace_id,
                product_row.id, next_business_version,
                product_row.ingredient_serving_quantity, product_row.unit,
                'legacy_assumed_same_unit', 'partial', 'legacy_import',
                'legacy_current_only', product_row.updated_at,
                next_effective_from, CURRENT_TIMESTAMP
            );

            INSERT INTO ingredient_profile_items (
                id, user_id, workspace_id, product_id, ingredient_profile_version_id,
                item_order, legacy_ingredient_id, original_key, original_name,
                label_amount, label_unit, mapping_status, created_at
            )
            SELECT gen_random_uuid(), pi.user_id, pi.workspace_id, pi.product_id,
                   next_profile_id,
                   row_number() OVER (ORDER BY pi.created_at, pi.id) - 1,
                   pi.id, pi.ingredient_key, pi.name, pi.amount, pi.unit,
                   'unmapped', CURRENT_TIMESTAMP
            FROM product_ingredients pi
            WHERE pi.product_id = product_row.id
              AND pi.user_id = product_row.user_id
              AND pi.workspace_id = product_row.workspace_id
            ORDER BY pi.created_at, pi.id;
            current_ingredient_profile_id := next_profile_id;
        END IF;

        UPDATE products
        SET catalog_state = COALESCE(
                catalog_state,
                CASE WHEN status = 'archived' THEN 'archived' ELSE 'in_cabinet' END
            ),
            aggregate_version = CASE
                WHEN target_fact_changed THEN COALESCE(aggregate_version, 1) + 1
                ELSE COALESCE(aggregate_version, 1)
            END,
            current_product_profile_version_id = current_product_profile_id,
            current_ingredient_profile_version_id = current_ingredient_profile_id
        WHERE id = product_row.id
          AND user_id = product_row.user_id
          AND workspace_id = product_row.workspace_id;

        UPDATE inventory_batches
        SET ingredient_profile_version_id = current_ingredient_profile_id
        WHERE product_id = product_row.id
          AND user_id = product_row.user_id
          AND workspace_id = product_row.workspace_id
          AND ingredient_profile_version_id IS NULL;
    END LOOP;

    IF processed_in_batch = 0
       OR NOT EXISTS (
           SELECT 1
           FROM products
           WHERE product_type = 'supplement'
             AND id > last_product_id
       ) THEN
        UPDATE r1_product_profile_backfill_state
        SET cycle_state = 'idle',
            cursor_product_id = NULL,
            processed_count = processed_count + processed_in_batch,
            last_completed_at = CURRENT_TIMESTAMP,
            updated_at = CURRENT_TIMESTAMP
        WHERE singleton;
        RETURN true;
    END IF;

    UPDATE r1_product_profile_backfill_state
    SET cursor_product_id = last_product_id,
        processed_count = processed_count + processed_in_batch,
        updated_at = CURRENT_TIMESTAMP
    WHERE singleton;
    RETURN false;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_backfill_product_profiles() RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    cycle_complete boolean := false;
BEGIN
    WHILE NOT cycle_complete LOOP
        cycle_complete := r1_backfill_product_profiles_batch(100);
    END LOOP;
END;
$$;
-- +goose StatementEnd

SELECT r1_backfill_product_profiles();

ALTER TABLE products
    ADD CONSTRAINT products_current_product_profile_fk
        FOREIGN KEY (current_product_profile_version_id, id, user_id, workspace_id)
        REFERENCES product_profile_versions (id, product_id, user_id, workspace_id)
        ON DELETE RESTRICT NOT VALID,
    ADD CONSTRAINT products_current_ingredient_profile_fk
        FOREIGN KEY (current_ingredient_profile_version_id, id, user_id, workspace_id)
        REFERENCES ingredient_profile_versions (id, product_id, user_id, workspace_id)
        ON DELETE RESTRICT NOT VALID;

ALTER TABLE inventory_batches
    ADD CONSTRAINT inventory_batches_ingredient_profile_fk
        FOREIGN KEY (ingredient_profile_version_id, product_id, user_id, workspace_id)
        REFERENCES ingredient_profile_versions (id, product_id, user_id, workspace_id)
        ON DELETE RESTRICT NOT VALID;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM product_profile_versions
        WHERE source <> 'legacy_current'
           OR business_version > 1
    ) OR EXISTS (
        SELECT 1
        FROM ingredient_profile_versions
        WHERE change_kind <> 'legacy_import'
           OR business_version > 1
    ) OR EXISTS (
        SELECT 1 FROM product_media_links
    ) OR EXISTS (
        SELECT 1 FROM product_deletion_jobs
    ) OR EXISTS (
        SELECT 1
        FROM products
        WHERE aggregate_version > 1
           OR (product_type = 'supplement'
               AND catalog_state IS DISTINCT FROM
                   CASE WHEN status = 'archived' THEN 'archived' ELSE 'in_cabinet' END)
    ) THEN
        RAISE EXCEPTION 'E2 target writes exist; rollback requires a forward fix'
            USING ERRCODE = '55000';
    END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE inventory_batches DROP CONSTRAINT IF EXISTS inventory_batches_ingredient_profile_fk;
ALTER TABLE inventory_batches DROP COLUMN IF EXISTS ingredient_profile_version_id;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_current_ingredient_profile_fk;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_current_product_profile_fk;
ALTER TABLE products DROP COLUMN IF EXISTS current_ingredient_profile_version_id;
ALTER TABLE products DROP COLUMN IF EXISTS current_product_profile_version_id;
ALTER TABLE products DROP COLUMN IF EXISTS aggregate_version;
ALTER TABLE products DROP COLUMN IF EXISTS catalog_state;
DROP FUNCTION IF EXISTS r1_backfill_product_profiles();
DROP FUNCTION IF EXISTS r1_backfill_product_profiles_batch(integer);
DROP TABLE IF EXISTS r1_product_profile_backfill_state;
DROP TABLE IF EXISTS product_deletion_jobs;
DROP TABLE IF EXISTS product_media_links;
DROP TABLE IF EXISTS ingredient_profile_items;
DROP TABLE IF EXISTS ingredient_profile_versions;
DROP TABLE IF EXISTS product_profile_versions;
DROP FUNCTION IF EXISTS r1_validate_ingredient_profile_item();
DROP FUNCTION IF EXISTS r1_block_immutable_version_change();
DROP FUNCTION IF EXISTS r1_validate_ingredient_profile_version();
DROP FUNCTION IF EXISTS r1_validate_product_profile_version();
DROP INDEX IF EXISTS files_tenant_identity_uidx;
DROP INDEX IF EXISTS products_tenant_identity_uidx;
