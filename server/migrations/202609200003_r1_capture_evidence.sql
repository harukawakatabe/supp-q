-- +goose Up
CREATE UNIQUE INDEX recognition_sets_tenant_identity_uidx
    ON recognition_sets (id, user_id, workspace_id);
CREATE UNIQUE INDEX recognition_jobs_tenant_identity_uidx
    ON recognition_jobs (id, user_id, workspace_id);

CREATE TABLE capture_drafts (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    source_recognition_set_id uuid REFERENCES recognition_sets(id) ON DELETE SET NULL,
    status text NOT NULL CHECK (status IN ('draft','processing','awaiting_confirmation','confirmed','cancelled')),
    aggregate_version bigint NOT NULL DEFAULT 1 CHECK (aggregate_version > 0),
    current_confirmation_draft_id uuid,
    confirmed_product_id uuid,
    source text NOT NULL CHECK (source IN ('legacy_migration','legacy_compat','target_application')),
    history_completeness text NOT NULL CHECK (history_completeness IN ('complete','legacy_current_only')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    confirmed_at timestamptz,
    cancelled_at timestamptz,
    UNIQUE (source_recognition_set_id),
    UNIQUE (id, user_id, workspace_id),
    FOREIGN KEY (confirmed_product_id, user_id, workspace_id)
        REFERENCES products (id, user_id, workspace_id)
        ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED,
    CHECK ((status='confirmed' AND confirmed_product_id IS NOT NULL AND confirmed_at IS NOT NULL)
        OR status<>'confirmed'),
    CHECK ((status='cancelled' AND cancelled_at IS NOT NULL) OR status<>'cancelled')
);

CREATE INDEX capture_drafts_owner_status_idx
    ON capture_drafts (user_id, workspace_id, status, updated_at DESC);
CREATE UNIQUE INDEX capture_drafts_one_target_active_uidx
    ON capture_drafts (workspace_id)
    WHERE source='target_application' AND status IN ('draft','processing','awaiting_confirmation');

CREATE TABLE capture_slots (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    capture_draft_id uuid NOT NULL,
    role text NOT NULL CHECK (role IN ('front','facts','expiry')),
    current_slot_version_id uuid,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (capture_draft_id, role),
    UNIQUE (id, capture_draft_id, user_id, workspace_id),
    FOREIGN KEY (capture_draft_id, user_id, workspace_id)
        REFERENCES capture_drafts (id, user_id, workspace_id) ON DELETE CASCADE
);

CREATE TABLE capture_slot_versions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    capture_draft_id uuid NOT NULL,
    capture_slot_id uuid NOT NULL,
    evidence_version integer NOT NULL CHECK (evidence_version > 0),
    input_kind text NOT NULL CHECK (input_kind IN ('upload','skipped','manual')),
    processing_state text NOT NULL CHECK (processing_state IN (
        'queued','running','succeeded','partial','failed','skipped','cancelled','stale'
    )),
    file_id uuid,
    legacy_recognition_job_id uuid,
    source text NOT NULL CHECK (source IN ('legacy_migration','legacy_compat','target_application')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    superseded_at timestamptz,
    UNIQUE (capture_slot_id, evidence_version),
    UNIQUE (legacy_recognition_job_id),
    UNIQUE (id, user_id, workspace_id),
    UNIQUE (id, capture_draft_id, user_id, workspace_id),
    UNIQUE (id, file_id, capture_draft_id, user_id, workspace_id),
    UNIQUE (id, capture_slot_id, capture_draft_id, user_id, workspace_id),
    FOREIGN KEY (capture_slot_id, capture_draft_id, user_id, workspace_id)
        REFERENCES capture_slots (id, capture_draft_id, user_id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (file_id, user_id, workspace_id)
        REFERENCES files (id, user_id, workspace_id) ON DELETE RESTRICT,
    FOREIGN KEY (legacy_recognition_job_id, user_id, workspace_id)
        REFERENCES recognition_jobs (id, user_id, workspace_id)
        ON DELETE SET NULL (legacy_recognition_job_id),
    CHECK ((input_kind='upload' AND file_id IS NOT NULL)
        OR (input_kind IN ('skipped','manual') AND file_id IS NULL AND legacy_recognition_job_id IS NULL)),
    CHECK ((processing_state='stale' AND superseded_at IS NOT NULL) OR processing_state<>'stale')
);

ALTER TABLE capture_slots
    ADD CONSTRAINT capture_slots_current_version_fk
    FOREIGN KEY (current_slot_version_id, id, capture_draft_id, user_id, workspace_id)
    REFERENCES capture_slot_versions (id, capture_slot_id, capture_draft_id, user_id, workspace_id)
    ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;

CREATE INDEX capture_slot_versions_draft_idx
    ON capture_slot_versions (user_id, workspace_id, capture_draft_id, capture_slot_id, evidence_version DESC);

-- Target recognition is version-bound. The legacy queue is only an optional
-- compatibility source and is not the target job identity.
CREATE TABLE capture_recognition_jobs (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    capture_draft_id uuid NOT NULL,
    capture_slot_version_id uuid NOT NULL,
    file_id uuid NOT NULL,
    legacy_recognition_job_id uuid,
    status text NOT NULL CHECK (status IN ('queued','running','succeeded','partial','failed','cancelled','stale')),
    current_attempt integer NOT NULL DEFAULT 0 CHECK (current_attempt BETWEEN 0 AND 20),
    max_attempts integer NOT NULL DEFAULT 3 CHECK (max_attempts BETWEEN 1 AND 20),
    provider text NOT NULL DEFAULT 'pending' CHECK (char_length(provider) BETWEEN 1 AND 120),
    source text NOT NULL CHECK (source IN ('legacy_migration','legacy_compat','target_application')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (capture_slot_version_id),
    UNIQUE (legacy_recognition_job_id),
    UNIQUE (id, capture_slot_version_id, capture_draft_id, user_id, workspace_id),
    UNIQUE (id, file_id, capture_slot_version_id, capture_draft_id, user_id, workspace_id),
    FOREIGN KEY (capture_slot_version_id, file_id, capture_draft_id, user_id, workspace_id)
        REFERENCES capture_slot_versions (id, file_id, capture_draft_id, user_id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (file_id, user_id, workspace_id)
        REFERENCES files (id, user_id, workspace_id) ON DELETE RESTRICT,
    FOREIGN KEY (legacy_recognition_job_id, user_id, workspace_id)
        REFERENCES recognition_jobs (id, user_id, workspace_id)
        ON DELETE SET NULL (legacy_recognition_job_id)
);

CREATE INDEX capture_recognition_jobs_claim_idx
    ON capture_recognition_jobs (status, updated_at, created_at)
    WHERE status IN ('queued','running');

CREATE TABLE capture_recognition_attempts (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    capture_draft_id uuid NOT NULL,
    capture_slot_version_id uuid NOT NULL,
    capture_recognition_job_id uuid NOT NULL,
    attempt_number integer NOT NULL CHECK (attempt_number BETWEEN 1 AND 20),
    status text NOT NULL CHECK (status IN ('running','succeeded','partial','failed','cancelled','superseded')),
    provider text NOT NULL CHECK (char_length(provider) BETWEEN 1 AND 120),
    lease_until timestamptz,
    started_at timestamptz,
    completed_at timestamptz,
    source text NOT NULL CHECK (source IN ('legacy_migration','legacy_compat','target_application')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (capture_recognition_job_id, attempt_number),
    UNIQUE (capture_recognition_job_id, attempt_number, capture_slot_version_id, capture_draft_id, user_id, workspace_id),
    FOREIGN KEY (capture_recognition_job_id, capture_slot_version_id, capture_draft_id, user_id, workspace_id)
        REFERENCES capture_recognition_jobs (id, capture_slot_version_id, capture_draft_id, user_id, workspace_id) ON DELETE CASCADE
);

CREATE TABLE file_links (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    file_id uuid NOT NULL,
    capture_draft_id uuid NOT NULL,
    capture_slot_version_id uuid NOT NULL,
    product_id uuid,
    purpose text NOT NULL CHECK (purpose IN ('capture_slot_source','confirmed_product_source')),
    link_state text NOT NULL CHECK (link_state IN ('active','released')),
    source text NOT NULL CHECK (source IN ('legacy_migration','legacy_compat','target_application')),
    created_at timestamptz NOT NULL,
    released_at timestamptz,
    UNIQUE (capture_slot_version_id, purpose),
    FOREIGN KEY (file_id, user_id, workspace_id)
        REFERENCES files (id, user_id, workspace_id) ON DELETE RESTRICT,
    FOREIGN KEY (capture_draft_id, user_id, workspace_id)
        REFERENCES capture_drafts (id, user_id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (capture_slot_version_id, capture_draft_id, user_id, workspace_id)
        REFERENCES capture_slot_versions (id, capture_draft_id, user_id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (product_id, user_id, workspace_id)
        REFERENCES products (id, user_id, workspace_id) ON DELETE CASCADE,
    CHECK ((purpose='capture_slot_source' AND product_id IS NULL)
        OR (purpose='confirmed_product_source' AND product_id IS NOT NULL)),
    CHECK ((link_state='released' AND released_at IS NOT NULL)
        OR (link_state='active' AND released_at IS NULL))
);

CREATE INDEX file_links_file_state_idx
    ON file_links (user_id, workspace_id, file_id, link_state);

CREATE TABLE recognition_evidence (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    capture_draft_id uuid NOT NULL,
    capture_slot_version_id uuid NOT NULL,
    capture_recognition_job_id uuid NOT NULL,
    legacy_recognition_job_id uuid,
    job_attempt integer NOT NULL CHECK (job_attempt > 0),
    file_id uuid NOT NULL,
    stage text NOT NULL CHECK (stage='ocr_transcript'),
    raw_text text NOT NULL CHECK (char_length(raw_text) <= 30000),
    provider text NOT NULL CHECK (char_length(provider) BETWEEN 1 AND 120),
    model text NOT NULL CHECK (char_length(model) BETWEEN 1 AND 160),
    duration_ms integer CHECK (duration_ms BETWEEN 0 AND 600000),
    completed_at timestamptz NOT NULL,
    source text NOT NULL CHECK (source IN ('legacy_migration','legacy_compat','target_application')),
    created_at timestamptz NOT NULL,
    UNIQUE (capture_recognition_job_id, job_attempt, stage),
    FOREIGN KEY (capture_recognition_job_id, job_attempt, capture_slot_version_id, capture_draft_id, user_id, workspace_id)
        REFERENCES capture_recognition_attempts (capture_recognition_job_id, attempt_number, capture_slot_version_id, capture_draft_id, user_id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (capture_recognition_job_id, file_id, capture_slot_version_id, capture_draft_id, user_id, workspace_id)
        REFERENCES capture_recognition_jobs (id, file_id, capture_slot_version_id, capture_draft_id, user_id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (file_id, user_id, workspace_id)
        REFERENCES files (id, user_id, workspace_id) ON DELETE RESTRICT,
    FOREIGN KEY (legacy_recognition_job_id, user_id, workspace_id)
        REFERENCES recognition_jobs (id, user_id, workspace_id)
        ON DELETE SET NULL (legacy_recognition_job_id)
);

CREATE TABLE recognition_candidates (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    capture_draft_id uuid NOT NULL,
    capture_slot_version_id uuid NOT NULL,
    capture_recognition_job_id uuid NOT NULL,
    legacy_recognition_job_id uuid,
    job_attempt integer NOT NULL CHECK (job_attempt > 0),
    candidate_kind text NOT NULL DEFAULT 'structured' CHECK (candidate_kind='structured'),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload)='object'),
    trace jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(trace)='object'),
    confidence numeric(5,4) NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    provider text NOT NULL CHECK (char_length(provider) BETWEEN 1 AND 120),
    completed_at timestamptz NOT NULL,
    source text NOT NULL CHECK (source IN ('legacy_migration','legacy_compat','target_application')),
    created_at timestamptz NOT NULL,
    UNIQUE (capture_recognition_job_id, job_attempt, candidate_kind),
    UNIQUE (id, capture_slot_version_id, capture_draft_id, user_id, workspace_id),
    FOREIGN KEY (capture_recognition_job_id, job_attempt, capture_slot_version_id, capture_draft_id, user_id, workspace_id)
        REFERENCES capture_recognition_attempts (capture_recognition_job_id, attempt_number, capture_slot_version_id, capture_draft_id, user_id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (legacy_recognition_job_id, user_id, workspace_id)
        REFERENCES recognition_jobs (id, user_id, workspace_id)
        ON DELETE SET NULL (legacy_recognition_job_id)
);

CREATE TABLE confirmation_drafts (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    capture_draft_id uuid NOT NULL,
    draft_version integer NOT NULL CHECK (draft_version > 0),
    expected_aggregate_version bigint NOT NULL CHECK (expected_aggregate_version > 0),
    candidate_payload jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(candidate_payload)='object'),
    candidate_refs jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(candidate_refs)='object'),
    user_payload jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(user_payload)='object'),
    field_sources jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(field_sources)='object'),
    status text NOT NULL CHECK (status IN ('editing','confirmed','superseded')),
    source text NOT NULL CHECK (source IN ('legacy_migration','legacy_compat','target_application')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    confirmed_at timestamptz,
    UNIQUE (capture_draft_id, draft_version),
    UNIQUE (id, capture_draft_id, user_id, workspace_id),
    FOREIGN KEY (capture_draft_id, user_id, workspace_id)
        REFERENCES capture_drafts (id, user_id, workspace_id) ON DELETE CASCADE,
    CHECK ((status='confirmed' AND confirmed_at IS NOT NULL) OR status<>'confirmed')
);

ALTER TABLE capture_drafts
    ADD CONSTRAINT capture_drafts_current_confirmation_fk
    FOREIGN KEY (current_confirmation_draft_id, id, user_id, workspace_id)
    REFERENCES confirmation_drafts (id, capture_draft_id, user_id, workspace_id)
    ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE recognition_sets ADD COLUMN capture_draft_id uuid;
ALTER TABLE recognition_jobs ADD COLUMN capture_slot_version_id uuid;
ALTER TABLE recognition_sets
    ADD CONSTRAINT recognition_sets_capture_draft_fk
    FOREIGN KEY (capture_draft_id, user_id, workspace_id)
    REFERENCES capture_drafts (id, user_id, workspace_id)
    ON DELETE SET NULL (capture_draft_id) DEFERRABLE INITIALLY DEFERRED NOT VALID;
ALTER TABLE recognition_jobs
    ADD CONSTRAINT recognition_jobs_capture_slot_version_fk
    FOREIGN KEY (capture_slot_version_id, user_id, workspace_id)
    REFERENCES capture_slot_versions (id, user_id, workspace_id)
    ON DELETE SET NULL (capture_slot_version_id) DEFERRABLE INITIALLY DEFERRED NOT VALID;

-- +goose StatementBegin
CREATE FUNCTION r1_block_capture_history_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' AND pg_trigger_depth()>1 THEN RETURN OLD; END IF;
    RAISE EXCEPTION 'capture evidence history is immutable' USING ERRCODE='23514';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER recognition_evidence_immutable
    BEFORE UPDATE OR DELETE ON recognition_evidence
    FOR EACH ROW EXECUTE FUNCTION r1_block_capture_history_mutation();
CREATE TRIGGER recognition_candidates_immutable
    BEFORE UPDATE OR DELETE ON recognition_candidates
    FOR EACH ROW EXECUTE FUNCTION r1_block_capture_history_mutation();

-- +goose StatementBegin
CREATE FUNCTION r1_validate_capture_slot_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        IF pg_trigger_depth()>1 THEN RETURN OLD; END IF;
        RAISE EXCEPTION 'capture slot version history is immutable' USING ERRCODE='23514';
    END IF;
    IF (to_jsonb(NEW)-ARRAY['processing_state','updated_at','superseded_at'])
       IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['processing_state','updated_at','superseded_at']) THEN
        RAISE EXCEPTION 'capture slot version body is immutable' USING ERRCODE='23514';
    END IF;
    IF OLD.processing_state='stale' AND NEW.processing_state<>'stale' THEN
        RAISE EXCEPTION 'stale capture slot version cannot be reactivated' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER capture_slot_versions_validate
    BEFORE UPDATE OR DELETE ON capture_slot_versions
    FOR EACH ROW EXECUTE FUNCTION r1_validate_capture_slot_version();

-- +goose StatementBegin
CREATE FUNCTION r1_validate_capture_recognition_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        IF pg_trigger_depth()>1 THEN RETURN OLD; END IF;
        RAISE EXCEPTION 'capture recognition history cannot be deleted directly' USING ERRCODE='23514';
    END IF;
    IF TG_TABLE_NAME='capture_recognition_jobs' THEN
        IF (to_jsonb(NEW)-ARRAY['status','current_attempt','max_attempts','provider','updated_at'])
           IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['status','current_attempt','max_attempts','provider','updated_at']) THEN
            RAISE EXCEPTION 'capture recognition job binding is immutable' USING ERRCODE='23514';
        END IF;
    ELSE
        IF (to_jsonb(NEW)-ARRAY['status','provider','lease_until','started_at','completed_at','updated_at'])
           IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['status','provider','lease_until','started_at','completed_at','updated_at']) THEN
            RAISE EXCEPTION 'capture recognition attempt binding is immutable' USING ERRCODE='23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER capture_recognition_jobs_validate
    BEFORE UPDATE OR DELETE ON capture_recognition_jobs
    FOR EACH ROW EXECUTE FUNCTION r1_validate_capture_recognition_identity();
CREATE TRIGGER capture_recognition_attempts_validate
    BEFORE UPDATE OR DELETE ON capture_recognition_attempts
    FOR EACH ROW EXECUTE FUNCTION r1_validate_capture_recognition_identity();

-- +goose StatementBegin
CREATE FUNCTION r1_validate_file_link() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE file_state text;
BEGIN
    IF TG_OP='DELETE' THEN
        IF pg_trigger_depth()>1 THEN RETURN OLD; END IF;
        RAISE EXCEPTION 'file link history must be released, not deleted' USING ERRCODE='23514';
    END IF;
    IF TG_OP='UPDATE' AND (to_jsonb(NEW)-ARRAY['link_state','released_at'])
       IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['link_state','released_at']) THEN
        RAISE EXCEPTION 'file link binding is immutable' USING ERRCODE='23514';
    END IF;
    IF TG_OP='UPDATE' AND OLD.link_state='released' AND NEW.link_state<>'released' THEN
        RAISE EXCEPTION 'released file link cannot be reactivated' USING ERRCODE='23514';
    END IF;
    IF NEW.link_state='active' THEN
        SELECT status INTO file_state FROM files
        WHERE id=NEW.file_id AND user_id=NEW.user_id AND workspace_id=NEW.workspace_id;
        IF file_state IS DISTINCT FROM 'active' THEN
            RAISE EXCEPTION 'active file link requires active file' USING ERRCODE='23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER file_links_validate
    BEFORE INSERT OR UPDATE OR DELETE ON file_links
    FOR EACH ROW EXECUTE FUNCTION r1_validate_file_link();

-- +goose StatementBegin
CREATE FUNCTION r1_prevent_linked_file_delete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status='active' AND NEW.status='deleted'
       AND EXISTS (SELECT 1 FROM file_links WHERE file_id=OLD.id AND link_state='active') THEN
        RAISE EXCEPTION 'active file links must be released before deleting a file' USING ERRCODE='23503';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER files_prevent_linked_delete
    BEFORE UPDATE OF status ON files
    FOR EACH ROW EXECUTE FUNCTION r1_prevent_linked_file_delete();

CREATE TABLE r1_capture_backfill_state (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    cycle_id uuid,
    cycle_state text NOT NULL DEFAULT 'idle' CHECK (cycle_state IN ('idle','running','paused','failed')),
    cursor_recognition_set_id uuid,
    pause_requested boolean NOT NULL DEFAULT false,
    attempt_count bigint NOT NULL DEFAULT 0 CHECK (attempt_count>=0),
    processed_count bigint NOT NULL DEFAULT 0 CHECK (processed_count>=0),
    mapped_count bigint NOT NULL DEFAULT 0 CHECK (mapped_count>=0),
    unchanged_count bigint NOT NULL DEFAULT 0 CHECK (unchanged_count>=0),
    quarantined_count bigint NOT NULL DEFAULT 0 CHECK (quarantined_count>=0),
    source_count bigint NOT NULL DEFAULT 0 CHECK (source_count>=0),
    source_snapshot_hash char(32),
    last_error text NOT NULL DEFAULT '',
    cycle_started_at timestamptz,
    cycle_completed_at timestamptz,
    last_progress_at timestamptz,
    updated_at timestamptz NOT NULL
);
INSERT INTO r1_capture_backfill_state (singleton,updated_at) VALUES (true,CURRENT_TIMESTAMP);

-- Map one legacy recognition-set snapshot. Empty roles stay empty; no file,
-- evidence, candidate, or attempt is fabricated for them.
-- +goose StatementBegin
CREATE FUNCTION r1_sync_capture_compat_set(source_set_id uuid, row_source text)
RETURNS uuid LANGUAGE plpgsql AS $$
DECLARE
    source_set record;
    source_item record;
    draft_id uuid;
    confirmation_id uuid;
    slot_id uuid;
    version_id uuid;
    target_job_id uuid;
    target_attempt integer;
    candidate_id uuid;
    slot_role text;
    target_status text;
    attempt_status text;
    merged_payload jsonb := '{}'::jsonb;
    merged_refs jsonb := '{}'::jsonb;
    same_tenant_product boolean := false;
    draft_created boolean := false;
    draft_was_active boolean := false;
BEGIN
    IF row_source NOT IN ('legacy_migration','legacy_compat') THEN
        RAISE EXCEPTION 'invalid capture compatibility source' USING ERRCODE='22023';
    END IF;
    SELECT rs.* INTO source_set FROM recognition_sets rs WHERE rs.id=source_set_id FOR UPDATE;
    IF NOT FOUND THEN RETURN NULL; END IF;

    IF source_set.product_id IS NOT NULL THEN
        SELECT EXISTS(
            SELECT 1 FROM products p
            WHERE p.id=source_set.product_id AND p.user_id=source_set.user_id
              AND p.workspace_id=source_set.workspace_id AND p.product_type='supplement'
        ) INTO same_tenant_product;
    END IF;
    target_status := CASE
        WHEN source_set.status='confirmed' AND same_tenant_product THEN 'confirmed'
        WHEN source_set.status='confirmed' THEN 'awaiting_confirmation'
        ELSE source_set.status
    END;

    SELECT id,current_confirmation_draft_id,
           status IN ('draft','processing','awaiting_confirmation')
    INTO draft_id,confirmation_id,draft_was_active
    FROM capture_drafts WHERE source_recognition_set_id=source_set.id;
    IF draft_id IS NULL THEN
        draft_created := true;
        draft_id := gen_random_uuid();
        confirmation_id := gen_random_uuid();
        INSERT INTO capture_drafts (
            id,user_id,workspace_id,source_recognition_set_id,status,aggregate_version,
            confirmed_product_id,source,history_completeness,created_at,updated_at,
            confirmed_at,cancelled_at
        ) VALUES (
            draft_id,source_set.user_id,source_set.workspace_id,source_set.id,target_status,1,
            CASE WHEN target_status='confirmed' THEN source_set.product_id END,row_source,
            'legacy_current_only',source_set.created_at,source_set.updated_at,
            CASE WHEN target_status='confirmed' THEN COALESCE(source_set.confirmed_at,source_set.updated_at) END,
            CASE WHEN target_status='cancelled' THEN source_set.updated_at END
        );
        INSERT INTO confirmation_drafts (
            id,user_id,workspace_id,capture_draft_id,draft_version,expected_aggregate_version,
            candidate_payload,candidate_refs,user_payload,field_sources,status,source,
            created_at,updated_at,confirmed_at
        ) VALUES (
            confirmation_id,source_set.user_id,source_set.workspace_id,draft_id,1,1,
            '{}'::jsonb,'{}'::jsonb,COALESCE(source_set.confirmed_payload,'{}'::jsonb),'{}'::jsonb,
            CASE WHEN target_status='confirmed' THEN 'confirmed' ELSE 'editing' END,row_source,
            source_set.created_at,source_set.updated_at,
            CASE WHEN target_status='confirmed' THEN COALESCE(source_set.confirmed_at,source_set.updated_at) END
        );
        UPDATE capture_drafts SET current_confirmation_draft_id=confirmation_id WHERE id=draft_id;
    ELSIF draft_was_active THEN
        UPDATE capture_drafts SET
            status=target_status,
            confirmed_product_id=CASE WHEN target_status='confirmed' THEN source_set.product_id END,
            confirmed_at=CASE WHEN target_status='confirmed' THEN COALESCE(source_set.confirmed_at,source_set.updated_at) END,
            cancelled_at=CASE WHEN target_status='cancelled' THEN source_set.updated_at END,
            updated_at=source_set.updated_at
        WHERE id=draft_id;
        IF target_status='confirmed' THEN
            UPDATE confirmation_drafts SET status='confirmed',
                confirmed_at=COALESCE(source_set.confirmed_at,source_set.updated_at),updated_at=source_set.updated_at
            WHERE id=confirmation_id AND status='editing';
        END IF;
    END IF;

    UPDATE recognition_sets SET capture_draft_id=draft_id WHERE id=source_set.id;

    FOREACH slot_role IN ARRAY ARRAY['front','facts','expiry'] LOOP
        SELECT id INTO slot_id FROM capture_slots
        WHERE capture_draft_id=draft_id AND role=slot_role;
        IF slot_id IS NULL THEN
            slot_id := gen_random_uuid();
            INSERT INTO capture_slots (
                id,user_id,workspace_id,capture_draft_id,role,created_at,updated_at
            ) VALUES (
                slot_id,source_set.user_id,source_set.workspace_id,draft_id,slot_role,
                source_set.created_at,source_set.updated_at
            );
        END IF;

        source_item := NULL;
        SELECT f.id AS file_id,f.status AS file_status,j.id AS job_id,j.status AS job_status,
               j.attempt,j.max_attempts,j.result,j.trace,j.confidence,j.provider,j.ocr_text,
               j.ocr_provider,j.ocr_model,j.ocr_duration_ms,j.ocr_completed_at,j.completed_at,
               j.lease_until,j.started_at,j.created_at,j.updated_at
        INTO source_item
        FROM recognition_files rf
        JOIN files f ON f.id=rf.file_id
        JOIN recognition_jobs j ON j.recognition_set_id=rf.recognition_set_id
             AND j.file_id=rf.file_id AND j.role=rf.role
        WHERE rf.recognition_set_id=source_set.id AND rf.role=slot_role
          AND f.user_id=source_set.user_id AND f.workspace_id=source_set.workspace_id
          AND j.user_id=source_set.user_id AND j.workspace_id=source_set.workspace_id;

        IF source_item.job_id IS NULL THEN
            IF EXISTS (SELECT 1 FROM recognition_files rf WHERE rf.recognition_set_id=source_set.id AND rf.role=slot_role)
               OR EXISTS (SELECT 1 FROM recognition_jobs j WHERE j.recognition_set_id=source_set.id AND j.role=slot_role) THEN
                INSERT INTO migration_quarantines (
                    source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
                ) VALUES (
                    'recognition_sets',source_set.id,'capture_role_file_job_mismatch',
                    source_set.user_id,source_set.workspace_id,'open',CURRENT_TIMESTAMP
                ) ON CONFLICT DO NOTHING;
            END IF;
            CONTINUE;
        END IF;

        SELECT id INTO version_id FROM capture_slot_versions
        WHERE legacy_recognition_job_id=source_item.job_id;
        IF version_id IS NULL THEN
            version_id := gen_random_uuid();
            INSERT INTO capture_slot_versions (
                id,user_id,workspace_id,capture_draft_id,capture_slot_id,evidence_version,
                input_kind,processing_state,file_id,legacy_recognition_job_id,source,created_at,updated_at
            ) VALUES (
                version_id,source_set.user_id,source_set.workspace_id,draft_id,slot_id,1,
                'upload',source_item.job_status,source_item.file_id,source_item.job_id,row_source,
                source_item.created_at,source_item.updated_at
            );
            UPDATE capture_slots SET current_slot_version_id=version_id,updated_at=source_item.updated_at
            WHERE id=slot_id AND current_slot_version_id IS NULL;
        END IF;
        UPDATE recognition_jobs SET capture_slot_version_id=version_id WHERE id=source_item.job_id;

        SELECT id INTO target_job_id FROM capture_recognition_jobs
        WHERE legacy_recognition_job_id=source_item.job_id;
        IF target_job_id IS NULL THEN
            target_job_id := gen_random_uuid();
            INSERT INTO capture_recognition_jobs (
                id,user_id,workspace_id,capture_draft_id,capture_slot_version_id,file_id,
                legacy_recognition_job_id,status,current_attempt,max_attempts,provider,source,created_at,updated_at
            ) VALUES (
                target_job_id,source_set.user_id,source_set.workspace_id,draft_id,version_id,source_item.file_id,
                source_item.job_id,source_item.job_status,source_item.attempt,source_item.max_attempts,
                COALESCE(NULLIF(source_item.provider,''),'pending'),row_source,source_item.created_at,source_item.updated_at
            );
        ELSE
            UPDATE capture_recognition_jobs SET status=source_item.job_status,
                current_attempt=source_item.attempt,max_attempts=source_item.max_attempts,
                provider=COALESCE(NULLIF(source_item.provider,''),'pending'),updated_at=source_item.updated_at
            WHERE id=target_job_id;
        END IF;

        target_attempt := CASE
            WHEN source_item.attempt>0 THEN source_item.attempt
            WHEN source_item.ocr_completed_at IS NOT NULL OR source_item.completed_at IS NOT NULL THEN 1
            ELSE 0
        END;
        IF target_attempt>0 THEN
            attempt_status := CASE source_item.job_status
                WHEN 'running' THEN 'running'
                WHEN 'succeeded' THEN 'succeeded'
                WHEN 'partial' THEN 'partial'
                WHEN 'failed' THEN 'failed'
                WHEN 'cancelled' THEN 'cancelled'
                ELSE 'superseded'
            END;
            INSERT INTO capture_recognition_attempts (
                id,user_id,workspace_id,capture_draft_id,capture_slot_version_id,
                capture_recognition_job_id,attempt_number,status,provider,lease_until,
                started_at,completed_at,source,created_at,updated_at
            ) VALUES (
                gen_random_uuid(),source_set.user_id,source_set.workspace_id,draft_id,version_id,
                target_job_id,target_attempt,attempt_status,
                COALESCE(NULLIF(source_item.provider,''),'pending'),source_item.lease_until,
                source_item.started_at,source_item.completed_at,row_source,source_item.created_at,source_item.updated_at
            ) ON CONFLICT (capture_recognition_job_id,attempt_number) DO UPDATE SET
                status=EXCLUDED.status,provider=EXCLUDED.provider,lease_until=EXCLUDED.lease_until,
                completed_at=EXCLUDED.completed_at,updated_at=EXCLUDED.updated_at;
        END IF;

        INSERT INTO file_links (
            id,user_id,workspace_id,file_id,capture_draft_id,capture_slot_version_id,
            purpose,link_state,source,created_at,released_at
        ) VALUES (
            gen_random_uuid(),source_set.user_id,source_set.workspace_id,source_item.file_id,draft_id,version_id,
            'capture_slot_source',CASE WHEN source_item.file_status='active' THEN 'active' ELSE 'released' END,
            row_source,source_item.created_at,
            CASE WHEN source_item.file_status='active' THEN NULL ELSE source_item.updated_at END
        ) ON CONFLICT (capture_slot_version_id,purpose) DO NOTHING;

        IF target_status='confirmed' AND same_tenant_product THEN
            INSERT INTO file_links (
                id,user_id,workspace_id,file_id,capture_draft_id,capture_slot_version_id,
                product_id,purpose,link_state,source,created_at,released_at
            ) VALUES (
                gen_random_uuid(),source_set.user_id,source_set.workspace_id,source_item.file_id,draft_id,version_id,
                source_set.product_id,'confirmed_product_source',
                CASE WHEN source_item.file_status='active' THEN 'active' ELSE 'released' END,
                row_source,COALESCE(source_set.confirmed_at,source_set.updated_at),
                CASE WHEN source_item.file_status='active' THEN NULL ELSE source_item.updated_at END
            ) ON CONFLICT (capture_slot_version_id,purpose) DO NOTHING;
        END IF;

        IF source_item.ocr_completed_at IS NOT NULL AND target_attempt>0 THEN
            INSERT INTO recognition_evidence (
                id,user_id,workspace_id,capture_draft_id,capture_slot_version_id,capture_recognition_job_id,
                legacy_recognition_job_id,job_attempt,file_id,stage,raw_text,provider,model,duration_ms,
                completed_at,source,created_at
            ) VALUES (
                gen_random_uuid(),source_set.user_id,source_set.workspace_id,draft_id,version_id,target_job_id,
                source_item.job_id,target_attempt,source_item.file_id,'ocr_transcript',source_item.ocr_text,
                COALESCE(NULLIF(source_item.ocr_provider,''),'legacy:unknown'),
                COALESCE(NULLIF(source_item.ocr_model,''),'legacy:unknown'),source_item.ocr_duration_ms,
                source_item.ocr_completed_at,row_source,source_item.ocr_completed_at
            ) ON CONFLICT (capture_recognition_job_id,job_attempt,stage) DO NOTHING;
        END IF;

        IF source_item.result IS NOT NULL AND source_item.completed_at IS NOT NULL AND target_attempt>0 THEN
            candidate_id := gen_random_uuid();
            INSERT INTO recognition_candidates (
                id,user_id,workspace_id,capture_draft_id,capture_slot_version_id,capture_recognition_job_id,
                legacy_recognition_job_id,job_attempt,payload,trace,confidence,provider,
                completed_at,source,created_at
            ) VALUES (
                candidate_id,source_set.user_id,source_set.workspace_id,draft_id,version_id,target_job_id,
                source_item.job_id,target_attempt,source_item.result,COALESCE(source_item.trace,'{}'::jsonb),
                source_item.confidence,COALESCE(NULLIF(source_item.provider,''),'legacy:unknown'),
                source_item.completed_at,row_source,source_item.completed_at
            ) ON CONFLICT (capture_recognition_job_id,job_attempt,candidate_kind) DO NOTHING
            RETURNING id INTO candidate_id;
            IF candidate_id IS NULL THEN
                SELECT id INTO candidate_id FROM recognition_candidates
                WHERE capture_recognition_job_id=target_job_id AND job_attempt=target_attempt
                  AND candidate_kind='structured';
            END IF;
            IF (SELECT current_slot_version_id=version_id FROM capture_slots WHERE id=slot_id) THEN
                merged_payload := jsonb_set(merged_payload,ARRAY[slot_role],source_item.result,true);
                merged_refs := jsonb_set(merged_refs,ARRAY[slot_role],to_jsonb(candidate_id::text),true);
            END IF;
        END IF;
    END LOOP;

    IF draft_created OR draft_was_active THEN
        UPDATE confirmation_drafts SET candidate_payload=merged_payload,candidate_refs=merged_refs,
            user_payload=COALESCE(source_set.confirmed_payload,user_payload),updated_at=source_set.updated_at
        WHERE id=confirmation_id;
    END IF;

    IF source_set.status IN ('processing','awaiting_confirmation')
       AND (SELECT count(*) FROM recognition_sets other
            WHERE other.workspace_id=source_set.workspace_id
              AND other.status IN ('processing','awaiting_confirmation'))>1 THEN
        INSERT INTO migration_quarantines (
            source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
        ) VALUES (
            'recognition_sets',source_set.id,'multiple_active_capture_drafts',
            source_set.user_id,source_set.workspace_id,'open',CURRENT_TIMESTAMP
        ) ON CONFLICT DO NOTHING;
    END IF;
    IF source_set.status='confirmed' AND NOT same_tenant_product THEN
        INSERT INTO migration_quarantines (
            source_table,source_id,reason_code,user_id,workspace_id,state,discovered_at
        ) VALUES (
            'recognition_sets',source_set.id,'confirmed_capture_product_mismatch',
            source_set.user_id,source_set.workspace_id,'open',CURRENT_TIMESTAMP
        ) ON CONFLICT DO NOTHING;
    END IF;
    RETURN draft_id;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_backfill_capture_drafts_batch(batch_size integer DEFAULT 100)
RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE
    source_id uuid;
    previous_draft uuid;
    processed integer := 0;
    mapped integer := 0;
    unchanged integer := 0;
    last_id uuid;
BEGIN
    IF batch_size<1 OR batch_size>10000 THEN
        RAISE EXCEPTION 'batch_size must be between 1 and 10000' USING ERRCODE='22023';
    END IF;
    IF (SELECT cycle_state='idle' FROM r1_capture_backfill_state WHERE singleton) THEN
        UPDATE r1_capture_backfill_state SET cycle_id=gen_random_uuid(),cycle_state='running',
            cursor_recognition_set_id=NULL,attempt_count=attempt_count+1,processed_count=0,mapped_count=0,
            unchanged_count=0,quarantined_count=0,
            source_count=(SELECT count(*) FROM recognition_sets),
            source_snapshot_hash=md5((SELECT count(*)::text||':'||COALESCE(max(id::text),'') FROM recognition_sets)),
            cycle_started_at=CURRENT_TIMESTAMP,cycle_completed_at=NULL,last_progress_at=CURRENT_TIMESTAMP,
            last_error='',updated_at=CURRENT_TIMESTAMP
        WHERE singleton;
    END IF;
    IF (SELECT pause_requested FROM r1_capture_backfill_state WHERE singleton) THEN
        UPDATE r1_capture_backfill_state SET cycle_state='paused',updated_at=CURRENT_TIMESTAMP WHERE singleton;
        RETURN false;
    END IF;
    UPDATE r1_capture_backfill_state SET cycle_state='running' WHERE singleton AND cycle_state='paused';

    FOR source_id IN
        SELECT id FROM recognition_sets
        WHERE id>COALESCE((SELECT cursor_recognition_set_id FROM r1_capture_backfill_state WHERE singleton),'00000000-0000-0000-0000-000000000000'::uuid)
        ORDER BY id LIMIT batch_size
    LOOP
        SELECT capture_draft_id INTO previous_draft FROM recognition_sets WHERE id=source_id;
        PERFORM r1_sync_capture_compat_set(source_id,'legacy_migration');
        processed := processed+1;
        IF previous_draft IS NULL THEN mapped:=mapped+1; ELSE unchanged:=unchanged+1; END IF;
        last_id:=source_id;
    END LOOP;

    IF processed=0 THEN
        UPDATE r1_capture_backfill_state SET cycle_state='idle',cursor_recognition_set_id=NULL,
            quarantined_count=(SELECT count(*) FROM migration_quarantines
                WHERE source_table='recognition_sets'
                  AND (reason_code LIKE 'capture_%' OR reason_code='multiple_active_capture_drafts')),
            cycle_completed_at=CURRENT_TIMESTAMP,last_progress_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
        WHERE singleton;
        RETURN true;
    END IF;
    UPDATE r1_capture_backfill_state SET cursor_recognition_set_id=last_id,
        processed_count=processed_count+processed,mapped_count=mapped_count+mapped,
        unchanged_count=unchanged_count+unchanged,last_progress_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
    WHERE singleton;
    RETURN false;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_backfill_capture_drafts() RETURNS void LANGUAGE plpgsql AS $$
DECLARE complete boolean:=false;
BEGIN
    WHILE NOT complete LOOP
        complete:=r1_backfill_capture_drafts_batch(100);
    END LOOP;
END;
$$;
-- +goose StatementEnd

SELECT r1_backfill_capture_drafts();

-- Called in the same transaction that claims the compatibility queue row.
-- +goose StatementBegin
CREATE FUNCTION r1_begin_capture_attempt(
    target_legacy_job_id uuid, expected_attempt integer, provider_value text,
    lease_value timestamptz, started_value timestamptz
) RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE item record;
BEGIN
    SELECT j.user_id,j.workspace_id,j.attempt,j.status,j.capture_slot_version_id,
           cj.id AS target_job_id,cj.capture_draft_id
    INTO item
    FROM recognition_jobs j
    JOIN capture_recognition_jobs cj ON cj.legacy_recognition_job_id=j.id
    WHERE j.id=target_legacy_job_id
    FOR UPDATE OF j,cj;
    IF item.target_job_id IS NULL OR item.attempt<>expected_attempt OR item.status<>'running' THEN
        RETURN false;
    END IF;
    UPDATE capture_recognition_attempts SET status='superseded',lease_until=NULL,
        completed_at=COALESCE(completed_at,started_value),updated_at=started_value
    WHERE capture_recognition_job_id=item.target_job_id
      AND attempt_number<expected_attempt AND status='running';
    INSERT INTO capture_recognition_attempts (
        id,user_id,workspace_id,capture_draft_id,capture_slot_version_id,
        capture_recognition_job_id,attempt_number,status,provider,lease_until,
        started_at,source,created_at,updated_at
    ) VALUES (
        gen_random_uuid(),item.user_id,item.workspace_id,item.capture_draft_id,item.capture_slot_version_id,
        item.target_job_id,expected_attempt,'running',provider_value,lease_value,
        started_value,'legacy_compat',started_value,started_value
    ) ON CONFLICT (capture_recognition_job_id,attempt_number) DO UPDATE SET
        status='running',provider=EXCLUDED.provider,lease_until=EXCLUDED.lease_until,
        completed_at=NULL,updated_at=EXCLUDED.updated_at;
    UPDATE capture_recognition_jobs SET status='running',current_attempt=expected_attempt,
        provider=provider_value,updated_at=started_value WHERE id=item.target_job_id;
    RETURN true;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_record_recognition_evidence(
    target_legacy_job_id uuid, expected_attempt integer, raw_value text,
    provider_value text, model_value text, duration_value integer, completed_value timestamptz
) RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE item record;
BEGIN
    SELECT j.user_id,j.workspace_id,j.file_id,j.capture_slot_version_id,
           cj.id AS target_job_id,cj.capture_draft_id
    INTO item
    FROM recognition_jobs j
    JOIN capture_recognition_jobs cj ON cj.legacy_recognition_job_id=j.id
    JOIN capture_recognition_attempts ca
      ON ca.capture_recognition_job_id=cj.id AND ca.attempt_number=expected_attempt
    WHERE j.id=target_legacy_job_id;
    IF item.target_job_id IS NULL THEN RETURN false; END IF;
    INSERT INTO recognition_evidence (
        id,user_id,workspace_id,capture_draft_id,capture_slot_version_id,capture_recognition_job_id,
        legacy_recognition_job_id,job_attempt,file_id,stage,raw_text,provider,model,duration_ms,
        completed_at,source,created_at
    ) VALUES (
        gen_random_uuid(),item.user_id,item.workspace_id,item.capture_draft_id,item.capture_slot_version_id,
        item.target_job_id,target_legacy_job_id,expected_attempt,item.file_id,'ocr_transcript',raw_value,
        provider_value,model_value,duration_value,completed_value,'legacy_compat',completed_value
    ) ON CONFLICT (capture_recognition_job_id,job_attempt,stage) DO NOTHING;
    RETURN true;
END;
$$;
-- +goose StatementEnd

-- The legacy terminal CAS runs before this function. Lock order is legacy job
-- -> slot -> draft -> confirmation, and the merge UPDATE repeats the current
-- pointer check so neither lease reclaim nor image replacement can leak a stale
-- candidate into the editable confirmation.
-- +goose StatementBegin
CREATE FUNCTION r1_record_recognition_candidate(
    target_legacy_job_id uuid, expected_attempt integer, payload_value jsonb,
    trace_value jsonb, confidence_value numeric, provider_value text,
    completed_value timestamptz
) RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE item record;
DECLARE candidate_id uuid;
DECLARE changed integer:=0;
BEGIN
    SELECT j.user_id,j.workspace_id,j.attempt,j.status AS legacy_status,j.role,
           j.capture_slot_version_id,cj.id AS target_job_id,cj.capture_draft_id,
           v.capture_slot_id
    INTO item
    FROM recognition_jobs j
    JOIN capture_recognition_jobs cj ON cj.legacy_recognition_job_id=j.id
    JOIN capture_slot_versions v ON v.id=cj.capture_slot_version_id
    JOIN capture_recognition_attempts ca
      ON ca.capture_recognition_job_id=cj.id AND ca.attempt_number=expected_attempt
    WHERE j.id=target_legacy_job_id
    FOR UPDATE OF j,cj,ca;
    IF item.target_job_id IS NULL THEN RETURN false; END IF;

    candidate_id:=gen_random_uuid();
    INSERT INTO recognition_candidates (
        id,user_id,workspace_id,capture_draft_id,capture_slot_version_id,capture_recognition_job_id,
        legacy_recognition_job_id,job_attempt,payload,trace,confidence,provider,
        completed_at,source,created_at
    ) VALUES (
        candidate_id,item.user_id,item.workspace_id,item.capture_draft_id,item.capture_slot_version_id,
        item.target_job_id,target_legacy_job_id,expected_attempt,payload_value,
        COALESCE(trace_value,'{}'::jsonb),confidence_value,provider_value,
        completed_value,'legacy_compat',completed_value
    ) ON CONFLICT (capture_recognition_job_id,job_attempt,candidate_kind) DO NOTHING
    RETURNING id INTO candidate_id;
    IF candidate_id IS NULL THEN
        SELECT id INTO candidate_id FROM recognition_candidates
        WHERE capture_recognition_job_id=item.target_job_id AND job_attempt=expected_attempt
          AND candidate_kind='structured';
    END IF;

    PERFORM 1 FROM capture_slots s
    WHERE s.id=item.capture_slot_id FOR UPDATE;
    PERFORM 1 FROM capture_drafts d
    WHERE d.id=item.capture_draft_id FOR UPDATE;

    IF item.attempt=expected_attempt AND item.legacy_status IN ('succeeded','partial') THEN
        UPDATE confirmation_drafts confirmation SET
            candidate_payload=jsonb_set(confirmation.candidate_payload,ARRAY[item.role],payload_value,true),
            candidate_refs=jsonb_set(confirmation.candidate_refs,ARRAY[item.role],to_jsonb(candidate_id::text),true),
            updated_at=completed_value
        FROM capture_drafts d,capture_slots s
        WHERE confirmation.id=d.current_confirmation_draft_id
          AND d.id=item.capture_draft_id AND d.status NOT IN ('confirmed','cancelled')
          AND confirmation.status='editing'
          AND s.id=item.capture_slot_id
          AND s.current_slot_version_id=item.capture_slot_version_id;
        GET DIAGNOSTICS changed=ROW_COUNT;
        IF changed=1 THEN
            UPDATE capture_drafts SET aggregate_version=aggregate_version+1,updated_at=completed_value
            WHERE id=item.capture_draft_id;
            RETURN true;
        END IF;
    END IF;
    RETURN false;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION r1_refresh_capture_compat_set(target_set_id uuid, changed_at timestamptz)
RETURNS void LANGUAGE plpgsql AS $$
BEGIN
    UPDATE capture_recognition_jobs cj SET status=j.status,current_attempt=j.attempt,
        max_attempts=j.max_attempts,provider=COALESCE(NULLIF(j.provider,''),'pending'),updated_at=changed_at
    FROM recognition_jobs j
    WHERE j.recognition_set_id=target_set_id AND cj.legacy_recognition_job_id=j.id;

    UPDATE capture_recognition_attempts ca SET
        status=CASE
            WHEN ca.attempt_number<j.attempt OR j.status='queued' THEN 'superseded'
            WHEN j.status='running' THEN 'running'
            WHEN j.status='succeeded' THEN 'succeeded'
            WHEN j.status='partial' THEN 'partial'
            WHEN j.status='failed' THEN 'failed'
            ELSE 'cancelled'
        END,
        provider=COALESCE(NULLIF(j.provider,''),ca.provider),lease_until=j.lease_until,
        completed_at=CASE WHEN j.status IN ('succeeded','partial','failed','cancelled') THEN j.completed_at ELSE NULL END,
        updated_at=changed_at
    FROM capture_recognition_jobs cj,recognition_jobs j
    WHERE cj.legacy_recognition_job_id=j.id AND j.recognition_set_id=target_set_id
      AND ca.capture_recognition_job_id=cj.id AND ca.attempt_number<=j.attempt;

    UPDATE capture_slot_versions v SET processing_state=j.status,updated_at=changed_at
    FROM recognition_jobs j
    WHERE j.recognition_set_id=target_set_id AND j.capture_slot_version_id=v.id
      AND v.processing_state<>'stale';
    UPDATE capture_drafts d SET status=rs.status,updated_at=changed_at
    FROM recognition_sets rs
    WHERE rs.id=target_set_id AND d.id=rs.capture_draft_id
      AND d.status NOT IN ('confirmed','cancelled');
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM capture_drafts WHERE source<>'legacy_migration')
       OR EXISTS (SELECT 1 FROM capture_slot_versions
                  WHERE source<>'legacy_migration' OR evidence_version>1 OR input_kind<>'upload')
       OR EXISTS (SELECT 1 FROM capture_recognition_jobs WHERE source<>'legacy_migration')
       OR EXISTS (SELECT 1 FROM capture_recognition_attempts WHERE source<>'legacy_migration')
       OR EXISTS (SELECT 1 FROM file_links WHERE source<>'legacy_migration')
       OR EXISTS (SELECT 1 FROM recognition_evidence WHERE source<>'legacy_migration')
       OR EXISTS (SELECT 1 FROM recognition_candidates WHERE source<>'legacy_migration')
       OR EXISTS (SELECT 1 FROM confirmation_drafts WHERE source<>'legacy_migration') THEN
        RAISE EXCEPTION 'E4 target writes exist; rollback requires a forward fix' USING ERRCODE='55000';
    END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE recognition_jobs DROP CONSTRAINT IF EXISTS recognition_jobs_capture_slot_version_fk;
ALTER TABLE recognition_sets DROP CONSTRAINT IF EXISTS recognition_sets_capture_draft_fk;
ALTER TABLE recognition_jobs DROP COLUMN IF EXISTS capture_slot_version_id;
ALTER TABLE recognition_sets DROP COLUMN IF EXISTS capture_draft_id;
DROP FUNCTION IF EXISTS r1_refresh_capture_compat_set(uuid,timestamptz);
DROP FUNCTION IF EXISTS r1_record_recognition_candidate(uuid,integer,jsonb,jsonb,numeric,text,timestamptz);
DROP FUNCTION IF EXISTS r1_record_recognition_evidence(uuid,integer,text,text,text,integer,timestamptz);
DROP FUNCTION IF EXISTS r1_begin_capture_attempt(uuid,integer,text,timestamptz,timestamptz);
DROP FUNCTION IF EXISTS r1_backfill_capture_drafts();
DROP FUNCTION IF EXISTS r1_backfill_capture_drafts_batch(integer);
DROP FUNCTION IF EXISTS r1_sync_capture_compat_set(uuid,text);
DROP TABLE IF EXISTS r1_capture_backfill_state;
DROP TRIGGER IF EXISTS files_prevent_linked_delete ON files;
DROP FUNCTION IF EXISTS r1_prevent_linked_file_delete();
DROP TRIGGER IF EXISTS file_links_validate ON file_links;
DROP FUNCTION IF EXISTS r1_validate_file_link();
DROP TRIGGER IF EXISTS capture_slot_versions_validate ON capture_slot_versions;
DROP FUNCTION IF EXISTS r1_validate_capture_slot_version();
DROP TRIGGER IF EXISTS capture_recognition_attempts_validate ON capture_recognition_attempts;
DROP TRIGGER IF EXISTS capture_recognition_jobs_validate ON capture_recognition_jobs;
DROP FUNCTION IF EXISTS r1_validate_capture_recognition_identity();
DROP TRIGGER IF EXISTS recognition_candidates_immutable ON recognition_candidates;
DROP TRIGGER IF EXISTS recognition_evidence_immutable ON recognition_evidence;
DROP FUNCTION IF EXISTS r1_block_capture_history_mutation();
ALTER TABLE capture_drafts DROP CONSTRAINT IF EXISTS capture_drafts_current_confirmation_fk;
ALTER TABLE capture_slots DROP CONSTRAINT IF EXISTS capture_slots_current_version_fk;
DROP TABLE IF EXISTS confirmation_drafts;
DROP TABLE IF EXISTS recognition_candidates;
DROP TABLE IF EXISTS recognition_evidence;
DROP TABLE IF EXISTS file_links;
DROP TABLE IF EXISTS capture_recognition_attempts;
DROP TABLE IF EXISTS capture_recognition_jobs;
DROP TABLE IF EXISTS capture_slot_versions;
DROP TABLE IF EXISTS capture_slots;
DROP TABLE IF EXISTS capture_drafts;
DROP INDEX IF EXISTS recognition_jobs_tenant_identity_uidx;
DROP INDEX IF EXISTS recognition_sets_tenant_identity_uidx;
