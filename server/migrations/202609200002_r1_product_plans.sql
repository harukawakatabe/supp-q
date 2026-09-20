-- +goose Up
CREATE UNIQUE INDEX IF NOT EXISTS workspace_timezone_versions_tenant_identity_uidx
    ON workspace_timezone_versions (id, user_id, workspace_id);

ALTER TABLE workspaces DROP CONSTRAINT IF EXISTS workspaces_current_timezone_version_fk;
ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_current_timezone_version_fk
        FOREIGN KEY (current_timezone_version_id, owner_user_id, id)
        REFERENCES workspace_timezone_versions (id, user_id, workspace_id)
        ON DELETE RESTRICT NOT VALID;

-- +goose StatementBegin
CREATE FUNCTION r1_validate_workspace_timezone_version() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.workspace_id::text, 4));
    IF TG_OP = 'UPDATE' THEN
        IF (to_jsonb(NEW) - 'effective_to') IS DISTINCT FROM (to_jsonb(OLD) - 'effective_to')
           OR OLD.effective_to IS NOT NULL
           OR NEW.effective_to IS NULL THEN
            RAISE EXCEPTION 'workspace timezone versions are immutable after activation'
                USING ERRCODE = '23514';
        END IF;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM workspace_timezone_versions existing
        WHERE existing.workspace_id = NEW.workspace_id
          AND existing.id <> NEW.id
          AND tstzrange(existing.effective_from, existing.effective_to, '[)')
              && tstzrange(NEW.effective_from, NEW.effective_to, '[)')
    ) THEN
        RAISE EXCEPTION 'workspace timezone effective intervals overlap'
            USING ERRCODE = '23P01';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_block_timezone_history_delete() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF pg_trigger_depth() > 1 THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'workspace timezone history is immutable'
        USING ERRCODE = '23514';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER workspace_timezone_versions_validate
    BEFORE INSERT OR UPDATE ON workspace_timezone_versions
    FOR EACH ROW EXECUTE FUNCTION r1_validate_workspace_timezone_version();
CREATE TRIGGER workspace_timezone_versions_block_delete
    BEFORE DELETE ON workspace_timezone_versions
    FOR EACH ROW EXECUTE FUNCTION r1_block_timezone_history_delete();

CREATE TABLE product_plans (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    aggregate_version integer NOT NULL DEFAULT 1 CHECK (aggregate_version > 0),
    current_schedule_version_id uuid,
    state_history_completeness text NOT NULL DEFAULT 'legacy_current_only'
        CHECK (state_history_completeness IN ('complete', 'legacy_current_only')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (product_id),
    UNIQUE (id, product_id, user_id, workspace_id),
    CONSTRAINT product_plans_product_fk
        FOREIGN KEY (product_id, user_id, workspace_id)
        REFERENCES products (id, user_id, workspace_id)
        ON DELETE CASCADE
);

CREATE TABLE schedule_versions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    product_plan_id uuid NOT NULL,
    product_profile_version_id uuid NOT NULL,
    business_version integer NOT NULL CHECK (business_version > 0),
    version_state text NOT NULL DEFAULT 'active'
        CHECK (version_state IN ('active', 'cancelled')),
    effective_from date NOT NULL,
    effective_to date,
    timezone_version_id uuid NOT NULL,
    iana_timezone text NOT NULL CHECK (char_length(iana_timezone) BETWEEN 1 AND 120),
    timezone_ruleset text NOT NULL CHECK (char_length(timezone_ruleset) BETWEEN 1 AND 80),
    plan_start_date date NOT NULL,
    weekdays smallint[] NOT NULL,
    day_cycle_enabled boolean NOT NULL,
    day_cycle_days integer NOT NULL CHECK (day_cycle_days BETWEEN 1 AND 365),
    day_take_days integer NOT NULL CHECK (day_take_days BETWEEN 1 AND day_cycle_days),
    day_anchor_date date NOT NULL,
    long_cycle_enabled boolean NOT NULL,
    long_take_weeks integer NOT NULL CHECK (long_take_weeks BETWEEN 1 AND 52),
    long_rest_weeks integer NOT NULL CHECK (long_rest_weeks BETWEEN 0 AND 52),
    long_anchor_date date NOT NULL,
    source text NOT NULL
        CHECK (source IN ('legacy_migration', 'initial', 'capture_confirmed', 'user_edit', 'resumed_copy')),
    history_completeness text NOT NULL
        CHECK (history_completeness IN ('complete', 'legacy_current_only')),
    legacy_schedule_version integer,
    source_updated_at timestamptz NOT NULL,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL,
    CHECK (cardinality(weekdays) BETWEEN 1 AND 7),
    CHECK (weekdays <@ ARRAY[0,1,2,3,4,5,6]::smallint[]),
    CHECK (
        (version_state = 'active' AND (effective_to IS NULL OR effective_to > effective_from))
        OR (version_state = 'cancelled' AND effective_to IS NOT NULL AND effective_to >= effective_from)
    ),
    UNIQUE (product_plan_id, business_version),
    UNIQUE (id, product_plan_id, product_id, user_id, workspace_id),
    CONSTRAINT schedule_versions_plan_fk
        FOREIGN KEY (product_plan_id, product_id, user_id, workspace_id)
        REFERENCES product_plans (id, product_id, user_id, workspace_id)
        ON DELETE CASCADE,
    CONSTRAINT schedule_versions_timezone_fk
        FOREIGN KEY (timezone_version_id, user_id, workspace_id)
        REFERENCES workspace_timezone_versions (id, user_id, workspace_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT schedule_versions_product_profile_fk
        FOREIGN KEY (product_profile_version_id, product_id, user_id, workspace_id)
        REFERENCES product_profile_versions (id, product_id, user_id, workspace_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX schedule_versions_effective_idx
    ON schedule_versions (product_plan_id, effective_from, effective_to)
    WHERE version_state = 'active';

CREATE TABLE dose_slots (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    product_plan_id uuid NOT NULL,
    schedule_version_id uuid NOT NULL,
    slot_key text NOT NULL CHECK (char_length(slot_key) BETWEEN 1 AND 64),
    sort_order smallint NOT NULL CHECK (sort_order BETWEEN 0 AND 7),
    local_time time NOT NULL,
    quantity numeric(20,6) NOT NULL CHECK (quantity > 0),
    meal_relation text NOT NULL DEFAULT 'unspecified'
        CHECK (meal_relation IN ('unspecified', 'with_meal', 'before_meal', 'after_meal', 'empty_stomach')),
    label text NOT NULL DEFAULT '' CHECK (char_length(label) <= 80),
    created_at timestamptz NOT NULL,
    UNIQUE (schedule_version_id, slot_key),
    UNIQUE (schedule_version_id, local_time),
    UNIQUE (schedule_version_id, sort_order),
    UNIQUE (id, schedule_version_id, product_plan_id, product_id, user_id, workspace_id),
    CONSTRAINT dose_slots_schedule_fk
        FOREIGN KEY (schedule_version_id, product_plan_id, product_id, user_id, workspace_id)
        REFERENCES schedule_versions (id, product_plan_id, product_id, user_id, workspace_id)
        ON DELETE CASCADE
);

CREATE TABLE plan_state_intervals (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    product_plan_id uuid NOT NULL,
    state text NOT NULL CHECK (state IN ('active', 'paused')),
    reason text NOT NULL
        CHECK (reason IN ('legacy_status', 'archived', 'user_pause', 'user_resume', 'initial')),
    source text NOT NULL CHECK (source IN ('legacy_migration', 'application')),
    history_completeness text NOT NULL
        CHECK (history_completeness IN ('complete', 'legacy_current_only')),
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    created_at timestamptz NOT NULL,
    CHECK (effective_to IS NULL OR effective_to > effective_from),
    UNIQUE (product_plan_id, effective_from),
    UNIQUE (id, product_plan_id, product_id, user_id, workspace_id),
    CONSTRAINT plan_state_intervals_plan_fk
        FOREIGN KEY (product_plan_id, product_id, user_id, workspace_id)
        REFERENCES product_plans (id, product_id, user_id, workspace_id)
        ON DELETE CASCADE
);

CREATE INDEX plan_state_intervals_effective_idx
    ON plan_state_intervals (product_plan_id, effective_from, effective_to);

-- +goose StatementBegin
CREATE FUNCTION r1_scheduled_occurrence_id(
    schedule_id uuid,
    occurrence_date date,
    slot_id uuid
) RETURNS uuid
LANGUAGE sql IMMUTABLE STRICT AS $$
    SELECT (
        substr(value,1,8) || '-' || substr(value,9,4) || '-5' || substr(value,14,3) || '-' ||
        substr('89ab', 1 + (get_byte(decode(substr(value,17,2),'hex'),0) % 4), 1) ||
        substr(value,18,3) || '-' || substr(value,21,12)
    )::uuid
    FROM (
        SELECT encode(
            sha256(convert_to(schedule_id::text || '|' || occurrence_date::text || '|' || slot_id::text, 'UTF8')),
            'hex'
        ) AS value
    ) hashed;
$$;
-- +goose StatementEnd

CREATE TABLE scheduled_occurrences (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    product_plan_id uuid NOT NULL,
    schedule_version_id uuid NOT NULL,
    dose_slot_id uuid NOT NULL,
    timezone_version_id uuid NOT NULL,
    local_date date NOT NULL,
    local_time time NOT NULL,
    scheduled_at timestamptz NOT NULL,
    iana_timezone text NOT NULL CHECK (char_length(iana_timezone) BETWEEN 1 AND 120),
    timezone_ruleset text NOT NULL CHECK (char_length(timezone_ruleset) BETWEEN 1 AND 80),
    utc_offset_minutes smallint NOT NULL CHECK (utc_offset_minutes BETWEEN -900 AND 900),
    time_resolution text NOT NULL
        CHECK (time_resolution IN ('exact', 'dst_gap_shifted', 'dst_fold_earlier', 'dst_fold_later')),
    planned_quantity numeric(20,6) NOT NULL CHECK (planned_quantity > 0),
    unit_snapshot text NOT NULL CHECK (char_length(unit_snapshot) BETWEEN 1 AND 40),
    meal_relation_snapshot text NOT NULL
        CHECK (meal_relation_snapshot IN ('unspecified', 'with_meal', 'before_meal', 'after_meal', 'empty_stomach')),
    slot_label_snapshot text NOT NULL DEFAULT '' CHECK (char_length(slot_label_snapshot) <= 80),
    materialized_at timestamptz NOT NULL,
    CHECK (id = r1_scheduled_occurrence_id(schedule_version_id, local_date, dose_slot_id)),
    UNIQUE (workspace_id, product_id, schedule_version_id, local_date, dose_slot_id),
    CONSTRAINT scheduled_occurrences_schedule_fk
        FOREIGN KEY (schedule_version_id, product_plan_id, product_id, user_id, workspace_id)
        REFERENCES schedule_versions (id, product_plan_id, product_id, user_id, workspace_id)
        ON DELETE CASCADE,
    CONSTRAINT scheduled_occurrences_slot_fk
        FOREIGN KEY (dose_slot_id, schedule_version_id, product_plan_id, product_id, user_id, workspace_id)
        REFERENCES dose_slots (id, schedule_version_id, product_plan_id, product_id, user_id, workspace_id)
        ON DELETE CASCADE,
    CONSTRAINT scheduled_occurrences_timezone_fk
        FOREIGN KEY (timezone_version_id, user_id, workspace_id)
        REFERENCES workspace_timezone_versions (id, user_id, workspace_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX scheduled_occurrences_day_idx
    ON scheduled_occurrences (user_id, workspace_id, local_date, local_time, product_id);

CREATE TABLE r1_product_plan_backfill_state (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    cycle_id uuid,
    cycle_state text NOT NULL DEFAULT 'idle' CHECK (cycle_state IN ('idle', 'running', 'paused', 'failed')),
    cursor_product_id uuid,
    attempt_count bigint NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    processed_count bigint NOT NULL DEFAULT 0 CHECK (processed_count >= 0),
    mapped_count bigint NOT NULL DEFAULT 0 CHECK (mapped_count >= 0),
    unchanged_count bigint NOT NULL DEFAULT 0 CHECK (unchanged_count >= 0),
    appended_count bigint NOT NULL DEFAULT 0 CHECK (appended_count >= 0),
    quarantined_count bigint NOT NULL DEFAULT 0 CHECK (quarantined_count >= 0),
    source_count bigint NOT NULL DEFAULT 0 CHECK (source_count >= 0),
    pause_requested boolean NOT NULL DEFAULT false,
    source_snapshot_hash char(32),
    last_error text,
    cycle_started_at timestamptz,
    last_progress_at timestamptz,
    last_completed_at timestamptz,
    updated_at timestamptz NOT NULL,
    CONSTRAINT r1_product_plan_backfill_state_running_check
        CHECK ((cycle_state IN ('running','paused','failed') AND cycle_id IS NOT NULL AND cycle_started_at IS NOT NULL)
            OR (cycle_state = 'idle' AND cursor_product_id IS NULL))
);

INSERT INTO r1_product_plan_backfill_state (singleton, updated_at)
VALUES (true, CURRENT_TIMESTAMP);

-- +goose StatementBegin
CREATE FUNCTION r1_validate_schedule_version() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.product_plan_id::text, 2));
    IF cardinality(NEW.weekdays) IS DISTINCT FROM (
        SELECT count(DISTINCT value) FROM unnest(NEW.weekdays) AS value
    ) THEN
        RAISE EXCEPTION 'schedule weekdays must be unique'
            USING ERRCODE = '23514';
    END IF;
    IF TG_OP = 'UPDATE' THEN
        IF (to_jsonb(NEW) - ARRAY['effective_to','version_state'])
                IS DISTINCT FROM (to_jsonb(OLD) - ARRAY['effective_to','version_state'])
           OR OLD.effective_to IS NOT NULL
           OR (NEW.version_state = 'active' AND NEW.effective_to IS NULL)
           OR (OLD.version_state = 'cancelled') THEN
            RAISE EXCEPTION 'schedule versions are immutable after activation'
                USING ERRCODE = '23514';
        END IF;
    END IF;
    IF NEW.version_state = 'active' AND EXISTS (
        SELECT 1
        FROM schedule_versions existing
        WHERE existing.product_plan_id = NEW.product_plan_id
          AND existing.id <> NEW.id
          AND existing.version_state = 'active'
          AND daterange(existing.effective_from, existing.effective_to, '[)')
              && daterange(NEW.effective_from, NEW.effective_to, '[)')
    ) THEN
        RAISE EXCEPTION 'schedule version effective intervals overlap'
            USING ERRCODE = '23P01';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_validate_dose_slot() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    parent_plan_id uuid;
    parent_effective_to date;
    parent_state text;
BEGIN
    IF TG_OP = 'DELETE' AND pg_trigger_depth() > 1 THEN
        RETURN OLD;
    END IF;
    IF TG_OP IN ('UPDATE', 'DELETE') THEN
        RAISE EXCEPTION 'dose slots are immutable after activation'
            USING ERRCODE = '23514';
    END IF;
    SELECT product_plan_id, effective_to, version_state
    INTO parent_plan_id, parent_effective_to, parent_state
    FROM schedule_versions
    WHERE id = NEW.schedule_version_id;
    IF parent_plan_id IS NULL THEN
        RETURN NEW;
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(parent_plan_id::text, 2));
    IF parent_effective_to IS NOT NULL
       OR parent_state = 'cancelled'
       OR EXISTS (
           SELECT 1 FROM product_plans
           WHERE id = parent_plan_id
             AND current_schedule_version_id = NEW.schedule_version_id
       ) THEN
        RAISE EXCEPTION 'dose slots cannot be appended after schedule activation'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_validate_plan_state_interval() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.product_plan_id::text, 3));
    IF TG_OP = 'UPDATE' THEN
        IF (to_jsonb(NEW) - 'effective_to') IS DISTINCT FROM (to_jsonb(OLD) - 'effective_to')
           OR OLD.effective_to IS NOT NULL
           OR NEW.effective_to IS NULL THEN
            RAISE EXCEPTION 'plan state intervals are immutable after activation'
                USING ERRCODE = '23514';
        END IF;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM plan_state_intervals existing
        WHERE existing.product_plan_id = NEW.product_plan_id
          AND existing.id <> NEW.id
          AND tstzrange(existing.effective_from, existing.effective_to, '[)')
              && tstzrange(NEW.effective_from, NEW.effective_to, '[)')
    ) THEN
        RAISE EXCEPTION 'plan state intervals overlap'
            USING ERRCODE = '23P01';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_block_plan_history_delete() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF pg_trigger_depth() > 1 THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'activated plan history is immutable'
        USING ERRCODE = '23514';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER schedule_versions_validate
    BEFORE INSERT OR UPDATE ON schedule_versions
    FOR EACH ROW EXECUTE FUNCTION r1_validate_schedule_version();
CREATE TRIGGER schedule_versions_block_delete
    BEFORE DELETE ON schedule_versions
    FOR EACH ROW EXECUTE FUNCTION r1_block_plan_history_delete();
CREATE TRIGGER dose_slots_validate
    BEFORE INSERT OR UPDATE OR DELETE ON dose_slots
    FOR EACH ROW EXECUTE FUNCTION r1_validate_dose_slot();
CREATE TRIGGER plan_state_intervals_validate
    BEFORE INSERT OR UPDATE ON plan_state_intervals
    FOR EACH ROW EXECUTE FUNCTION r1_validate_plan_state_interval();
CREATE TRIGGER plan_state_intervals_block_delete
    BEFORE DELETE ON plan_state_intervals
    FOR EACH ROW EXECUTE FUNCTION r1_block_plan_history_delete();

-- +goose StatementBegin
CREATE FUNCTION r1_backfill_product_plans_batch(batch_size integer DEFAULT 100) RETURNS boolean
LANGUAGE plpgsql AS $$
DECLARE
    state_cycle text;
    state_cursor uuid;
    state_pause_requested boolean;
    product_row record;
    history_row record;
    processed_in_batch integer := 0;
    last_product_id uuid;
    plan_id uuid;
    current_schedule_id uuid;
    current_schedule_effective_from date;
    current_schedule_source text;
    next_business_version integer;
    target_timezone_version_id uuid;
    timezone_name text;
    schedule_changed boolean;
    normalized_weekdays smallint[];
    reminder_values text[];
    reminder_value text;
    reminder_count integer;
    reminder_index integer;
    expected_meal_relation text;
    new_schedule_id uuid;
    new_effective_from date;
    latest_effective_date date;
    previous_schedule_id uuid;
    previous_effective_from date;
    history_seen boolean;
    last_day_cycle_enabled boolean;
    last_day_cycle_days integer;
    last_day_take_days integer;
    last_day_anchor_date date;
    open_state_id uuid;
    open_state text;
    open_state_source text;
    expected_state text;
    expected_reason text;
    local_mapped bigint := 0;
    local_unchanged bigint := 0;
    local_appended bigint := 0;
    local_quarantined bigint := 0;
    source_total bigint;
    snapshot_hash char(32);
BEGIN
    IF batch_size < 1 OR batch_size > 10000 THEN
        RAISE EXCEPTION 'product plan backfill batch size must be between 1 and 10000'
            USING ERRCODE = '22023';
    END IF;

    PERFORM pg_advisory_xact_lock(hashtextextended('r1_product_plan_backfill', 0));
    SELECT cycle_state, cursor_product_id, pause_requested
    INTO state_cycle, state_cursor, state_pause_requested
    FROM r1_product_plan_backfill_state
    WHERE singleton
    FOR UPDATE;

    IF state_cycle = 'idle' THEN
        SELECT count(*) INTO source_total
        FROM products
        WHERE product_type = 'supplement';
        state_cursor := NULL;
        UPDATE r1_product_plan_backfill_state
        SET cycle_id = gen_random_uuid(), cycle_state = 'running', cursor_product_id = NULL,
            processed_count = 0, mapped_count = 0, unchanged_count = 0,
            appended_count = 0, quarantined_count = 0, source_count = source_total,
            source_snapshot_hash = NULL, last_error = NULL,
            cycle_started_at = CURRENT_TIMESTAMP, last_progress_at = CURRENT_TIMESTAMP,
            updated_at = CURRENT_TIMESTAMP
        WHERE singleton;
        state_cycle := 'running';
    ELSIF state_cycle = 'failed' THEN
        RAISE EXCEPTION 'product plan backfill is failed; clear the recorded error before resuming'
            USING ERRCODE = '55000';
    ELSIF state_cycle = 'paused' AND NOT state_pause_requested THEN
        UPDATE r1_product_plan_backfill_state
        SET cycle_state = 'running', updated_at = CURRENT_TIMESTAMP
        WHERE singleton;
        state_cycle := 'running';
    END IF;

    UPDATE r1_product_plan_backfill_state
    SET attempt_count = attempt_count + 1, updated_at = CURRENT_TIMESTAMP
    WHERE singleton;

    IF state_pause_requested THEN
        UPDATE r1_product_plan_backfill_state
        SET cycle_state = 'paused', last_progress_at = CURRENT_TIMESTAMP,
            updated_at = CURRENT_TIMESTAMP
        WHERE singleton;
        RETURN false;
    END IF;

    FOR product_row IN
        SELECT p.*, ps.version AS legacy_schedule_version, ps.start_date, ps.weekdays,
               ps.day_cycle_enabled, ps.day_cycle_days, ps.day_take_days, ps.day_anchor_date,
               ps.long_cycle_enabled, ps.long_take_weeks, ps.long_rest_weeks, ps.long_start_date,
               ps.reminder_times, ps.updated_at AS schedule_updated_at
        FROM products p
        LEFT JOIN product_schedules ps
          ON ps.product_id = p.id AND ps.user_id = p.user_id AND ps.workspace_id = p.workspace_id
        WHERE p.product_type = 'supplement'
          AND (state_cursor IS NULL OR p.id > state_cursor)
        ORDER BY p.id
        LIMIT batch_size
    LOOP
        processed_in_batch := processed_in_batch + 1;
        last_product_id := product_row.id;

        IF product_row.start_date IS NULL THEN
            INSERT INTO migration_quarantines (
                source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
            ) VALUES (
                'products',product_row.id,
                CASE WHEN EXISTS (
                    SELECT 1 FROM product_schedules source_schedule
                    WHERE source_schedule.product_id = product_row.id
                      AND (source_schedule.user_id <> product_row.user_id
                           OR source_schedule.workspace_id <> product_row.workspace_id)
                ) THEN 'schedule_tenant_mismatch' ELSE 'schedule_missing' END,
                product_row.user_id,product_row.workspace_id,'open',CURRENT_TIMESTAMP
            ) ON CONFLICT (source_table,source_id,reason_code) DO NOTHING;
            local_quarantined := local_quarantined + 1;
            CONTINUE;
        END IF;

        IF product_row.current_product_profile_version_id IS NULL THEN
            INSERT INTO migration_quarantines (
                source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
            ) VALUES (
                'products',product_row.id,'product_profile_missing',
                product_row.user_id,product_row.workspace_id,'open',CURRENT_TIMESTAMP
            ) ON CONFLICT (source_table,source_id,reason_code) DO NOTHING;
            local_quarantined := local_quarantined + 1;
            CONTINUE;
        END IF;

        SELECT w.current_timezone_version_id, tz.iana_timezone
        INTO target_timezone_version_id, timezone_name
        FROM workspaces w
        JOIN workspace_timezone_versions tz
          ON tz.id = w.current_timezone_version_id
         AND tz.user_id = w.owner_user_id
         AND tz.workspace_id = w.id
        WHERE w.id = product_row.workspace_id
          AND w.owner_user_id = product_row.user_id;
        IF target_timezone_version_id IS NULL THEN
            INSERT INTO migration_quarantines (
                source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
            ) VALUES (
                'products',product_row.id,'workspace_timezone_missing',
                product_row.user_id,product_row.workspace_id,'open',CURRENT_TIMESTAMP
            ) ON CONFLICT (source_table,source_id,reason_code) DO NOTHING;
            local_quarantined := local_quarantined + 1;
            CONTINUE;
        END IF;

        IF EXISTS (
            SELECT 1 FROM day_cycle_versions history_source
            WHERE history_source.product_id = product_row.id
              AND (history_source.user_id <> product_row.user_id
                   OR history_source.workspace_id <> product_row.workspace_id)
        ) THEN
            INSERT INTO migration_quarantines (
                source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
            ) VALUES (
                'products',product_row.id,'day_cycle_tenant_mismatch',
                product_row.user_id,product_row.workspace_id,'open',CURRENT_TIMESTAMP
            ) ON CONFLICT (source_table,source_id,reason_code) DO NOTHING;
            local_quarantined := local_quarantined + 1;
            CONTINUE;
        END IF;

        SELECT ARRAY(
            SELECT DISTINCT value
            FROM unnest(product_row.weekdays) AS value
            ORDER BY value
        ) INTO normalized_weekdays;
        IF cardinality(normalized_weekdays) IS DISTINCT FROM cardinality(product_row.weekdays)
           OR cardinality(normalized_weekdays) NOT BETWEEN 1 AND 7
           OR NOT (normalized_weekdays <@ ARRAY[0,1,2,3,4,5,6]::smallint[])
           OR product_row.day_cycle_days NOT BETWEEN 1 AND 365
           OR product_row.day_take_days NOT BETWEEN 1 AND product_row.day_cycle_days
           OR product_row.long_take_weeks NOT BETWEEN 1 AND 52
           OR product_row.long_rest_weeks NOT BETWEEN 0 AND 52
           OR EXISTS (
               SELECT 1 FROM day_cycle_versions history_source
               WHERE history_source.product_id = product_row.id
                 AND history_source.user_id = product_row.user_id
                 AND history_source.workspace_id = product_row.workspace_id
                 AND (history_source.cycle_days NOT BETWEEN 1 AND 365
                      OR history_source.take_days NOT BETWEEN 1 AND history_source.cycle_days)
           ) THEN
            INSERT INTO migration_quarantines (
                source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
            ) VALUES (
                'product_schedules',product_row.id,'schedule_rule_out_of_range',
                product_row.user_id,product_row.workspace_id,'open',CURRENT_TIMESTAMP
            ) ON CONFLICT (source_table,source_id,reason_code) DO NOTHING;
            local_quarantined := local_quarantined + 1;
            CONTINUE;
        END IF;

        IF product_row.reminder_times IS NULL OR EXISTS (
            SELECT 1 FROM unnest(product_row.reminder_times) AS value
            WHERE value !~ '^(?:[01][0-9]|2[0-3]):[0-5][0-9]$'
        ) THEN
            INSERT INTO migration_quarantines (
                source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
            ) VALUES (
                'product_schedules',product_row.id,'schedule_invalid_local_time',
                product_row.user_id,product_row.workspace_id,'open',CURRENT_TIMESTAMP
            ) ON CONFLICT (source_table,source_id,reason_code) DO NOTHING;
            local_quarantined := local_quarantined + 1;
            CONTINUE;
        END IF;

        SELECT ARRAY(
            SELECT DISTINCT value
            FROM unnest(product_row.reminder_times) AS value
            ORDER BY value
        ) INTO reminder_values;
        IF cardinality(reminder_values) IS DISTINCT FROM cardinality(product_row.reminder_times) THEN
            INSERT INTO migration_quarantines (
                source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
            ) VALUES (
                'product_schedules',product_row.id,'schedule_duplicate_local_time',
                product_row.user_id,product_row.workspace_id,'open',CURRENT_TIMESTAMP
            ) ON CONFLICT (source_table,source_id,reason_code) DO NOTHING;
            local_quarantined := local_quarantined + 1;
            CONTINUE;
        END IF;
        reminder_count := cardinality(reminder_values);
        IF reminder_count IS NULL OR reminder_count = 0
           OR reminder_count <> product_row.dose_times_per_day THEN
            INSERT INTO migration_quarantines (
                source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
            ) VALUES (
                'product_schedules',product_row.id,'schedule_slot_count_mismatch',
                product_row.user_id,product_row.workspace_id,'open',CURRENT_TIMESTAMP
            ) ON CONFLICT (source_table,source_id,reason_code) DO NOTHING;
            local_quarantined := local_quarantined + 1;
            CONTINUE;
        END IF;

        UPDATE migration_quarantines
        SET state = 'resolved', resolution_code = 'source_corrected', resolved_at = CURRENT_TIMESTAMP
        WHERE source_id = product_row.id
          AND state = 'open'
          AND reason_code IN (
              'schedule_missing','schedule_tenant_mismatch','product_profile_missing',
              'workspace_timezone_missing','day_cycle_tenant_mismatch','schedule_rule_out_of_range',
              'schedule_invalid_local_time','schedule_duplicate_local_time','schedule_slot_count_mismatch'
          );

        expected_meal_relation := CASE WHEN product_row.with_food IS TRUE THEN 'with_meal' ELSE 'unspecified' END;
        SELECT id, current_schedule_version_id
        INTO plan_id, current_schedule_id
        FROM product_plans
        WHERE product_id = product_row.id
          AND user_id = product_row.user_id
          AND workspace_id = product_row.workspace_id
        FOR UPDATE;
        IF plan_id IS NULL THEN
            plan_id := gen_random_uuid();
            INSERT INTO product_plans (
                id,user_id,workspace_id,product_id,aggregate_version,created_at,updated_at
            ) VALUES (
                plan_id,product_row.user_id,product_row.workspace_id,product_row.id,1,
                CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
            );
            current_schedule_id := NULL;
        END IF;

        schedule_changed := current_schedule_id IS NULL;
        IF current_schedule_id IS NOT NULL THEN
            SELECT sv.effective_from, sv.source
            INTO current_schedule_effective_from, current_schedule_source
            FROM schedule_versions sv
            WHERE sv.id = current_schedule_id
              AND sv.product_plan_id = plan_id;
            schedule_changed := NOT EXISTS (
                SELECT 1
                FROM schedule_versions sv
                WHERE sv.id = current_schedule_id
                  AND sv.version_state = 'active'
                  AND sv.timezone_version_id = target_timezone_version_id
                  AND sv.iana_timezone = timezone_name
                  AND sv.timezone_ruleset = 'legacy_unversioned'
                  AND sv.plan_start_date = product_row.start_date
                  AND sv.weekdays = normalized_weekdays
                  AND sv.day_cycle_enabled = product_row.day_cycle_enabled
                  AND sv.day_cycle_days = product_row.day_cycle_days
                  AND sv.day_take_days = product_row.day_take_days
                  AND sv.day_anchor_date = product_row.day_anchor_date
                  AND sv.long_cycle_enabled = product_row.long_cycle_enabled
                  AND sv.long_take_weeks = product_row.long_take_weeks
                  AND sv.long_rest_weeks = product_row.long_rest_weeks
                  AND sv.long_anchor_date = product_row.long_start_date
                  AND (SELECT array_agg(to_char(ds.local_time, 'HH24:MI') ORDER BY ds.local_time)
                       FROM dose_slots ds WHERE ds.schedule_version_id = sv.id) = reminder_values
                  AND (SELECT count(*) FROM dose_slots ds
                       WHERE ds.schedule_version_id = sv.id) = reminder_count
                  AND NOT EXISTS (
                      SELECT 1 FROM dose_slots ds
                      WHERE ds.schedule_version_id = sv.id
                        AND (ds.quantity <> product_row.dose_quantity
                             OR ds.meal_relation <> expected_meal_relation)
                  )
                  AND EXISTS (
                      SELECT 1
                      FROM product_profile_versions bound_profile
                      JOIN product_profile_versions current_profile
                        ON current_profile.id = product_row.current_product_profile_version_id
                       AND current_profile.product_id = product_row.id
                       AND current_profile.user_id = product_row.user_id
                       AND current_profile.workspace_id = product_row.workspace_id
                      WHERE bound_profile.id = sv.product_profile_version_id
                        AND bound_profile.management_unit = current_profile.management_unit
                  )
            );
        END IF;

        IF current_schedule_id IS NULL THEN
            next_business_version := 1;
            previous_schedule_id := NULL;
            previous_effective_from := NULL;
            history_seen := false;
            FOR history_row IN
                SELECT DISTINCT ON (GREATEST(product_row.start_date, source_history.effective_date))
                       GREATEST(product_row.start_date, source_history.effective_date) AS boundary_date,
                       source_history.enabled, source_history.cycle_days, source_history.take_days,
                       source_history.anchor_date, source_history.created_at
                FROM day_cycle_versions source_history
                WHERE source_history.product_id = product_row.id
                  AND source_history.user_id = product_row.user_id
                  AND source_history.workspace_id = product_row.workspace_id
                ORDER BY GREATEST(product_row.start_date, source_history.effective_date),
                         source_history.effective_date DESC, source_history.created_at DESC, source_history.id DESC
            LOOP
                history_seen := true;
                new_effective_from := history_row.boundary_date;
                IF previous_schedule_id IS NOT NULL THEN
                    UPDATE schedule_versions SET effective_to = new_effective_from
                    WHERE id = previous_schedule_id AND version_state = 'active' AND effective_to IS NULL;
                END IF;
                new_schedule_id := gen_random_uuid();
                INSERT INTO schedule_versions (
                    id,user_id,workspace_id,product_id,product_plan_id,product_profile_version_id,
                    business_version,version_state,effective_from,timezone_version_id,iana_timezone,
                    timezone_ruleset,plan_start_date,weekdays,day_cycle_enabled,day_cycle_days,
                    day_take_days,day_anchor_date,long_cycle_enabled,long_take_weeks,long_rest_weeks,
                    long_anchor_date,source,history_completeness,legacy_schedule_version,
                    source_updated_at,created_at
                ) VALUES (
                    new_schedule_id,product_row.user_id,product_row.workspace_id,product_row.id,plan_id,
                    product_row.current_product_profile_version_id,next_business_version,'active',
                    new_effective_from,target_timezone_version_id,timezone_name,'legacy_unversioned',
                    product_row.start_date,normalized_weekdays,history_row.enabled,history_row.cycle_days,
                    history_row.take_days,history_row.anchor_date,product_row.long_cycle_enabled,
                    product_row.long_take_weeks,product_row.long_rest_weeks,product_row.long_start_date,
                    'legacy_migration','legacy_current_only',product_row.legacy_schedule_version,
                    history_row.created_at,CURRENT_TIMESTAMP
                );
                reminder_index := 0;
                FOREACH reminder_value IN ARRAY reminder_values LOOP
                    INSERT INTO dose_slots (
                        id,user_id,workspace_id,product_id,product_plan_id,schedule_version_id,
                        slot_key,sort_order,local_time,quantity,meal_relation,label,created_at
                    ) VALUES (
                        gen_random_uuid(),product_row.user_id,product_row.workspace_id,product_row.id,
                        plan_id,new_schedule_id,'legacy-' || (reminder_index + 1),reminder_index,
                        reminder_value::time,product_row.dose_quantity,expected_meal_relation,'',CURRENT_TIMESTAMP
                    );
                    reminder_index := reminder_index + 1;
                END LOOP;
                previous_schedule_id := new_schedule_id;
                previous_effective_from := new_effective_from;
                last_day_cycle_enabled := history_row.enabled;
                last_day_cycle_days := history_row.cycle_days;
                last_day_take_days := history_row.take_days;
                last_day_anchor_date := history_row.anchor_date;
                next_business_version := next_business_version + 1;
                local_appended := local_appended + 1;
            END LOOP;

            IF NOT history_seen
               OR last_day_cycle_enabled IS DISTINCT FROM product_row.day_cycle_enabled
               OR last_day_cycle_days IS DISTINCT FROM product_row.day_cycle_days
               OR last_day_take_days IS DISTINCT FROM product_row.day_take_days
               OR last_day_anchor_date IS DISTINCT FROM product_row.day_anchor_date THEN
                new_effective_from := CASE
                    WHEN NOT history_seen THEN product_row.start_date
                    ELSE GREATEST(product_row.start_date,
                                  (product_row.schedule_updated_at AT TIME ZONE 'UTC')::date,
                                  previous_effective_from)
                END;
                IF previous_schedule_id IS NOT NULL AND new_effective_from = previous_effective_from THEN
                    UPDATE schedule_versions
                    SET version_state = 'cancelled', effective_to = effective_from
                    WHERE id = previous_schedule_id AND version_state = 'active' AND effective_to IS NULL;
                ELSIF previous_schedule_id IS NOT NULL THEN
                    UPDATE schedule_versions SET effective_to = new_effective_from
                    WHERE id = previous_schedule_id AND version_state = 'active' AND effective_to IS NULL;
                END IF;
                new_schedule_id := gen_random_uuid();
                INSERT INTO schedule_versions (
                    id,user_id,workspace_id,product_id,product_plan_id,product_profile_version_id,
                    business_version,version_state,effective_from,timezone_version_id,iana_timezone,
                    timezone_ruleset,plan_start_date,weekdays,day_cycle_enabled,day_cycle_days,
                    day_take_days,day_anchor_date,long_cycle_enabled,long_take_weeks,long_rest_weeks,
                    long_anchor_date,source,history_completeness,legacy_schedule_version,
                    source_updated_at,created_at
                ) VALUES (
                    new_schedule_id,product_row.user_id,product_row.workspace_id,product_row.id,plan_id,
                    product_row.current_product_profile_version_id,next_business_version,'active',
                    new_effective_from,target_timezone_version_id,timezone_name,'legacy_unversioned',
                    product_row.start_date,normalized_weekdays,product_row.day_cycle_enabled,
                    product_row.day_cycle_days,product_row.day_take_days,product_row.day_anchor_date,
                    product_row.long_cycle_enabled,product_row.long_take_weeks,product_row.long_rest_weeks,
                    product_row.long_start_date,'legacy_migration','legacy_current_only',
                    product_row.legacy_schedule_version,product_row.schedule_updated_at,CURRENT_TIMESTAMP
                );
                reminder_index := 0;
                FOREACH reminder_value IN ARRAY reminder_values LOOP
                    INSERT INTO dose_slots (
                        id,user_id,workspace_id,product_id,product_plan_id,schedule_version_id,
                        slot_key,sort_order,local_time,quantity,meal_relation,label,created_at
                    ) VALUES (
                        gen_random_uuid(),product_row.user_id,product_row.workspace_id,product_row.id,
                        plan_id,new_schedule_id,'legacy-' || (reminder_index + 1),reminder_index,
                        reminder_value::time,product_row.dose_quantity,expected_meal_relation,'',CURRENT_TIMESTAMP
                    );
                    reminder_index := reminder_index + 1;
                END LOOP;
                previous_schedule_id := new_schedule_id;
                local_appended := local_appended + 1;
            END IF;
            current_schedule_id := previous_schedule_id;
            UPDATE product_plans
            SET current_schedule_version_id = current_schedule_id, updated_at = CURRENT_TIMESTAMP
            WHERE id = plan_id;
        ELSIF schedule_changed THEN
            IF current_schedule_source <> 'legacy_migration' THEN
                INSERT INTO migration_quarantines (
                    source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
                ) VALUES (
                    'product_schedules',product_row.id,'target_schedule_conflict',
                    product_row.user_id,product_row.workspace_id,'open',CURRENT_TIMESTAMP
                ) ON CONFLICT (source_table,source_id,reason_code) DO NOTHING;
                local_quarantined := local_quarantined + 1;
                CONTINUE;
            END IF;
            SELECT max(dcv.effective_date) INTO latest_effective_date
            FROM day_cycle_versions dcv
            WHERE dcv.product_id = product_row.id
              AND dcv.user_id = product_row.user_id
              AND dcv.workspace_id = product_row.workspace_id;
            new_effective_from := GREATEST(product_row.start_date, CURRENT_DATE,
                                           COALESCE(latest_effective_date, CURRENT_DATE));
            IF current_schedule_effective_from >= new_effective_from THEN
                new_effective_from := current_schedule_effective_from;
                UPDATE schedule_versions
                SET version_state = 'cancelled', effective_to = effective_from
                WHERE id = current_schedule_id AND version_state = 'active';
            ELSE
                UPDATE schedule_versions SET effective_to = new_effective_from
                WHERE id = current_schedule_id AND version_state = 'active' AND effective_to IS NULL;
            END IF;
            SELECT COALESCE(max(business_version),0)+1 INTO next_business_version
            FROM schedule_versions WHERE product_plan_id = plan_id;
            new_schedule_id := gen_random_uuid();
            INSERT INTO schedule_versions (
                id,user_id,workspace_id,product_id,product_plan_id,product_profile_version_id,business_version,
                version_state,effective_from,timezone_version_id,iana_timezone,timezone_ruleset,
                plan_start_date,weekdays,day_cycle_enabled,day_cycle_days,day_take_days,
                day_anchor_date,long_cycle_enabled,long_take_weeks,long_rest_weeks,long_anchor_date,
                source,history_completeness,legacy_schedule_version,source_updated_at,created_at
            ) VALUES (
                new_schedule_id,product_row.user_id,product_row.workspace_id,product_row.id,plan_id,
                product_row.current_product_profile_version_id,next_business_version,'active',new_effective_from,
                target_timezone_version_id,timezone_name,'legacy_unversioned',product_row.start_date,
                normalized_weekdays,product_row.day_cycle_enabled,product_row.day_cycle_days,
                product_row.day_take_days,product_row.day_anchor_date,product_row.long_cycle_enabled,
                product_row.long_take_weeks,product_row.long_rest_weeks,product_row.long_start_date,
                'legacy_migration','legacy_current_only',product_row.legacy_schedule_version,
                product_row.schedule_updated_at,CURRENT_TIMESTAMP
            );
            reminder_index := 0;
            FOREACH reminder_value IN ARRAY reminder_values LOOP
                INSERT INTO dose_slots (
                    id,user_id,workspace_id,product_id,product_plan_id,schedule_version_id,
                    slot_key,sort_order,local_time,quantity,meal_relation,label,created_at
                ) VALUES (
                    gen_random_uuid(),product_row.user_id,product_row.workspace_id,product_row.id,
                    plan_id,new_schedule_id,'legacy-' || (reminder_index + 1),reminder_index,
                    reminder_value::time,product_row.dose_quantity,expected_meal_relation,'',CURRENT_TIMESTAMP
                );
                reminder_index := reminder_index + 1;
            END LOOP;
            UPDATE product_plans
            SET current_schedule_version_id = new_schedule_id,
                aggregate_version = aggregate_version + 1,
                updated_at = CURRENT_TIMESTAMP
            WHERE id = plan_id;
            current_schedule_id := new_schedule_id;
            local_appended := local_appended + 1;
        ELSE
            local_unchanged := local_unchanged + 1;
        END IF;

        expected_state := CASE WHEN product_row.status IN ('paused','archived') THEN 'paused' ELSE 'active' END;
        expected_reason := CASE
            WHEN product_row.status = 'archived' THEN 'archived'
            WHEN product_row.status = 'paused' THEN 'legacy_status'
            ELSE 'initial'
        END;
        SELECT id, state, source INTO open_state_id, open_state, open_state_source
        FROM plan_state_intervals
        WHERE product_plan_id = plan_id AND effective_to IS NULL
        ORDER BY effective_from DESC
        LIMIT 1;
        IF open_state_id IS NULL THEN
            INSERT INTO plan_state_intervals (
                id,user_id,workspace_id,product_id,product_plan_id,state,reason,source,
                history_completeness,effective_from,created_at
            ) VALUES (
                gen_random_uuid(),product_row.user_id,product_row.workspace_id,product_row.id,
                plan_id,expected_state,expected_reason,'legacy_migration','legacy_current_only',
                CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
            );
        ELSIF open_state IS DISTINCT FROM expected_state AND open_state_source = 'legacy_migration' THEN
            UPDATE plan_state_intervals
            SET effective_to = GREATEST(CURRENT_TIMESTAMP, effective_from + interval '1 microsecond')
            WHERE id = open_state_id;
            INSERT INTO plan_state_intervals (
                id,user_id,workspace_id,product_id,product_plan_id,state,reason,source,
                history_completeness,effective_from,created_at
            ) SELECT
                gen_random_uuid(),product_row.user_id,product_row.workspace_id,product_row.id,
                plan_id,expected_state,
                CASE WHEN expected_state = 'active' THEN 'user_resume' ELSE expected_reason END,
                'legacy_migration','legacy_current_only',effective_to,CURRENT_TIMESTAMP
            FROM plan_state_intervals WHERE id = open_state_id;
        ELSIF open_state IS DISTINCT FROM expected_state THEN
            INSERT INTO migration_quarantines (
                source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
            ) VALUES (
                'products',product_row.id,'target_plan_state_conflict',
                product_row.user_id,product_row.workspace_id,'open',CURRENT_TIMESTAMP
            ) ON CONFLICT (source_table,source_id,reason_code) DO NOTHING;
            local_quarantined := local_quarantined + 1;
            CONTINUE;
        END IF;
        local_mapped := local_mapped + 1;
    END LOOP;

    IF processed_in_batch = 0 OR NOT EXISTS (
        SELECT 1 FROM products p
        WHERE p.product_type='supplement' AND p.id > last_product_id
    ) THEN
        SELECT md5(COALESCE(string_agg(
            p.id::text || ':' || COALESCE(ps.version::text,'missing') || ':' || p.updated_at::text,
            '|' ORDER BY p.id
        ),''))
        INTO snapshot_hash
        FROM products p
        LEFT JOIN product_schedules ps
          ON ps.product_id=p.id AND ps.user_id=p.user_id AND ps.workspace_id=p.workspace_id
        WHERE p.product_type='supplement';
        UPDATE r1_product_plan_backfill_state
        SET cycle_state='idle',cursor_product_id=NULL,
            processed_count=processed_count+processed_in_batch,
            mapped_count=mapped_count+local_mapped,
            unchanged_count=unchanged_count+local_unchanged,
            appended_count=appended_count+local_appended,
            quarantined_count=quarantined_count+local_quarantined,
            source_snapshot_hash=snapshot_hash,last_progress_at=CURRENT_TIMESTAMP,
            last_completed_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
        WHERE singleton;
        RETURN true;
    END IF;
    UPDATE r1_product_plan_backfill_state
    SET cursor_product_id=last_product_id,
        processed_count=processed_count+processed_in_batch,
        mapped_count=mapped_count+local_mapped,
        unchanged_count=unchanged_count+local_unchanged,
        appended_count=appended_count+local_appended,
        quarantined_count=quarantined_count+local_quarantined,
        last_progress_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
    WHERE singleton;
    RETURN false;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_backfill_product_plans() RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    cycle_complete boolean := false;
BEGIN
    WHILE NOT cycle_complete LOOP
        cycle_complete := r1_backfill_product_plans_batch(100);
    END LOOP;
END;
$$;
-- +goose StatementEnd

SELECT r1_backfill_product_plans();

ALTER TABLE product_plans
    ADD CONSTRAINT product_plans_current_schedule_fk
        FOREIGN KEY (current_schedule_version_id, id, product_id, user_id, workspace_id)
        REFERENCES schedule_versions (id, product_plan_id, product_id, user_id, workspace_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM schedule_versions WHERE source <> 'legacy_migration')
       OR EXISTS (SELECT 1 FROM plan_state_intervals WHERE source <> 'legacy_migration')
       OR EXISTS (SELECT 1 FROM scheduled_occurrences) THEN
        RAISE EXCEPTION 'E3 target writes exist; rollback requires a forward fix'
            USING ERRCODE = '55000';
    END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE product_plans DROP CONSTRAINT IF EXISTS product_plans_current_schedule_fk;
DROP FUNCTION IF EXISTS r1_backfill_product_plans();
DROP FUNCTION IF EXISTS r1_backfill_product_plans_batch(integer);
DROP TABLE IF EXISTS r1_product_plan_backfill_state;
DROP TABLE IF EXISTS scheduled_occurrences;
DROP TABLE IF EXISTS plan_state_intervals;
DROP TABLE IF EXISTS dose_slots;
DROP TABLE IF EXISTS schedule_versions;
DROP TABLE IF EXISTS product_plans;
DROP FUNCTION IF EXISTS r1_scheduled_occurrence_id(uuid,date,uuid);
DROP FUNCTION IF EXISTS r1_block_plan_history_delete();
DROP FUNCTION IF EXISTS r1_validate_plan_state_interval();
DROP FUNCTION IF EXISTS r1_validate_dose_slot();
DROP FUNCTION IF EXISTS r1_validate_schedule_version();
DROP TRIGGER IF EXISTS workspace_timezone_versions_block_delete ON workspace_timezone_versions;
DROP TRIGGER IF EXISTS workspace_timezone_versions_validate ON workspace_timezone_versions;
DROP FUNCTION IF EXISTS r1_block_timezone_history_delete();
DROP FUNCTION IF EXISTS r1_validate_workspace_timezone_version();
ALTER TABLE workspaces DROP CONSTRAINT IF EXISTS workspaces_current_timezone_version_fk;
ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_current_timezone_version_fk
        FOREIGN KEY (current_timezone_version_id)
        REFERENCES workspace_timezone_versions(id)
        ON DELETE RESTRICT;
DROP INDEX IF EXISTS workspace_timezone_versions_tenant_identity_uidx;
