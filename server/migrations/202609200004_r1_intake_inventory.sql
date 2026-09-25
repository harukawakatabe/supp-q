-- +goose Up
ALTER TABLE inventory_batches
    DROP CONSTRAINT IF EXISTS inventory_batches_current_quantity_check;

ALTER TABLE inventory_batches
    ADD COLUMN aggregate_version bigint,
    ADD COLUMN lifecycle_state text,
    ADD COLUMN unit_snapshot text,
    ADD COLUMN received_quantity numeric(20,6),
    ADD COLUMN received_at date,
    ADD COLUMN lot_code text,
    ADD COLUMN expiry_raw_value text,
    ADD COLUMN expiry_precision text,
    ADD COLUMN opened_at timestamptz,
    ADD COLUMN currency char(3),
    ADD COLUMN source_client_action_id uuid,
    ADD COLUMN source_request_key text,
    ADD COLUMN request_hash char(64),
    ADD COLUMN source_kind text,
    ADD COLUMN updated_at timestamptz,
    ADD CONSTRAINT inventory_batches_current_quantity_nonnegative_check
        CHECK (current_quantity >= 0),
    ADD CONSTRAINT inventory_batches_aggregate_version_check
        CHECK (aggregate_version IS NULL OR aggregate_version > 0),
    ADD CONSTRAINT inventory_batches_lifecycle_state_check
        CHECK (lifecycle_state IS NULL OR lifecycle_state IN (
            'available_unstarted','in_use','depleted','voided'
        )),
    ADD CONSTRAINT inventory_batches_unit_snapshot_check
        CHECK (unit_snapshot IS NULL OR char_length(unit_snapshot) BETWEEN 1 AND 40),
    ADD CONSTRAINT inventory_batches_received_quantity_check
        CHECK (received_quantity IS NULL OR received_quantity > 0),
    ADD CONSTRAINT inventory_batches_lot_code_check
        CHECK (lot_code IS NULL OR char_length(lot_code) <= 120),
    ADD CONSTRAINT inventory_batches_expiry_precision_check
        CHECK (expiry_precision IS NULL OR expiry_precision IN ('day','month','year','unknown')),
    ADD CONSTRAINT inventory_batches_currency_check
        CHECK (currency IS NULL OR currency ~ '^[A-Z]{3}$'),
    ADD CONSTRAINT inventory_batches_request_hash_check
        CHECK (request_hash IS NULL OR request_hash ~ '^[0-9a-f]{64}$'),
    ADD CONSTRAINT inventory_batches_source_kind_check
        CHECK (source_kind IS NULL OR source_kind IN ('legacy_migration','legacy_n1','application_current'));

CREATE UNIQUE INDEX inventory_batches_tenant_product_identity_uidx
    ON inventory_batches (id, user_id, workspace_id, product_id);
CREATE UNIQUE INDEX inventory_batches_source_request_uidx
    ON inventory_batches (workspace_id, source_request_key)
    WHERE source_request_key IS NOT NULL;

ALTER TABLE intake_records
    ADD COLUMN aggregate_version bigint,
    ADD COLUMN client_action_id uuid,
    ADD COLUMN request_hash char(64),
    ADD COLUMN scheduled_occurrence_id uuid,
    ADD COLUMN occurred_at timestamptz,
    ADD COLUMN occurred_time_precision text,
    ADD COLUMN timezone_version_id uuid,
    ADD COLUMN iana_timezone_snapshot text,
    ADD COLUMN product_profile_version_id uuid,
    ADD COLUMN ingredient_profile_version_id uuid,
    ADD COLUMN supersedes_intake_id uuid,
    ADD COLUMN superseded_by_intake_id uuid,
    ADD COLUMN history_completeness text,
    ADD COLUMN source_kind text,
    ADD CONSTRAINT intake_records_aggregate_version_check
        CHECK (aggregate_version IS NULL OR aggregate_version > 0),
    ADD CONSTRAINT intake_records_request_hash_check
        CHECK (request_hash IS NULL OR request_hash ~ '^[0-9a-f]{64}$'),
    ADD CONSTRAINT intake_records_time_precision_check
        CHECK (occurred_time_precision IS NULL OR occurred_time_precision IN ('date','minute')),
    ADD CONSTRAINT intake_records_timezone_snapshot_check
        CHECK (iana_timezone_snapshot IS NULL OR char_length(iana_timezone_snapshot) BETWEEN 1 AND 120),
    ADD CONSTRAINT intake_records_history_completeness_check
        CHECK (history_completeness IS NULL OR history_completeness IN ('complete','legacy_current_only')),
    ADD CONSTRAINT intake_records_source_kind_check
        CHECK (source_kind IS NULL OR source_kind IN ('legacy_migration','legacy_n1','application_current')),
    ADD CONSTRAINT intake_records_supersession_distinct_check
        CHECK (supersedes_intake_id IS NULL OR supersedes_intake_id <> id),
    ADD CONSTRAINT intake_records_superseded_by_distinct_check
        CHECK (superseded_by_intake_id IS NULL OR superseded_by_intake_id <> id);

CREATE UNIQUE INDEX intake_records_tenant_product_identity_uidx
    ON intake_records (id, user_id, workspace_id, product_id);
CREATE UNIQUE INDEX scheduled_occurrences_tenant_product_identity_uidx
    ON scheduled_occurrences (id, user_id, workspace_id, product_id);

ALTER TABLE inventory_events
    ADD COLUMN ledger_kind text,
    ADD COLUMN source_type text,
    ADD COLUMN source_id uuid,
    ADD COLUMN source_request_key text,
    ADD COLUMN client_action_id uuid,
    ADD COLUMN request_hash char(64),
    ADD COLUMN posted_at timestamptz,
    ADD COLUMN occurred_at timestamptz,
    ADD COLUMN reason_code text,
    ADD COLUMN note text,
    ADD COLUMN actor_user_id uuid,
    ADD COLUMN batch_version_before bigint,
    ADD COLUMN balance_after numeric(20,6),
    ADD COLUMN compensates_event_id uuid,
    ADD COLUMN source_kind text,
    ADD CONSTRAINT inventory_events_ledger_kind_check
        CHECK (ledger_kind IS NULL OR ledger_kind IN (
            'opening','restock','intake','intake_undo',
            'adjustment_increase','adjustment_decrease','batch_void_reversal'
        )),
    ADD CONSTRAINT inventory_events_source_type_check
        CHECK (source_type IS NULL OR char_length(source_type) BETWEEN 1 AND 80),
    ADD CONSTRAINT inventory_events_request_hash_check
        CHECK (request_hash IS NULL OR request_hash ~ '^[0-9a-f]{64}$'),
    ADD CONSTRAINT inventory_events_reason_code_check
        CHECK (reason_code IS NULL OR char_length(reason_code) <= 120),
    ADD CONSTRAINT inventory_events_note_check
        CHECK (note IS NULL OR char_length(note) <= 500),
    ADD CONSTRAINT inventory_events_batch_version_check
        CHECK (batch_version_before IS NULL OR batch_version_before >= 0),
    ADD CONSTRAINT inventory_events_balance_after_check
        CHECK (balance_after IS NULL OR balance_after >= 0),
    ADD CONSTRAINT inventory_events_source_kind_check
        CHECK (source_kind IS NULL OR source_kind IN ('legacy_migration','legacy_n1','application_current')),
    ADD CONSTRAINT inventory_events_compensation_distinct_check
        CHECK (compensates_event_id IS NULL OR compensates_event_id <> id);

CREATE UNIQUE INDEX inventory_events_source_identity_uidx
    ON inventory_events (workspace_id, source_type, source_id, batch_id, ledger_kind)
    WHERE source_type IS NOT NULL AND source_id IS NOT NULL AND ledger_kind IS NOT NULL;
CREATE UNIQUE INDEX inventory_events_compensation_uidx
    ON inventory_events (compensates_event_id)
    WHERE compensates_event_id IS NOT NULL;
CREATE INDEX inventory_events_batch_posted_idx
    ON inventory_events (workspace_id, batch_id, posted_at, id);

ALTER TABLE intake_allocations
    ADD COLUMN user_id uuid,
    ADD COLUMN workspace_id uuid,
    ADD COLUMN product_id uuid,
    ADD COLUMN allocation_mode text,
    ADD COLUMN inventory_event_id uuid,
    ADD COLUMN batch_version_as_allocated bigint,
    ADD COLUMN batch_balance_before numeric(20,6),
    ADD COLUMN batch_balance_after numeric(20,6),
    ADD COLUMN unit_snapshot text,
    ADD COLUMN ingredient_profile_version_id uuid,
    ADD COLUMN source_kind text,
    ADD CONSTRAINT intake_allocations_mode_check
        CHECK (allocation_mode IS NULL OR allocation_mode IN ('fefo_auto','manual_selected','migration_existing')),
    ADD CONSTRAINT intake_allocations_batch_version_check
        CHECK (batch_version_as_allocated IS NULL OR batch_version_as_allocated >= 0),
    ADD CONSTRAINT intake_allocations_balance_before_check
        CHECK (batch_balance_before IS NULL OR batch_balance_before >= quantity),
    ADD CONSTRAINT intake_allocations_balance_after_check
        CHECK (batch_balance_after IS NULL OR batch_balance_after >= 0),
    ADD CONSTRAINT intake_allocations_balance_delta_check
        CHECK (batch_balance_before IS NULL OR batch_balance_after IS NULL
            OR batch_balance_before - batch_balance_after = quantity),
    ADD CONSTRAINT intake_allocations_unit_snapshot_check
        CHECK (unit_snapshot IS NULL OR char_length(unit_snapshot) BETWEEN 1 AND 40),
    ADD CONSTRAINT intake_allocations_source_kind_check
        CHECK (source_kind IS NULL OR source_kind IN ('legacy_migration','legacy_n1','application_current'));

ALTER TABLE intake_allocations
    DROP CONSTRAINT intake_allocations_batch_id_fkey,
    ADD CONSTRAINT intake_allocations_batch_id_fkey
        FOREIGN KEY (batch_id) REFERENCES inventory_batches(id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE inventory_batches
    DROP CONSTRAINT inventory_batches_ingredient_profile_fk,
    ADD CONSTRAINT inventory_batches_ingredient_profile_fk
        FOREIGN KEY (ingredient_profile_version_id,product_id,user_id,workspace_id)
        REFERENCES ingredient_profile_versions(id,product_id,user_id,workspace_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID;

CREATE UNIQUE INDEX intake_allocations_inventory_event_uidx
    ON intake_allocations (inventory_event_id)
    WHERE inventory_event_id IS NOT NULL;

CREATE TABLE intake_status_facts (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    intake_id uuid NOT NULL,
    business_version bigint NOT NULL CHECK (business_version > 0),
    status text NOT NULL CHECK (status IN ('active','revoked','superseded')),
    effective_at timestamptz NOT NULL,
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    source_kind text NOT NULL CHECK (source_kind IN ('legacy_migration','legacy_n1','application_current')),
    created_at timestamptz NOT NULL,
    UNIQUE (intake_id, business_version)
);

CREATE INDEX intake_status_facts_workspace_intake_idx
    ON intake_status_facts (workspace_id, intake_id, business_version DESC);

CREATE TABLE r1_intake_inventory_backfill_state (
    singleton boolean PRIMARY KEY DEFAULT true
        CONSTRAINT r1_intake_inventory_backfill_singleton_check CHECK (singleton),
    cycle_id uuid,
    cycle_state text NOT NULL DEFAULT 'idle'
        CONSTRAINT r1_intake_inventory_backfill_cycle_check CHECK (cycle_state IN ('idle','running')),
    phase text NOT NULL DEFAULT 'batches'
        CONSTRAINT r1_intake_inventory_backfill_phase_check CHECK (phase IN ('batches','intakes','events','allocations')),
    cursor_id uuid,
    attempt_count bigint NOT NULL DEFAULT 0
        CONSTRAINT r1_intake_inventory_backfill_attempt_check CHECK (attempt_count >= 0),
    processed_count bigint NOT NULL DEFAULT 0
        CONSTRAINT r1_intake_inventory_backfill_processed_check CHECK (processed_count >= 0),
    source_count bigint NOT NULL DEFAULT 0
        CONSTRAINT r1_intake_inventory_backfill_source_count_check CHECK (source_count >= 0),
    source_snapshot_hash char(32),
    cycle_started_at timestamptz,
    last_progress_at timestamptz,
    last_completed_at timestamptz,
    updated_at timestamptz NOT NULL,
    CONSTRAINT r1_intake_inventory_backfill_running_check
        CHECK ((cycle_state='running' AND cycle_id IS NOT NULL AND cycle_started_at IS NOT NULL)
            OR (cycle_state='idle' AND cycle_id IS NULL AND cursor_id IS NULL))
);

INSERT INTO r1_intake_inventory_backfill_state (singleton, updated_at)
VALUES (true, CURRENT_TIMESTAMP);

-- +goose StatementBegin
CREATE FUNCTION r1_prepare_inventory_batch() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    product_unit text;
BEGIN
    IF TG_OP='INSERT' THEN
        SELECT unit INTO product_unit
        FROM products
        WHERE id=NEW.product_id AND user_id=NEW.user_id AND workspace_id=NEW.workspace_id;
        IF product_unit IS NULL THEN
            RAISE EXCEPTION 'inventory batch product tenant mismatch' USING ERRCODE='23503';
        END IF;
        NEW.aggregate_version:=COALESCE(NEW.aggregate_version,1);
        NEW.unit_snapshot:=COALESCE(NEW.unit_snapshot,product_unit);
        NEW.received_quantity:=COALESCE(NEW.received_quantity,NEW.initial_quantity);
        NEW.received_at:=COALESCE(NEW.received_at,NEW.created_at::date,CURRENT_DATE);
        NEW.expiry_precision:=COALESCE(NEW.expiry_precision,CASE WHEN NEW.expiry_date IS NULL THEN 'unknown' ELSE 'day' END);
        NEW.expiry_raw_value:=COALESCE(NEW.expiry_raw_value,NEW.expiry_date::text);
        NEW.currency:=COALESCE(NEW.currency,'CNY');
        NEW.source_kind:=COALESCE(NEW.source_kind,'legacy_n1');
        NEW.updated_at:=COALESCE(NEW.updated_at,NEW.created_at,CURRENT_TIMESTAMP);
    ELSIF NEW.current_quantity IS DISTINCT FROM OLD.current_quantity THEN
        IF NEW.aggregate_version IS NULL OR NEW.aggregate_version <= OLD.aggregate_version THEN
            NEW.aggregate_version:=OLD.aggregate_version+1;
        END IF;
        NEW.updated_at:=COALESCE(NEW.updated_at,CURRENT_TIMESTAMP);
    END IF;
    IF NEW.lifecycle_state IS DISTINCT FROM 'voided' THEN
        NEW.lifecycle_state:=CASE
            WHEN NEW.current_quantity=0 THEN 'depleted'
            WHEN NEW.current_quantity<COALESCE(NEW.received_quantity,NEW.initial_quantity) THEN 'in_use'
            ELSE 'available_unstarted'
        END;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER inventory_batches_prepare
BEFORE INSERT OR UPDATE ON inventory_batches
FOR EACH ROW EXECUTE FUNCTION r1_prepare_inventory_batch();

-- +goose StatementBegin
CREATE FUNCTION r1_prepare_intake_record() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    timezone_id uuid;
    timezone_name text;
    product_profile_id uuid;
    ingredient_profile_id uuid;
BEGIN
    IF TG_OP='INSERT' THEN
        SELECT w.current_timezone_version_id,tz.iana_timezone
        INTO timezone_id,timezone_name
        FROM workspaces w
        JOIN workspace_timezone_versions tz
          ON tz.id=w.current_timezone_version_id
         AND tz.workspace_id=w.id
         AND tz.user_id=w.owner_user_id
        WHERE w.id=NEW.workspace_id AND w.owner_user_id=NEW.user_id;
        SELECT current_product_profile_version_id,current_ingredient_profile_version_id
        INTO product_profile_id,ingredient_profile_id
        FROM products
        WHERE id=NEW.product_id AND user_id=NEW.user_id AND workspace_id=NEW.workspace_id;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'intake product tenant mismatch' USING ERRCODE='23503';
        END IF;
        NEW.aggregate_version:=COALESCE(NEW.aggregate_version,1);
        NEW.timezone_version_id:=COALESCE(NEW.timezone_version_id,timezone_id);
        NEW.iana_timezone_snapshot:=COALESCE(NEW.iana_timezone_snapshot,timezone_name,'Etc/UTC');
        NEW.occurred_time_precision:=COALESCE(NEW.occurred_time_precision,
            CASE WHEN NEW.intake_time IS NULL THEN 'date' ELSE 'minute' END);
        NEW.occurred_at:=COALESCE(NEW.occurred_at,
            (NEW.intake_date+COALESCE(NEW.intake_time,time '00:00')) AT TIME ZONE NEW.iana_timezone_snapshot);
        NEW.product_profile_version_id:=COALESCE(NEW.product_profile_version_id,product_profile_id);
        NEW.ingredient_profile_version_id:=COALESCE(NEW.ingredient_profile_version_id,ingredient_profile_id);
        NEW.history_completeness:=COALESCE(NEW.history_completeness,'legacy_current_only');
        NEW.source_kind:=COALESCE(NEW.source_kind,'legacy_n1');
    ELSIF NEW.status IS DISTINCT FROM OLD.status THEN
        IF NEW.aggregate_version IS NULL OR NEW.aggregate_version <= OLD.aggregate_version THEN
            NEW.aggregate_version:=OLD.aggregate_version+1;
        END IF;
    END IF;
    IF NEW.scheduled_occurrence_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM scheduled_occurrences occurrence
        WHERE occurrence.id=NEW.scheduled_occurrence_id
          AND occurrence.user_id=NEW.user_id
          AND occurrence.workspace_id=NEW.workspace_id
          AND occurrence.product_id=NEW.product_id
    ) THEN
        RAISE EXCEPTION 'intake occurrence tenant or product mismatch' USING ERRCODE='23503';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER intake_records_prepare
BEFORE INSERT OR UPDATE ON intake_records
FOR EACH ROW EXECUTE FUNCTION r1_prepare_intake_record();

-- +goose StatementBegin
CREATE FUNCTION r1_append_intake_status_fact() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='INSERT' OR NEW.status IS DISTINCT FROM OLD.status THEN
        INSERT INTO intake_status_facts (
            id,user_id,workspace_id,product_id,intake_id,business_version,status,
            effective_at,actor_user_id,source_kind,created_at
        ) VALUES (
            gen_random_uuid(),NEW.user_id,NEW.workspace_id,NEW.product_id,NEW.id,
            COALESCE(NEW.aggregate_version,1),NEW.status,
            CASE WHEN NEW.status='revoked' THEN COALESCE(NEW.revoked_at,CURRENT_TIMESTAMP) ELSE NEW.created_at END,
            NEW.user_id,COALESCE(NEW.source_kind,'legacy_n1'),CURRENT_TIMESTAMP
        ) ON CONFLICT (intake_id,business_version) DO NOTHING;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER intake_records_status_fact
AFTER INSERT OR UPDATE OF status ON intake_records
FOR EACH ROW EXECUTE FUNCTION r1_append_intake_status_fact();

-- +goose StatementBegin
CREATE FUNCTION r1_block_intake_status_fact_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' AND pg_trigger_depth()>1 THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'intake status facts are immutable' USING ERRCODE='55000';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER intake_status_facts_block_change
BEFORE UPDATE OR DELETE ON intake_status_facts
FOR EACH ROW EXECUTE FUNCTION r1_block_intake_status_fact_change();

-- +goose StatementBegin
CREATE FUNCTION r1_prepare_inventory_event() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    current_version bigint;
    current_balance numeric(20,6);
BEGIN
    SELECT aggregate_version,current_quantity
    INTO current_version,current_balance
    FROM inventory_batches
    WHERE id=NEW.batch_id AND user_id=NEW.user_id
      AND workspace_id=NEW.workspace_id AND product_id=NEW.product_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'inventory event batch tenant mismatch' USING ERRCODE='23503';
    END IF;
    IF NEW.intake_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM intake_records i
        WHERE i.id=NEW.intake_id AND i.user_id=NEW.user_id
          AND i.workspace_id=NEW.workspace_id AND i.product_id=NEW.product_id
    ) THEN
        RAISE EXCEPTION 'inventory event intake tenant mismatch' USING ERRCODE='23503';
    END IF;
    NEW.ledger_kind:=COALESCE(NEW.ledger_kind,CASE NEW.kind
        WHEN 'undo' THEN 'intake_undo'
        WHEN 'adjustment' THEN CASE WHEN NEW.quantity_delta>=0 THEN 'adjustment_increase' ELSE 'adjustment_decrease' END
        ELSE NEW.kind END);
    NEW.source_type:=COALESCE(NEW.source_type,CASE
        WHEN NEW.kind='intake' THEN 'intake'
        WHEN NEW.kind='undo' THEN 'intake_undo'
        WHEN NEW.kind IN ('opening','restock') THEN 'batch'
        ELSE 'inventory_event' END);
    NEW.source_id:=COALESCE(NEW.source_id,CASE
        WHEN NEW.kind IN ('intake','undo') THEN NEW.intake_id
        WHEN NEW.kind IN ('opening','restock') THEN NEW.batch_id
        ELSE NEW.id END);
    NEW.posted_at:=COALESCE(NEW.posted_at,NEW.created_at,CURRENT_TIMESTAMP);
    NEW.actor_user_id:=COALESCE(NEW.actor_user_id,NEW.user_id);
    NEW.batch_version_before:=COALESCE(NEW.batch_version_before,GREATEST(COALESCE(current_version,1)-1,0));
    NEW.balance_after:=COALESCE(NEW.balance_after,current_balance);
    NEW.source_kind:=COALESCE(NEW.source_kind,'legacy_n1');
    IF NEW.kind='undo' AND NEW.compensates_event_id IS NULL THEN
        SELECT id INTO NEW.compensates_event_id
        FROM inventory_events
        WHERE intake_id=NEW.intake_id AND batch_id=NEW.batch_id
          AND user_id=NEW.user_id AND workspace_id=NEW.workspace_id
          AND COALESCE(ledger_kind,kind)='intake'
        ORDER BY created_at,id LIMIT 1;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER inventory_events_prepare
BEFORE INSERT ON inventory_events
FOR EACH ROW EXECUTE FUNCTION r1_prepare_inventory_event();

-- +goose StatementBegin
CREATE FUNCTION r1_block_inventory_event_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF pg_trigger_depth()>1 THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'inventory events are immutable; append a compensation event'
        USING ERRCODE='55000';
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_prepare_intake_allocation() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    intake_row intake_records%ROWTYPE;
    batch_row inventory_batches%ROWTYPE;
BEGIN
    SELECT * INTO intake_row FROM intake_records WHERE id=NEW.intake_id;
    SELECT * INTO batch_row FROM inventory_batches WHERE id=NEW.batch_id;
    IF TG_OP='UPDATE' AND (
        NEW.intake_id IS DISTINCT FROM OLD.intake_id
        OR NEW.batch_id IS DISTINCT FROM OLD.batch_id
        OR NEW.quantity IS DISTINCT FROM OLD.quantity
        OR NEW.unit_cost_cny IS DISTINCT FROM OLD.unit_cost_cny
    ) THEN
        RAISE EXCEPTION 'intake allocation core facts are immutable' USING ERRCODE='55000';
    END IF;
    IF intake_row.id IS NULL OR batch_row.id IS NULL
       OR intake_row.user_id<>batch_row.user_id
       OR intake_row.workspace_id<>batch_row.workspace_id
       OR intake_row.product_id<>batch_row.product_id THEN
        RAISE EXCEPTION 'intake allocation tenant or product mismatch' USING ERRCODE='23503';
    END IF;
    NEW.user_id:=COALESCE(NEW.user_id,intake_row.user_id);
    NEW.workspace_id:=COALESCE(NEW.workspace_id,intake_row.workspace_id);
    NEW.product_id:=COALESCE(NEW.product_id,intake_row.product_id);
    IF NEW.user_id<>intake_row.user_id OR NEW.workspace_id<>intake_row.workspace_id
       OR NEW.product_id<>intake_row.product_id THEN
        RAISE EXCEPTION 'intake allocation owner mismatch' USING ERRCODE='23503';
    END IF;
    NEW.allocation_mode:=COALESCE(NEW.allocation_mode,'migration_existing');
    NEW.unit_snapshot:=COALESCE(NEW.unit_snapshot,batch_row.unit_snapshot);
    NEW.ingredient_profile_version_id:=COALESCE(NEW.ingredient_profile_version_id,batch_row.ingredient_profile_version_id);
    NEW.source_kind:=COALESCE(NEW.source_kind,'legacy_n1');
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER intake_allocations_prepare
BEFORE INSERT OR UPDATE ON intake_allocations
FOR EACH ROW EXECUTE FUNCTION r1_prepare_intake_allocation();

-- +goose StatementBegin
CREATE FUNCTION r1_backfill_intake_inventory_batch(batch_size integer DEFAULT 100) RETURNS boolean
LANGUAGE plpgsql AS $$
DECLARE
    state_row r1_intake_inventory_backfill_state%ROWTYPE;
    processed integer:=0;
    last_id uuid;
    observed_source_count bigint;
    observed_source_hash char(32);
BEGIN
    IF batch_size<1 OR batch_size>10000 THEN
        RAISE EXCEPTION 'intake inventory backfill batch size must be between 1 and 10000';
    END IF;
    SELECT * INTO state_row FROM r1_intake_inventory_backfill_state WHERE singleton FOR UPDATE;
    IF state_row.cycle_state='idle' THEN
        SELECT
            (SELECT count(*) FROM inventory_batches)
            +(SELECT count(*) FROM intake_records)
            +(SELECT count(*) FROM inventory_events)
            +(SELECT count(*) FROM intake_allocations),
            md5(concat_ws('|',
                (SELECT count(*) FROM inventory_batches),
                (SELECT count(*) FROM intake_records),
                (SELECT count(*) FROM inventory_events),
                (SELECT count(*) FROM intake_allocations),
                (SELECT COALESCE(max(id::text),'') FROM inventory_batches),
                (SELECT COALESCE(max(id::text),'') FROM intake_records),
                (SELECT COALESCE(max(id::text),'') FROM inventory_events)
            ))
        INTO observed_source_count,observed_source_hash;
        UPDATE r1_intake_inventory_backfill_state
        SET cycle_id=gen_random_uuid(),cycle_state='running',phase='batches',cursor_id=NULL,
            attempt_count=attempt_count+1,processed_count=0,
            source_count=observed_source_count,source_snapshot_hash=observed_source_hash,
            cycle_started_at=CURRENT_TIMESTAMP,last_progress_at=CURRENT_TIMESTAMP,
            updated_at=CURRENT_TIMESTAMP
        WHERE singleton RETURNING * INTO state_row;
    END IF;

    IF state_row.phase='batches' THEN
        WITH selected AS (
            SELECT b.id FROM inventory_batches b
            WHERE b.id>COALESCE(state_row.cursor_id,'00000000-0000-0000-0000-000000000000'::uuid)
              AND (b.aggregate_version IS NULL OR b.lifecycle_state IS NULL OR b.unit_snapshot IS NULL
                   OR b.received_quantity IS NULL OR b.received_at IS NULL OR b.expiry_precision IS NULL
                   OR b.currency IS NULL OR b.source_kind IS NULL OR b.updated_at IS NULL)
            ORDER BY b.id LIMIT batch_size
        ), changed AS (
            UPDATE inventory_batches b SET
                aggregate_version=COALESCE(b.aggregate_version,GREATEST(1,(
                    SELECT count(*) FROM inventory_events event WHERE event.batch_id=b.id
                ))),
                lifecycle_state=COALESCE(b.lifecycle_state,CASE WHEN b.current_quantity=0 THEN 'depleted'
                    WHEN b.current_quantity<b.initial_quantity THEN 'in_use' ELSE 'available_unstarted' END),
                unit_snapshot=COALESCE(b.unit_snapshot,p.unit),
                received_quantity=COALESCE(b.received_quantity,b.initial_quantity),
                received_at=COALESCE(b.received_at,b.created_at::date),
                expiry_raw_value=COALESCE(b.expiry_raw_value,b.expiry_date::text),
                expiry_precision=COALESCE(b.expiry_precision,CASE WHEN b.expiry_date IS NULL THEN 'unknown' ELSE 'day' END),
                currency=COALESCE(b.currency,'CNY'),source_kind=COALESCE(b.source_kind,'legacy_migration'),
                updated_at=COALESCE(b.updated_at,b.created_at)
            FROM selected s,products p
            WHERE b.id=s.id AND p.id=b.product_id AND p.user_id=b.user_id AND p.workspace_id=b.workspace_id
            RETURNING b.id
        ) SELECT count(*),(array_agg(id ORDER BY id DESC))[1] INTO processed,last_id FROM changed;
    ELSIF state_row.phase='intakes' THEN
        WITH selected AS (
            SELECT i.id FROM intake_records i
            WHERE i.id>COALESCE(state_row.cursor_id,'00000000-0000-0000-0000-000000000000'::uuid)
              AND (i.aggregate_version IS NULL OR i.occurred_at IS NULL OR i.occurred_time_precision IS NULL
                   OR i.iana_timezone_snapshot IS NULL OR i.history_completeness IS NULL OR i.source_kind IS NULL
                   OR NOT EXISTS (SELECT 1 FROM intake_status_facts f WHERE f.intake_id=i.id))
            ORDER BY i.id LIMIT batch_size
        ), changed AS (
            UPDATE intake_records i SET
                aggregate_version=COALESCE(i.aggregate_version,1),
                timezone_version_id=COALESCE(i.timezone_version_id,w.current_timezone_version_id),
                iana_timezone_snapshot=COALESCE(i.iana_timezone_snapshot,tz.iana_timezone,'Etc/UTC'),
                occurred_time_precision=COALESCE(i.occurred_time_precision,CASE WHEN i.intake_time IS NULL THEN 'date' ELSE 'minute' END),
                occurred_at=COALESCE(i.occurred_at,(i.intake_date+COALESCE(i.intake_time,time '00:00')) AT TIME ZONE COALESCE(tz.iana_timezone,'Etc/UTC')),
                product_profile_version_id=COALESCE(i.product_profile_version_id,p.current_product_profile_version_id),
                ingredient_profile_version_id=COALESCE(i.ingredient_profile_version_id,p.current_ingredient_profile_version_id),
                history_completeness=COALESCE(i.history_completeness,'legacy_current_only'),
                source_kind=COALESCE(i.source_kind,'legacy_migration')
            FROM selected s,products p,workspaces w
            LEFT JOIN workspace_timezone_versions tz ON tz.id=w.current_timezone_version_id
            WHERE i.id=s.id AND p.id=i.product_id AND p.user_id=i.user_id AND p.workspace_id=i.workspace_id
              AND w.id=i.workspace_id AND w.owner_user_id=i.user_id
            RETURNING i.*
        ), facts AS (
            INSERT INTO intake_status_facts (
                id,user_id,workspace_id,product_id,intake_id,business_version,status,
                effective_at,actor_user_id,source_kind,created_at
            ) SELECT gen_random_uuid(),user_id,workspace_id,product_id,id,aggregate_version,status,
                CASE WHEN status='revoked' THEN COALESCE(revoked_at,created_at) ELSE created_at END,
                user_id,'legacy_migration',created_at
              FROM changed
            ON CONFLICT (intake_id,business_version) DO NOTHING
            RETURNING intake_id
        ) SELECT count(*),(array_agg(id ORDER BY id DESC))[1] INTO processed,last_id FROM changed;
    ELSIF state_row.phase='events' THEN
        WITH selected AS (
            SELECT e.id FROM inventory_events e
            WHERE e.id>COALESCE(state_row.cursor_id,'00000000-0000-0000-0000-000000000000'::uuid)
              AND (e.ledger_kind IS NULL OR e.source_type IS NULL OR e.source_id IS NULL OR e.posted_at IS NULL
                   OR e.actor_user_id IS NULL OR e.batch_version_before IS NULL OR e.balance_after IS NULL OR e.source_kind IS NULL)
            ORDER BY e.id LIMIT batch_size
        ), calculated AS (
            SELECT e.id,
                CASE e.kind WHEN 'undo' THEN 'intake_undo' WHEN 'adjustment' THEN
                    CASE WHEN e.quantity_delta>=0 THEN 'adjustment_increase' ELSE 'adjustment_decrease' END
                    ELSE e.kind END AS target_kind,
                CASE WHEN e.kind='intake' THEN 'intake' WHEN e.kind='undo' THEN 'intake_undo'
                    WHEN e.kind IN ('opening','restock') THEN 'batch' ELSE 'inventory_event' END AS target_source_type,
                CASE WHEN e.kind IN ('intake','undo') THEN e.intake_id
                    WHEN e.kind IN ('opening','restock') THEN e.batch_id ELSE e.id END AS target_source_id,
                (SELECT count(*) FROM inventory_events prior
                 WHERE prior.batch_id=e.batch_id AND (prior.created_at,prior.id)<(e.created_at,e.id)) AS version_before,
                (SELECT COALESCE(sum(prior.quantity_delta),0) FROM inventory_events prior
                 WHERE prior.batch_id=e.batch_id AND (prior.created_at,prior.id)<=(e.created_at,e.id)) AS replay_balance,
                CASE WHEN e.kind='undo' THEN (
                    SELECT original.id FROM inventory_events original
                    WHERE original.intake_id=e.intake_id AND original.batch_id=e.batch_id AND original.kind='intake'
                    ORDER BY original.created_at,original.id LIMIT 1
                ) END AS compensation_id
            FROM inventory_events e JOIN selected s ON s.id=e.id
        ), changed AS (
            UPDATE inventory_events e SET
                ledger_kind=COALESCE(e.ledger_kind,c.target_kind),
                source_type=COALESCE(e.source_type,c.target_source_type),
                source_id=COALESCE(e.source_id,c.target_source_id),
                posted_at=COALESCE(e.posted_at,e.created_at),actor_user_id=COALESCE(e.actor_user_id,e.user_id),
                batch_version_before=COALESCE(e.batch_version_before,c.version_before),
                balance_after=COALESCE(e.balance_after,c.replay_balance),
                compensates_event_id=COALESCE(e.compensates_event_id,c.compensation_id),
                source_kind=COALESCE(e.source_kind,'legacy_migration')
            FROM calculated c WHERE e.id=c.id RETURNING e.id
        ) SELECT count(*),(array_agg(id ORDER BY id DESC))[1] INTO processed,last_id FROM changed;
    ELSE
        WITH selected_intakes AS (
            SELECT DISTINCT a.intake_id
            FROM intake_allocations a
            WHERE a.intake_id>COALESCE(state_row.cursor_id,'00000000-0000-0000-0000-000000000000'::uuid)
              AND (a.user_id IS NULL OR a.workspace_id IS NULL OR a.product_id IS NULL
                   OR a.allocation_mode IS NULL OR a.inventory_event_id IS NULL
                   OR a.batch_version_as_allocated IS NULL OR a.batch_balance_before IS NULL
                   OR a.batch_balance_after IS NULL OR a.unit_snapshot IS NULL OR a.source_kind IS NULL)
            ORDER BY a.intake_id LIMIT batch_size
        ), selected AS (
            SELECT a.intake_id,a.batch_id
            FROM intake_allocations a
            JOIN selected_intakes s ON s.intake_id=a.intake_id
        ), changed AS (
            UPDATE intake_allocations a SET
                user_id=COALESCE(a.user_id,i.user_id),workspace_id=COALESCE(a.workspace_id,i.workspace_id),
                product_id=COALESCE(a.product_id,i.product_id),allocation_mode=COALESCE(a.allocation_mode,'migration_existing'),
                inventory_event_id=COALESCE(a.inventory_event_id,e.id),
                batch_version_as_allocated=COALESCE(a.batch_version_as_allocated,e.batch_version_before),
                batch_balance_before=COALESCE(a.batch_balance_before,e.balance_after+a.quantity),
                batch_balance_after=COALESCE(a.batch_balance_after,e.balance_after),
                unit_snapshot=COALESCE(a.unit_snapshot,b.unit_snapshot),
                ingredient_profile_version_id=COALESCE(a.ingredient_profile_version_id,b.ingredient_profile_version_id),
                source_kind=COALESCE(a.source_kind,'legacy_migration')
            FROM selected s,intake_records i,inventory_batches b,inventory_events e
            WHERE a.intake_id=s.intake_id AND a.batch_id=s.batch_id
              AND i.id=a.intake_id AND b.id=a.batch_id
              AND e.intake_id=a.intake_id AND e.batch_id=a.batch_id AND e.ledger_kind='intake'
            RETURNING a.intake_id
        ) SELECT count(*),(array_agg(intake_id ORDER BY intake_id DESC))[1]
          INTO processed,last_id FROM changed;
    END IF;

    IF processed>0 THEN
        UPDATE r1_intake_inventory_backfill_state
        SET cursor_id=last_id,processed_count=processed_count+processed,
            last_progress_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
        WHERE singleton;
        RETURN false;
    END IF;
    IF state_row.phase='batches' THEN
        UPDATE r1_intake_inventory_backfill_state SET phase='intakes',cursor_id=NULL,updated_at=CURRENT_TIMESTAMP WHERE singleton;
        RETURN false;
    ELSIF state_row.phase='intakes' THEN
        UPDATE r1_intake_inventory_backfill_state SET phase='events',cursor_id=NULL,updated_at=CURRENT_TIMESTAMP WHERE singleton;
        RETURN false;
    ELSIF state_row.phase='events' THEN
        UPDATE r1_intake_inventory_backfill_state SET phase='allocations',cursor_id=NULL,updated_at=CURRENT_TIMESTAMP WHERE singleton;
        RETURN false;
    END IF;
    UPDATE r1_intake_inventory_backfill_state
    SET cycle_id=NULL,cycle_state='idle',phase='batches',cursor_id=NULL,
        last_progress_at=CURRENT_TIMESTAMP,last_completed_at=CURRENT_TIMESTAMP,
        updated_at=CURRENT_TIMESTAMP
    WHERE singleton;
    RETURN true;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_backfill_intake_inventory() RETURNS void
LANGUAGE plpgsql AS $$
DECLARE complete boolean:=false;
BEGIN
    WHILE NOT complete LOOP
        complete:=r1_backfill_intake_inventory_batch(100);
    END LOOP;
END;
$$;
-- +goose StatementEnd

SELECT r1_backfill_intake_inventory();

CREATE TRIGGER inventory_events_block_update
BEFORE UPDATE ON inventory_events
FOR EACH ROW EXECUTE FUNCTION r1_block_inventory_event_update();

ALTER TABLE intake_records
    ADD CONSTRAINT intake_records_timezone_version_fk
        FOREIGN KEY (timezone_version_id,user_id,workspace_id)
        REFERENCES workspace_timezone_versions(id,user_id,workspace_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID,
    ADD CONSTRAINT intake_records_client_action_fk
        FOREIGN KEY (workspace_id,client_action_id)
        REFERENCES client_actions(workspace_id,client_action_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID,
    ADD CONSTRAINT intake_records_occurrence_fk
        FOREIGN KEY (scheduled_occurrence_id)
        REFERENCES scheduled_occurrences(id) ON DELETE SET NULL NOT VALID,
    ADD CONSTRAINT intake_records_product_profile_fk
        FOREIGN KEY (product_profile_version_id,product_id,user_id,workspace_id)
        REFERENCES product_profile_versions(id,product_id,user_id,workspace_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID,
    ADD CONSTRAINT intake_records_ingredient_profile_fk
        FOREIGN KEY (ingredient_profile_version_id,product_id,user_id,workspace_id)
        REFERENCES ingredient_profile_versions(id,product_id,user_id,workspace_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID,
    ADD CONSTRAINT intake_records_supersedes_fk
        FOREIGN KEY (supersedes_intake_id,user_id,workspace_id,product_id)
        REFERENCES intake_records(id,user_id,workspace_id,product_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID,
    ADD CONSTRAINT intake_records_superseded_by_fk
        FOREIGN KEY (superseded_by_intake_id,user_id,workspace_id,product_id)
        REFERENCES intake_records(id,user_id,workspace_id,product_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID;

ALTER TABLE inventory_batches
    ADD CONSTRAINT inventory_batches_source_client_action_fk
        FOREIGN KEY (workspace_id,source_client_action_id)
        REFERENCES client_actions(workspace_id,client_action_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID;

ALTER TABLE inventory_events
    ADD CONSTRAINT inventory_events_batch_fk
        FOREIGN KEY (batch_id,user_id,workspace_id,product_id)
        REFERENCES inventory_batches(id,user_id,workspace_id,product_id) ON DELETE CASCADE NOT VALID,
    ADD CONSTRAINT inventory_events_client_action_fk
        FOREIGN KEY (workspace_id,client_action_id)
        REFERENCES client_actions(workspace_id,client_action_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID,
    ADD CONSTRAINT inventory_events_actor_fk
        FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE NO ACTION NOT VALID,
    ADD CONSTRAINT inventory_events_compensates_fk
        FOREIGN KEY (compensates_event_id) REFERENCES inventory_events(id) ON DELETE NO ACTION NOT VALID;

ALTER TABLE intake_allocations
    ADD CONSTRAINT intake_allocations_intake_fk
        FOREIGN KEY (intake_id,user_id,workspace_id,product_id)
        REFERENCES intake_records(id,user_id,workspace_id,product_id) ON DELETE CASCADE NOT VALID,
    ADD CONSTRAINT intake_allocations_batch_fk
        FOREIGN KEY (batch_id,user_id,workspace_id,product_id)
        REFERENCES inventory_batches(id,user_id,workspace_id,product_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID,
    ADD CONSTRAINT intake_allocations_inventory_event_fk
        FOREIGN KEY (inventory_event_id) REFERENCES inventory_events(id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID,
    ADD CONSTRAINT intake_allocations_ingredient_profile_fk
        FOREIGN KEY (ingredient_profile_version_id,product_id,user_id,workspace_id)
        REFERENCES ingredient_profile_versions(id,product_id,user_id,workspace_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED NOT VALID;

ALTER TABLE intake_status_facts
    ADD CONSTRAINT intake_status_facts_intake_fk
        FOREIGN KEY (intake_id,user_id,workspace_id,product_id)
        REFERENCES intake_records(id,user_id,workspace_id,product_id) ON DELETE CASCADE;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM inventory_batches WHERE source_kind IN ('legacy_n1','application_current'))
       OR EXISTS (SELECT 1 FROM intake_records WHERE source_kind IN ('legacy_n1','application_current'))
       OR EXISTS (SELECT 1 FROM inventory_events WHERE source_kind IN ('legacy_n1','application_current'))
       OR EXISTS (SELECT 1 FROM intake_allocations WHERE source_kind IN ('legacy_n1','application_current'))
       OR EXISTS (SELECT 1 FROM intake_status_facts WHERE source_kind IN ('legacy_n1','application_current')) THEN
        RAISE EXCEPTION 'E5 target writes exist; rollback requires a forward fix' USING ERRCODE='55000';
    END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE intake_status_facts DROP CONSTRAINT IF EXISTS intake_status_facts_intake_fk;
ALTER TABLE intake_allocations DROP CONSTRAINT IF EXISTS intake_allocations_ingredient_profile_fk;
ALTER TABLE intake_allocations DROP CONSTRAINT IF EXISTS intake_allocations_inventory_event_fk;
ALTER TABLE intake_allocations DROP CONSTRAINT IF EXISTS intake_allocations_batch_fk;
ALTER TABLE intake_allocations DROP CONSTRAINT IF EXISTS intake_allocations_intake_fk;
ALTER TABLE inventory_events DROP CONSTRAINT IF EXISTS inventory_events_compensates_fk;
ALTER TABLE inventory_events DROP CONSTRAINT IF EXISTS inventory_events_actor_fk;
ALTER TABLE inventory_events DROP CONSTRAINT IF EXISTS inventory_events_client_action_fk;
ALTER TABLE inventory_events DROP CONSTRAINT IF EXISTS inventory_events_batch_fk;
ALTER TABLE inventory_batches DROP CONSTRAINT IF EXISTS inventory_batches_source_client_action_fk;
ALTER TABLE intake_records DROP CONSTRAINT IF EXISTS intake_records_superseded_by_fk;
ALTER TABLE intake_records DROP CONSTRAINT IF EXISTS intake_records_supersedes_fk;
ALTER TABLE intake_records DROP CONSTRAINT IF EXISTS intake_records_ingredient_profile_fk;
ALTER TABLE intake_records DROP CONSTRAINT IF EXISTS intake_records_product_profile_fk;
ALTER TABLE intake_records DROP CONSTRAINT IF EXISTS intake_records_occurrence_fk;
ALTER TABLE intake_records DROP CONSTRAINT IF EXISTS intake_records_client_action_fk;
ALTER TABLE intake_records DROP CONSTRAINT IF EXISTS intake_records_timezone_version_fk;

ALTER TABLE inventory_batches
    DROP CONSTRAINT IF EXISTS inventory_batches_ingredient_profile_fk,
    ADD CONSTRAINT inventory_batches_ingredient_profile_fk
        FOREIGN KEY (ingredient_profile_version_id,product_id,user_id,workspace_id)
        REFERENCES ingredient_profile_versions(id,product_id,user_id,workspace_id)
        ON DELETE RESTRICT NOT VALID;

ALTER TABLE intake_allocations
    DROP CONSTRAINT IF EXISTS intake_allocations_batch_id_fkey,
    ADD CONSTRAINT intake_allocations_batch_id_fkey
        FOREIGN KEY (batch_id) REFERENCES inventory_batches(id);

DROP TRIGGER IF EXISTS intake_allocations_prepare ON intake_allocations;
DROP TRIGGER IF EXISTS inventory_events_block_update ON inventory_events;
DROP TRIGGER IF EXISTS inventory_events_prepare ON inventory_events;
DROP TRIGGER IF EXISTS intake_status_facts_block_change ON intake_status_facts;
DROP TRIGGER IF EXISTS intake_records_status_fact ON intake_records;
DROP TRIGGER IF EXISTS intake_records_prepare ON intake_records;
DROP TRIGGER IF EXISTS inventory_batches_prepare ON inventory_batches;
DROP FUNCTION IF EXISTS r1_prepare_intake_allocation();
DROP FUNCTION IF EXISTS r1_block_inventory_event_update();
DROP FUNCTION IF EXISTS r1_prepare_inventory_event();
DROP FUNCTION IF EXISTS r1_block_intake_status_fact_change();
DROP FUNCTION IF EXISTS r1_append_intake_status_fact();
DROP FUNCTION IF EXISTS r1_prepare_intake_record();
DROP FUNCTION IF EXISTS r1_prepare_inventory_batch();
DROP FUNCTION IF EXISTS r1_backfill_intake_inventory();
DROP FUNCTION IF EXISTS r1_backfill_intake_inventory_batch(integer);
DROP TABLE IF EXISTS r1_intake_inventory_backfill_state;
DROP TABLE IF EXISTS intake_status_facts;

DROP INDEX IF EXISTS intake_allocations_inventory_event_uidx;
ALTER TABLE intake_allocations
    DROP COLUMN IF EXISTS source_kind,
    DROP COLUMN IF EXISTS ingredient_profile_version_id,
    DROP COLUMN IF EXISTS unit_snapshot,
    DROP COLUMN IF EXISTS batch_balance_after,
    DROP COLUMN IF EXISTS batch_balance_before,
    DROP COLUMN IF EXISTS batch_version_as_allocated,
    DROP COLUMN IF EXISTS inventory_event_id,
    DROP COLUMN IF EXISTS allocation_mode,
    DROP COLUMN IF EXISTS product_id,
    DROP COLUMN IF EXISTS workspace_id,
    DROP COLUMN IF EXISTS user_id;

DROP INDEX IF EXISTS inventory_events_batch_posted_idx;
DROP INDEX IF EXISTS inventory_events_compensation_uidx;
DROP INDEX IF EXISTS inventory_events_source_identity_uidx;
ALTER TABLE inventory_events
    DROP COLUMN IF EXISTS source_kind,
    DROP COLUMN IF EXISTS compensates_event_id,
    DROP COLUMN IF EXISTS balance_after,
    DROP COLUMN IF EXISTS batch_version_before,
    DROP COLUMN IF EXISTS actor_user_id,
    DROP COLUMN IF EXISTS note,
    DROP COLUMN IF EXISTS reason_code,
    DROP COLUMN IF EXISTS occurred_at,
    DROP COLUMN IF EXISTS posted_at,
    DROP COLUMN IF EXISTS request_hash,
    DROP COLUMN IF EXISTS client_action_id,
    DROP COLUMN IF EXISTS source_request_key,
    DROP COLUMN IF EXISTS source_id,
    DROP COLUMN IF EXISTS source_type,
    DROP COLUMN IF EXISTS ledger_kind;

DROP INDEX IF EXISTS intake_records_tenant_product_identity_uidx;
DROP INDEX IF EXISTS scheduled_occurrences_tenant_product_identity_uidx;
ALTER TABLE intake_records
    DROP COLUMN IF EXISTS source_kind,
    DROP COLUMN IF EXISTS history_completeness,
    DROP COLUMN IF EXISTS superseded_by_intake_id,
    DROP COLUMN IF EXISTS supersedes_intake_id,
    DROP COLUMN IF EXISTS ingredient_profile_version_id,
    DROP COLUMN IF EXISTS product_profile_version_id,
    DROP COLUMN IF EXISTS iana_timezone_snapshot,
    DROP COLUMN IF EXISTS timezone_version_id,
    DROP COLUMN IF EXISTS occurred_time_precision,
    DROP COLUMN IF EXISTS occurred_at,
    DROP COLUMN IF EXISTS scheduled_occurrence_id,
    DROP COLUMN IF EXISTS request_hash,
    DROP COLUMN IF EXISTS client_action_id,
    DROP COLUMN IF EXISTS aggregate_version;

DROP INDEX IF EXISTS inventory_batches_source_request_uidx;
DROP INDEX IF EXISTS inventory_batches_tenant_product_identity_uidx;
ALTER TABLE inventory_batches
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS source_kind,
    DROP COLUMN IF EXISTS request_hash,
    DROP COLUMN IF EXISTS source_request_key,
    DROP COLUMN IF EXISTS source_client_action_id,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS opened_at,
    DROP COLUMN IF EXISTS expiry_precision,
    DROP COLUMN IF EXISTS expiry_raw_value,
    DROP COLUMN IF EXISTS lot_code,
    DROP COLUMN IF EXISTS received_at,
    DROP COLUMN IF EXISTS received_quantity,
    DROP COLUMN IF EXISTS unit_snapshot,
    DROP COLUMN IF EXISTS lifecycle_state,
    DROP COLUMN IF EXISTS aggregate_version;

ALTER TABLE inventory_batches
    ADD CONSTRAINT inventory_batches_current_quantity_check
    CHECK (current_quantity >= 0 AND current_quantity <= initial_quantity);
