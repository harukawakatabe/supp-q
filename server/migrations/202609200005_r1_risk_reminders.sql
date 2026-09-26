-- +goose Up
CREATE UNIQUE INDEX projection_revisions_tenant_identity_uidx
    ON projection_revisions (id,user_id,workspace_id);

CREATE TABLE inventory_risk_projection_sets (
    projection_revision_id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    expected_batch_count integer NOT NULL CHECK (expected_batch_count >= 0),
    completed_at timestamptz,
    created_at timestamptz NOT NULL,
    UNIQUE (projection_revision_id,user_id,workspace_id,product_id),
    CONSTRAINT inventory_risk_projection_sets_revision_fk
        FOREIGN KEY (projection_revision_id,user_id,workspace_id)
        REFERENCES projection_revisions(id,user_id,workspace_id) ON DELETE CASCADE,
    CONSTRAINT inventory_risk_projection_sets_product_fk
        FOREIGN KEY (product_id,user_id,workspace_id)
        REFERENCES products(id,user_id,workspace_id) ON DELETE CASCADE
);

CREATE TABLE inventory_risk_projections (
    id uuid PRIMARY KEY,
    projection_revision_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    result_scope text NOT NULL CHECK (result_scope IN ('product','batch')),
    batch_id uuid,
    stock_state text CHECK (stock_state IS NULL OR stock_state IN ('in_stock','low','depleted')),
    risk_state text NOT NULL CHECK (risk_state IN (
        'unknown','safe','near_expiry','unfinishable','expired','depleted'
    )),
    calculation_state text CHECK (calculation_state IS NULL OR calculation_state IN (
        'calculated','no_future_plan','paused','beyond_horizon','invalid_plan','failed'
    )),
    on_hand_quantity numeric(20,6) CHECK (on_hand_quantity IS NULL OR on_hand_quantity >= 0),
    auto_allocatable_quantity numeric(20,6)
        CHECK (auto_allocatable_quantity IS NULL OR auto_allocatable_quantity >= 0),
    expired_quantity numeric(20,6) CHECK (expired_quantity IS NULL OR expired_quantity >= 0),
    unknown_expiry_quantity numeric(20,6)
        CHECK (unknown_expiry_quantity IS NULL OR unknown_expiry_quantity >= 0),
    covered_occurrence_count integer CHECK (covered_occurrence_count IS NULL OR covered_occurrence_count >= 0),
    covered_plan_day_count integer CHECK (covered_plan_day_count IS NULL OR covered_plan_day_count >= 0),
    projected_depletion_at timestamptz,
    first_shortfall_occurrence_id uuid,
    first_shortfall_at timestamptz,
    shortfall_quantity numeric(20,6) CHECK (shortfall_quantity IS NULL OR shortfall_quantity > 0),
    expiry_precision text CHECK (expiry_precision IS NULL OR expiry_precision IN ('day','month','year','unknown')),
    expiry_period_start date,
    expiry_period_end date,
    precision_limited boolean NOT NULL DEFAULT false,
    projected_remaining_at_expiry numeric(20,6)
        CHECK (projected_remaining_at_expiry IS NULL OR projected_remaining_at_expiry >= 0),
    risk_episode_id uuid,
    reason_code text NOT NULL DEFAULT '' CHECK (char_length(reason_code) <= 120),
    calculated_at timestamptz NOT NULL,
    CONSTRAINT inventory_risk_projections_scope_check CHECK (
        (result_scope='product' AND batch_id IS NULL AND stock_state IS NOT NULL
            AND calculation_state IS NOT NULL AND expiry_precision IS NULL
            AND risk_state<>'depleted')
        OR
        (result_scope='batch' AND batch_id IS NOT NULL AND stock_state IS NULL
            AND calculation_state IS NULL)
    ),
    CONSTRAINT inventory_risk_projections_expiry_range_check CHECK (
        (expiry_period_start IS NULL AND expiry_period_end IS NULL)
        OR (expiry_period_start IS NOT NULL AND expiry_period_end IS NOT NULL
            AND expiry_period_end >= expiry_period_start)
    ),
    CONSTRAINT inventory_risk_projections_shortfall_check CHECK (
        (first_shortfall_occurrence_id IS NULL AND first_shortfall_at IS NULL AND shortfall_quantity IS NULL)
        OR (first_shortfall_occurrence_id IS NOT NULL AND first_shortfall_at IS NOT NULL
            AND shortfall_quantity IS NOT NULL)
    ),
    CONSTRAINT inventory_risk_projections_set_fk
        FOREIGN KEY (projection_revision_id,user_id,workspace_id,product_id)
        REFERENCES inventory_risk_projection_sets(projection_revision_id,user_id,workspace_id,product_id)
        ON DELETE CASCADE,
    CONSTRAINT inventory_risk_projections_product_fk
        FOREIGN KEY (product_id,user_id,workspace_id)
        REFERENCES products(id,user_id,workspace_id) ON DELETE CASCADE,
    CONSTRAINT inventory_risk_projections_batch_fk
        FOREIGN KEY (batch_id,user_id,workspace_id,product_id)
        REFERENCES inventory_batches(id,user_id,workspace_id,product_id)
        ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT inventory_risk_projections_shortfall_occurrence_fk
        FOREIGN KEY (first_shortfall_occurrence_id,user_id,workspace_id,product_id)
        REFERENCES scheduled_occurrences(id,user_id,workspace_id,product_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED
);

CREATE UNIQUE INDEX inventory_risk_projection_product_uidx
    ON inventory_risk_projections (projection_revision_id)
    WHERE result_scope='product';
CREATE UNIQUE INDEX inventory_risk_projection_batch_uidx
    ON inventory_risk_projections (projection_revision_id,batch_id)
    WHERE result_scope='batch';
CREATE INDEX inventory_risk_projection_active_lookup_idx
    ON inventory_risk_projections (workspace_id,product_id,risk_state,calculated_at DESC);

-- +goose StatementBegin
CREATE FUNCTION r1_guard_inventory_risk_result() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    revision_state text;
BEGIN
    SELECT projection_state INTO revision_state
    FROM projection_revisions
    WHERE id=NEW.projection_revision_id
      AND user_id=NEW.user_id
      AND workspace_id=NEW.workspace_id
      AND projection_type='inventory_risk'
      AND scope_type='product'
      AND scope_id=NEW.product_id;
    IF revision_state IS NULL THEN
        RAISE EXCEPTION 'inventory risk result must bind an inventory_risk product revision'
            USING ERRCODE='23514';
    END IF;
    IF revision_state <> 'building' THEN
        RAISE EXCEPTION 'inventory risk result can only be inserted while its revision is building'
            USING ERRCODE='55000';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER inventory_risk_projections_guard
BEFORE INSERT OR UPDATE ON inventory_risk_projections
FOR EACH ROW EXECUTE FUNCTION r1_guard_inventory_risk_result();

-- +goose StatementBegin
CREATE FUNCTION r1_guard_inventory_risk_activation() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    expected_batches integer;
    completed_at_value timestamptz;
    product_rows integer;
    batch_rows integer;
BEGIN
    IF NEW.projection_type='inventory_risk'
       AND NEW.projection_state='active'
       AND OLD.projection_state IS DISTINCT FROM 'active' THEN
        SELECT expected_batch_count,completed_at
        INTO expected_batches,completed_at_value
        FROM inventory_risk_projection_sets
        WHERE projection_revision_id=NEW.id
          AND user_id=NEW.user_id
          AND workspace_id=NEW.workspace_id
          AND product_id=NEW.scope_id;
        IF expected_batches IS NULL OR completed_at_value IS NULL THEN
            RAISE EXCEPTION 'inventory risk revision is not complete'
                USING ERRCODE='55000';
        END IF;
        SELECT count(*) FILTER (WHERE result_scope='product'),
               count(*) FILTER (WHERE result_scope='batch')
        INTO product_rows,batch_rows
        FROM inventory_risk_projections
        WHERE projection_revision_id=NEW.id;
        IF product_rows <> 1 OR batch_rows <> expected_batches THEN
            RAISE EXCEPTION 'inventory risk revision result count is incomplete'
                USING ERRCODE='55000';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER projection_revisions_inventory_risk_activation_guard
BEFORE UPDATE OF projection_state ON projection_revisions
FOR EACH ROW EXECUTE FUNCTION r1_guard_inventory_risk_activation();

-- +goose StatementBegin
CREATE FUNCTION r1_activate_inventory_risk_revision(
    target_revision_id uuid,
    completion_time timestamptz
) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    target projection_revisions%ROWTYPE;
BEGIN
    SELECT * INTO target
    FROM projection_revisions
    WHERE id=target_revision_id
      AND projection_type='inventory_risk'
      AND scope_type='product'
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'inventory risk revision not found' USING ERRCODE='P0002';
    END IF;
    IF target.projection_state <> 'building' THEN
        RAISE EXCEPTION 'inventory risk revision is not building' USING ERRCODE='55000';
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(
        target.workspace_id::text || ':inventory_risk:' || target.scope_id::text, 6));
    UPDATE inventory_risk_projection_sets
    SET completed_at=completion_time
    WHERE projection_revision_id=target.id;
    UPDATE projection_revisions
    SET projection_state='superseded'
    WHERE workspace_id=target.workspace_id
      AND projection_type='inventory_risk'
      AND scope_type='product'
      AND scope_id=target.scope_id
      AND projection_state='active';
    UPDATE projection_revisions
    SET projection_state='active',calculated_at=completion_time,activated_at=completion_time,
        failed_at=NULL,error_code=''
    WHERE id=target.id;
END;
$$;
-- +goose StatementEnd

CREATE TABLE reminder_preferences (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    configuration_state text NOT NULL DEFAULT 'needs_confirmation'
        CHECK (configuration_state IN ('needs_confirmation','configured','disabled')),
    aggregate_version bigint NOT NULL DEFAULT 0 CHECK (aggregate_version >= 0),
    current_version_id uuid,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (workspace_id),
    UNIQUE (id,user_id,workspace_id),
    CHECK ((configuration_state='needs_confirmation' AND current_version_id IS NULL AND aggregate_version=0)
        OR (configuration_state IN ('configured','disabled') AND current_version_id IS NOT NULL
            AND aggregate_version > 0))
);

CREATE TABLE reminder_preference_versions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    reminder_preference_id uuid NOT NULL,
    business_version bigint NOT NULL CHECK (business_version > 0),
    enabled boolean NOT NULL,
    grouping_mode text NOT NULL CHECK (grouping_mode IN ('time_window_digest','per_occurrence')),
    plan_due_enabled boolean NOT NULL,
    overdue_enabled boolean NOT NULL,
    overdue_grace_minutes integer NOT NULL CHECK (overdue_grace_minutes BETWEEN 1 AND 1440),
    inventory_enabled boolean NOT NULL,
    expiry_enabled boolean NOT NULL,
    channel text NOT NULL DEFAULT 'in_app' CHECK (channel='in_app'),
    channel_authorization text NOT NULL DEFAULT 'not_required' CHECK (channel_authorization='not_required'),
    source text NOT NULL CHECK (source IN ('user_confirmed','demo_seed')),
    effective_from timestamptz NOT NULL,
    effective_to timestamptz,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL,
    CHECK (effective_to IS NULL OR effective_to > effective_from),
    UNIQUE (reminder_preference_id,business_version),
    UNIQUE (id,reminder_preference_id,user_id,workspace_id),
    CONSTRAINT reminder_preference_versions_preference_fk
        FOREIGN KEY (reminder_preference_id,user_id,workspace_id)
        REFERENCES reminder_preferences(id,user_id,workspace_id) ON DELETE CASCADE
);

ALTER TABLE reminder_preferences
    ADD CONSTRAINT reminder_preferences_current_version_fk
        FOREIGN KEY (current_version_id,id,user_id,workspace_id)
        REFERENCES reminder_preference_versions(id,reminder_preference_id,user_id,workspace_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED;

CREATE UNIQUE INDEX reminder_preference_versions_current_uidx
    ON reminder_preference_versions (reminder_preference_id)
    WHERE effective_to IS NULL;

CREATE TABLE reminder_window_preferences (
    reminder_preference_version_id uuid NOT NULL,
    reminder_preference_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    window_name text NOT NULL CHECK (window_name IN ('morning','noon','afternoon','evening')),
    enabled boolean NOT NULL,
    summary_time time NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (reminder_preference_version_id,window_name),
    CONSTRAINT reminder_window_preferences_version_fk
        FOREIGN KEY (reminder_preference_version_id,reminder_preference_id,user_id,workspace_id)
        REFERENCES reminder_preference_versions(id,reminder_preference_id,user_id,workspace_id)
        ON DELETE CASCADE,
    CONSTRAINT reminder_window_preferences_time_check CHECK (
        (window_name='morning' AND summary_time >= time '06:00' AND summary_time < time '10:30')
        OR (window_name='noon' AND summary_time >= time '10:30' AND summary_time < time '13:00')
        OR (window_name='afternoon' AND summary_time >= time '13:00' AND summary_time < time '17:30')
        OR (window_name='evening' AND (summary_time >= time '17:30' OR summary_time < time '06:00'))
    )
);

-- +goose StatementBegin
CREATE FUNCTION r1_guard_reminder_preference_version() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.effective_to IS NOT NULL
       OR NEW.effective_to IS NULL
       OR (to_jsonb(NEW)-'effective_to') IS DISTINCT FROM (to_jsonb(OLD)-'effective_to') THEN
        RAISE EXCEPTION 'reminder preference versions are immutable except for first close'
            USING ERRCODE='55000';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER reminder_preference_versions_immutable
BEFORE UPDATE ON reminder_preference_versions
FOR EACH ROW EXECUTE FUNCTION r1_guard_reminder_preference_version();

-- +goose StatementBegin
CREATE FUNCTION r1_block_reminder_window_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'reminder window preference facts are immutable' USING ERRCODE='55000';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER reminder_window_preferences_immutable
BEFORE UPDATE ON reminder_window_preferences
FOR EACH ROW EXECUTE FUNCTION r1_block_reminder_window_update();

-- +goose StatementBegin
CREATE FUNCTION r1_set_reminder_preference(
    preference_id uuid,
    next_version_id uuid,
    expected_aggregate_version bigint,
    preference_enabled boolean,
    next_grouping_mode text,
    next_plan_due_enabled boolean,
    next_overdue_enabled boolean,
    next_overdue_grace_minutes integer,
    next_inventory_enabled boolean,
    next_expiry_enabled boolean,
    next_source text,
    actor_user_id uuid,
    window_names text[],
    window_enabled boolean[],
    summary_times time[]
) RETURNS bigint
LANGUAGE plpgsql AS $$
DECLARE
    preference reminder_preferences%ROWTYPE;
    next_business_version bigint;
BEGIN
    SELECT * INTO preference FROM reminder_preferences WHERE id=preference_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'reminder preference not found' USING ERRCODE='P0002';
    END IF;
    IF preference.aggregate_version <> expected_aggregate_version THEN
        RAISE EXCEPTION 'reminder preference version conflict' USING ERRCODE='40001';
    END IF;
    IF cardinality(window_names) <> 4
       OR cardinality(window_enabled) <> 4
       OR cardinality(summary_times) <> 4
       OR ARRAY(SELECT DISTINCT value FROM unnest(window_names) value ORDER BY value)
          <> ARRAY['afternoon','evening','morning','noon']::text[] THEN
        RAISE EXCEPTION 'all four unique reminder windows are required' USING ERRCODE='23514';
    END IF;
    next_business_version := preference.aggregate_version + 1;
    IF preference.current_version_id IS NOT NULL THEN
        UPDATE reminder_preference_versions
        SET effective_to=CURRENT_TIMESTAMP
        WHERE id=preference.current_version_id;
    END IF;
    INSERT INTO reminder_preference_versions (
        id,user_id,workspace_id,reminder_preference_id,business_version,enabled,
        grouping_mode,plan_due_enabled,overdue_enabled,overdue_grace_minutes,
        inventory_enabled,expiry_enabled,source,effective_from,created_by,created_at
    ) VALUES (
        next_version_id,preference.user_id,preference.workspace_id,preference.id,
        next_business_version,preference_enabled,next_grouping_mode,next_plan_due_enabled,
        next_overdue_enabled,next_overdue_grace_minutes,next_inventory_enabled,
        next_expiry_enabled,next_source,CURRENT_TIMESTAMP,actor_user_id,CURRENT_TIMESTAMP
    );
    INSERT INTO reminder_window_preferences (
        reminder_preference_version_id,reminder_preference_id,user_id,workspace_id,
        window_name,enabled,summary_time,created_at
    )
    SELECT next_version_id,preference.id,preference.user_id,preference.workspace_id,
           window_names[index],window_enabled[index],summary_times[index],CURRENT_TIMESTAMP
    FROM generate_subscripts(window_names,1) AS index;
    UPDATE reminder_preferences
    SET configuration_state=CASE WHEN preference_enabled THEN 'configured' ELSE 'disabled' END,
        aggregate_version=next_business_version,current_version_id=next_version_id,
        updated_at=CURRENT_TIMESTAMP
    WHERE id=preference.id;
    RETURN next_business_version;
END;
$$;
-- +goose StatementEnd

CREATE TABLE product_reminder_overrides (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    reminder_preference_id uuid NOT NULL,
    aggregate_version bigint NOT NULL DEFAULT 1 CHECK (aggregate_version > 0),
    plan_mode text NOT NULL DEFAULT 'inherit' CHECK (plan_mode IN ('inherit','muted')),
    inventory_mode text NOT NULL DEFAULT 'inherit' CHECK (inventory_mode IN ('inherit','muted')),
    expiry_mode text NOT NULL DEFAULT 'inherit' CHECK (expiry_mode IN ('inherit','muted')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (product_id),
    CONSTRAINT product_reminder_overrides_product_fk
        FOREIGN KEY (product_id,user_id,workspace_id)
        REFERENCES products(id,user_id,workspace_id) ON DELETE CASCADE,
    CONSTRAINT product_reminder_overrides_preference_fk
        FOREIGN KEY (reminder_preference_id,user_id,workspace_id)
        REFERENCES reminder_preferences(id,user_id,workspace_id) ON DELETE CASCADE
);

CREATE TABLE reminder_events (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    reminder_preference_version_id uuid NOT NULL,
    reminder_preference_id uuid NOT NULL,
    event_type text NOT NULL CHECK (event_type IN (
        'plan_due','plan_overdue','stock_low','stock_depleted','stock_shortfall',
        'expiry_near','expiry_unfinishable','expiry_expired'
    )),
    source_type text NOT NULL CHECK (source_type IN ('scheduled_occurrence','inventory_risk_projection')),
    source_id uuid NOT NULL,
    source_revision bigint NOT NULL CHECK (source_revision > 0),
    dedupe_key text NOT NULL CHECK (char_length(dedupe_key) BETWEEN 1 AND 300),
    scheduled_at timestamptz NOT NULL,
    available_at timestamptz NOT NULL,
    iana_timezone text NOT NULL CHECK (char_length(iana_timezone) BETWEEN 1 AND 120),
    channel text NOT NULL DEFAULT 'in_app' CHECK (channel='in_app'),
    event_status text NOT NULL CHECK (event_status IN (
        'scheduled','available','resolved','dismissed','expired','cancelled'
    )),
    read_at timestamptz,
    resolved_at timestamptz,
    dismissed_at timestamptz,
    expired_at timestamptz,
    cancelled_at timestamptz,
    title_snapshot text NOT NULL CHECK (char_length(title_snapshot) BETWEEN 1 AND 160),
    body_snapshot text NOT NULL DEFAULT '' CHECK (char_length(body_snapshot) <= 1000),
    fact_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(fact_snapshot)='object'),
    deep_link_path text NOT NULL DEFAULT '' CHECK (char_length(deep_link_path) <= 300),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (workspace_id,dedupe_key),
    UNIQUE (id,user_id,workspace_id),
    CONSTRAINT reminder_events_preference_version_fk
        FOREIGN KEY (reminder_preference_version_id,reminder_preference_id,user_id,workspace_id)
        REFERENCES reminder_preference_versions(id,reminder_preference_id,user_id,workspace_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT reminder_events_time_check CHECK (available_at >= scheduled_at),
    CONSTRAINT reminder_events_read_check CHECK (read_at IS NULL OR event_status<>'scheduled'),
    CONSTRAINT reminder_events_lifecycle_check CHECK (
        (event_status='scheduled' AND resolved_at IS NULL AND dismissed_at IS NULL
            AND expired_at IS NULL AND cancelled_at IS NULL)
        OR (event_status='available' AND resolved_at IS NULL AND dismissed_at IS NULL
            AND expired_at IS NULL AND cancelled_at IS NULL)
        OR (event_status='resolved' AND resolved_at IS NOT NULL AND dismissed_at IS NULL
            AND expired_at IS NULL AND cancelled_at IS NULL)
        OR (event_status='dismissed' AND dismissed_at IS NOT NULL AND resolved_at IS NULL
            AND expired_at IS NULL AND cancelled_at IS NULL)
        OR (event_status='expired' AND expired_at IS NOT NULL AND resolved_at IS NULL
            AND dismissed_at IS NULL AND cancelled_at IS NULL)
        OR (event_status='cancelled' AND cancelled_at IS NOT NULL AND resolved_at IS NULL
            AND dismissed_at IS NULL AND expired_at IS NULL)
    )
);

CREATE INDEX reminder_events_center_idx
    ON reminder_events (workspace_id,event_status,available_at DESC,id)
    WHERE event_status IN ('available','dismissed');
CREATE INDEX reminder_events_unread_idx
    ON reminder_events (workspace_id,available_at DESC,id)
    WHERE event_status='available' AND read_at IS NULL;
CREATE UNIQUE INDEX reminder_events_source_policy_uidx
    ON reminder_events (
        workspace_id,source_type,source_id,source_revision,
        reminder_preference_version_id,event_type
    );

-- +goose StatementBegin
CREATE FUNCTION r1_guard_reminder_event_transition() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id <> OLD.id OR NEW.user_id <> OLD.user_id OR NEW.workspace_id <> OLD.workspace_id
       OR NEW.reminder_preference_version_id <> OLD.reminder_preference_version_id
       OR NEW.reminder_preference_id <> OLD.reminder_preference_id
       OR NEW.event_type <> OLD.event_type OR NEW.source_type <> OLD.source_type
       OR NEW.source_id <> OLD.source_id OR NEW.source_revision <> OLD.source_revision
       OR NEW.dedupe_key <> OLD.dedupe_key OR NEW.scheduled_at <> OLD.scheduled_at
       OR NEW.available_at <> OLD.available_at OR NEW.iana_timezone <> OLD.iana_timezone
       OR NEW.channel <> OLD.channel OR NEW.title_snapshot <> OLD.title_snapshot
       OR NEW.body_snapshot <> OLD.body_snapshot OR NEW.fact_snapshot <> OLD.fact_snapshot
       OR NEW.deep_link_path <> OLD.deep_link_path OR NEW.created_at <> OLD.created_at THEN
        RAISE EXCEPTION 'reminder event identity and snapshots are immutable' USING ERRCODE='55000';
    END IF;
    IF NEW.event_status <> OLD.event_status AND NOT (
        (OLD.event_status='scheduled' AND NEW.event_status IN ('available','cancelled'))
        OR (OLD.event_status='available' AND NEW.event_status IN ('resolved','dismissed','expired','cancelled'))
        OR (OLD.event_status='dismissed' AND NEW.event_status IN ('available','resolved'))
    ) THEN
        RAISE EXCEPTION 'invalid reminder event transition % -> %',OLD.event_status,NEW.event_status
            USING ERRCODE='23514';
    END IF;
    IF OLD.read_at IS NOT NULL AND NEW.read_at IS DISTINCT FROM OLD.read_at THEN
        RAISE EXCEPTION 'reminder read timestamp cannot be cleared or rewritten' USING ERRCODE='55000';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER reminder_events_transition_guard
BEFORE UPDATE ON reminder_events
FOR EACH ROW EXECUTE FUNCTION r1_guard_reminder_event_transition();

CREATE TABLE reminder_targets (
    id uuid PRIMARY KEY,
    event_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    target_kind text NOT NULL CHECK (target_kind IN ('occurrence','product','batch')),
    occurrence_id uuid,
    product_id uuid NOT NULL,
    batch_id uuid,
    target_status text NOT NULL CHECK (target_status IN ('active','resolved','cancelled','expired')),
    remaining_quantity numeric(20,6) CHECK (remaining_quantity IS NULL OR remaining_quantity >= 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT reminder_targets_kind_check CHECK (
        (target_kind='occurrence' AND occurrence_id IS NOT NULL AND batch_id IS NULL)
        OR (target_kind='product' AND occurrence_id IS NULL AND batch_id IS NULL)
        OR (target_kind='batch' AND occurrence_id IS NULL AND batch_id IS NOT NULL)
    ),
    CONSTRAINT reminder_targets_event_fk
        FOREIGN KEY (event_id,user_id,workspace_id)
        REFERENCES reminder_events(id,user_id,workspace_id) ON DELETE CASCADE,
    CONSTRAINT reminder_targets_product_fk
        FOREIGN KEY (product_id,user_id,workspace_id)
        REFERENCES products(id,user_id,workspace_id) ON DELETE CASCADE,
    CONSTRAINT reminder_targets_occurrence_fk
        FOREIGN KEY (occurrence_id,user_id,workspace_id,product_id)
        REFERENCES scheduled_occurrences(id,user_id,workspace_id,product_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT reminder_targets_batch_fk
        FOREIGN KEY (batch_id,user_id,workspace_id,product_id)
        REFERENCES inventory_batches(id,user_id,workspace_id,product_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED
);

CREATE UNIQUE INDEX reminder_targets_occurrence_uidx
    ON reminder_targets (event_id,occurrence_id) WHERE occurrence_id IS NOT NULL;
CREATE UNIQUE INDEX reminder_targets_product_uidx
    ON reminder_targets (event_id,product_id) WHERE target_kind='product';
CREATE UNIQUE INDEX reminder_targets_batch_uidx
    ON reminder_targets (event_id,batch_id) WHERE batch_id IS NOT NULL;

CREATE TABLE reminder_materialization_cursors (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    policy_partition text NOT NULL CHECK (char_length(policy_partition) BETWEEN 1 AND 120),
    source_partition text NOT NULL CHECK (source_partition IN ('plan','risk')),
    cursor_version bigint NOT NULL DEFAULT 0 CHECK (cursor_version >= 0),
    last_processed_at timestamptz,
    last_event_id uuid,
    cursor_state text NOT NULL DEFAULT 'idle' CHECK (cursor_state IN ('idle','running','failed')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count BETWEEN 0 AND 1000),
    next_attempt_at timestamptz,
    error_code text NOT NULL DEFAULT '' CHECK (char_length(error_code) <= 120),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (workspace_id,policy_partition,source_partition),
    CHECK ((last_processed_at IS NULL AND last_event_id IS NULL)
        OR (last_processed_at IS NOT NULL AND last_event_id IS NOT NULL))
);

-- +goose StatementBegin
CREATE FUNCTION r1_guard_reminder_cursor_monotonic() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.user_id <> OLD.user_id OR NEW.workspace_id <> OLD.workspace_id
       OR NEW.policy_partition <> OLD.policy_partition
       OR NEW.source_partition <> OLD.source_partition
       OR NEW.created_at <> OLD.created_at THEN
        RAISE EXCEPTION 'reminder cursor identity is immutable' USING ERRCODE='55000';
    END IF;
    IF NEW.cursor_version <> OLD.cursor_version + 1 THEN
        RAISE EXCEPTION 'reminder cursor version must advance exactly once' USING ERRCODE='40001';
    END IF;
    IF OLD.last_processed_at IS NOT NULL AND (
        NEW.last_processed_at IS NULL
        OR NEW.last_processed_at < OLD.last_processed_at
        OR (NEW.last_processed_at = OLD.last_processed_at AND NEW.last_event_id::text < OLD.last_event_id::text)
    ) THEN
        RAISE EXCEPTION 'reminder cursor cannot move backwards' USING ERRCODE='22000';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER reminder_materialization_cursors_monotonic
BEFORE UPDATE ON reminder_materialization_cursors
FOR EACH ROW EXECUTE FUNCTION r1_guard_reminder_cursor_monotonic();

-- +goose StatementBegin
CREATE FUNCTION r1_backfill_reminder_preferences() RETURNS bigint
LANGUAGE plpgsql AS $$
DECLARE
    inserted_count bigint;
BEGIN
    INSERT INTO reminder_preferences (
        id,user_id,workspace_id,configuration_state,aggregate_version,
        current_version_id,created_at,updated_at
    )
    SELECT gen_random_uuid(),w.owner_user_id,w.id,'needs_confirmation',0,NULL,
           CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
    FROM workspaces w
    WHERE NOT EXISTS (
        SELECT 1 FROM reminder_preferences preference WHERE preference.workspace_id=w.id
    )
    ON CONFLICT (workspace_id) DO NOTHING;
    GET DIAGNOSTICS inserted_count = ROW_COUNT;
    RETURN inserted_count;
END;
$$;
-- +goose StatementEnd

SELECT r1_backfill_reminder_preferences();

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM reminder_preferences
        WHERE configuration_state <> 'needs_confirmation'
           OR aggregate_version <> 0 OR current_version_id IS NOT NULL
    )
       OR EXISTS (SELECT 1 FROM reminder_preference_versions)
       OR EXISTS (SELECT 1 FROM product_reminder_overrides)
       OR EXISTS (SELECT 1 FROM reminder_events)
       OR EXISTS (SELECT 1 FROM reminder_targets)
       OR EXISTS (SELECT 1 FROM reminder_materialization_cursors)
       OR EXISTS (SELECT 1 FROM inventory_risk_projection_sets)
       OR EXISTS (SELECT 1 FROM inventory_risk_projections)
       OR EXISTS (SELECT 1 FROM projection_revisions WHERE projection_type='inventory_risk') THEN
        RAISE EXCEPTION 'E6 target writes exist; rollback requires a forward fix' USING ERRCODE='55000';
    END IF;
END;
$$;
-- +goose StatementEnd

DROP FUNCTION IF EXISTS r1_backfill_reminder_preferences();
DROP TRIGGER IF EXISTS reminder_materialization_cursors_monotonic ON reminder_materialization_cursors;
DROP FUNCTION IF EXISTS r1_guard_reminder_cursor_monotonic();
DROP TABLE IF EXISTS reminder_materialization_cursors;
DROP INDEX IF EXISTS reminder_targets_batch_uidx;
DROP INDEX IF EXISTS reminder_targets_product_uidx;
DROP INDEX IF EXISTS reminder_targets_occurrence_uidx;
DROP TABLE IF EXISTS reminder_targets;
DROP TRIGGER IF EXISTS reminder_events_transition_guard ON reminder_events;
DROP FUNCTION IF EXISTS r1_guard_reminder_event_transition();
DROP INDEX IF EXISTS reminder_events_source_policy_uidx;
DROP INDEX IF EXISTS reminder_events_unread_idx;
DROP INDEX IF EXISTS reminder_events_center_idx;
DROP TABLE IF EXISTS reminder_events;
DROP TABLE IF EXISTS product_reminder_overrides;
DROP FUNCTION IF EXISTS r1_set_reminder_preference(
    uuid,uuid,bigint,boolean,text,boolean,boolean,integer,boolean,boolean,text,uuid,text[],boolean[],time[]
);
DROP TRIGGER IF EXISTS reminder_window_preferences_immutable ON reminder_window_preferences;
DROP FUNCTION IF EXISTS r1_block_reminder_window_update();
DROP TABLE IF EXISTS reminder_window_preferences;
ALTER TABLE reminder_preferences DROP CONSTRAINT IF EXISTS reminder_preferences_current_version_fk;
DROP TRIGGER IF EXISTS reminder_preference_versions_immutable ON reminder_preference_versions;
DROP FUNCTION IF EXISTS r1_guard_reminder_preference_version();
DROP TABLE IF EXISTS reminder_preference_versions;
DROP TABLE IF EXISTS reminder_preferences;
DROP FUNCTION IF EXISTS r1_activate_inventory_risk_revision(uuid,timestamptz);
DROP TRIGGER IF EXISTS projection_revisions_inventory_risk_activation_guard ON projection_revisions;
DROP FUNCTION IF EXISTS r1_guard_inventory_risk_activation();
DROP TRIGGER IF EXISTS inventory_risk_projections_guard ON inventory_risk_projections;
DROP FUNCTION IF EXISTS r1_guard_inventory_risk_result();
DROP INDEX IF EXISTS inventory_risk_projection_active_lookup_idx;
DROP INDEX IF EXISTS inventory_risk_projection_batch_uidx;
DROP INDEX IF EXISTS inventory_risk_projection_product_uidx;
DROP TABLE IF EXISTS inventory_risk_projections;
DROP TABLE IF EXISTS inventory_risk_projection_sets;
DROP INDEX IF EXISTS projection_revisions_tenant_identity_uidx;
