\set ON_ERROR_STOP on

BEGIN TRANSACTION READ ONLY;

SELECT current_database() AS database_name,
       current_schema() AS schema_name,
       current_setting('server_version') AS postgres_version,
       now() AS observed_at;

SELECT version_id, is_applied, tstamp
FROM goose_db_version
ORDER BY id;

SELECT 'users' AS object, count(*) AS row_count FROM users
UNION ALL SELECT 'workspaces', count(*) FROM workspaces
UNION ALL SELECT 'products', count(*) FROM products
UNION ALL SELECT 'product_ingredients', count(*) FROM product_ingredients
UNION ALL SELECT 'product_schedules', count(*) FROM product_schedules
UNION ALL SELECT 'day_cycle_versions', count(*) FROM day_cycle_versions
UNION ALL SELECT 'inventory_batches', count(*) FROM inventory_batches
UNION ALL SELECT 'inventory_events', count(*) FROM inventory_events
UNION ALL SELECT 'intake_records', count(*) FROM intake_records
UNION ALL SELECT 'intake_allocations', count(*) FROM intake_allocations
UNION ALL SELECT 'files', count(*) FROM files
UNION ALL SELECT 'recognition_sets', count(*) FROM recognition_sets
UNION ALL SELECT 'recognition_jobs', count(*) FROM recognition_jobs
ORDER BY object;

SELECT product_type, status, count(*) AS row_count
FROM products
GROUP BY product_type, status
ORDER BY product_type, status;

SELECT
    count(*) FILTER (WHERE p.product_type = 'supplement') AS supported_products,
    count(*) FILTER (WHERE p.product_type = 'supplement' AND NOT EXISTS (
        SELECT 1 FROM product_ingredients pi WHERE pi.product_id = p.id
    )) AS supported_products_without_ingredients,
    count(*) FILTER (WHERE p.product_type IN ('otc', 'prescription')) AS unsupported_products,
    count(*) FILTER (WHERE p.ingredient_serving_quantity <= 0) AS invalid_serving_quantity_products
FROM products p;

SELECT
    count(*) AS source_ingredient_rows,
    count(*) FILTER (WHERE amount = 0) AS zero_amount_rows,
    count(DISTINCT product_id) AS products_with_source_ingredients
FROM product_ingredients;

SELECT count(*) AS supported_batches_requiring_profile_binding
FROM inventory_batches b
JOIN products p
  ON p.id = b.product_id
 AND p.user_id = b.user_id
 AND p.workspace_id = b.workspace_id
WHERE p.product_type = 'supplement';

SELECT 'unsupported_product_type' AS anomaly, count(*) AS row_count
FROM products
WHERE product_type IN ('otc', 'prescription')
UNION ALL
SELECT 'product_owner_workspace_mismatch', count(*)
FROM products p
JOIN workspaces w ON w.id = p.workspace_id
WHERE w.owner_user_id <> p.user_id
UNION ALL
SELECT 'ingredient_tenant_mismatch', count(*)
FROM product_ingredients i
JOIN products p ON p.id = i.product_id
WHERE i.user_id <> p.user_id OR i.workspace_id <> p.workspace_id
UNION ALL
SELECT 'schedule_tenant_mismatch', count(*)
FROM product_schedules s
JOIN products p ON p.id = s.product_id
WHERE s.user_id <> p.user_id OR s.workspace_id <> p.workspace_id
UNION ALL
SELECT 'batch_tenant_mismatch', count(*)
FROM inventory_batches b
JOIN products p ON p.id = b.product_id
WHERE b.user_id <> p.user_id OR b.workspace_id <> p.workspace_id
UNION ALL
SELECT 'intake_tenant_mismatch', count(*)
FROM intake_records i
JOIN products p ON p.id = i.product_id
WHERE i.user_id <> p.user_id OR i.workspace_id <> p.workspace_id
UNION ALL
SELECT 'event_tenant_mismatch', count(*)
FROM inventory_events e
JOIN products p ON p.id = e.product_id
JOIN inventory_batches b ON b.id = e.batch_id
WHERE e.user_id <> p.user_id OR e.workspace_id <> p.workspace_id
   OR b.product_id <> p.id OR b.workspace_id <> p.workspace_id
ORDER BY anomaly;

WITH replay AS (
    SELECT b.id,
           b.current_quantity,
           COALESCE(sum(e.quantity_delta), 0::numeric) AS replay_quantity
    FROM inventory_batches b
    LEFT JOIN inventory_events e ON e.batch_id = b.id
    GROUP BY b.id, b.current_quantity
)
SELECT count(*) AS inventory_replay_mismatch_count
FROM replay
WHERE current_quantity <> replay_quantity;

WITH allocation_totals AS (
    SELECT i.id, i.quantity, COALESCE(sum(a.quantity), 0::numeric) AS allocated
    FROM intake_records i
    LEFT JOIN intake_allocations a ON a.intake_id = i.id
    GROUP BY i.id, i.quantity
)
SELECT count(*) AS intake_allocation_mismatch_count
FROM allocation_totals
WHERE quantity <> allocated;

SELECT
    count(*) FILTER (WHERE cardinality(weekdays) = 0) AS empty_weekday_rows,
    count(*) FILTER (WHERE cardinality(reminder_times) = 0) AS empty_slot_rows,
    count(*) FILTER (WHERE EXISTS (
        SELECT 1 FROM unnest(reminder_times) AS value
        WHERE value !~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'
    )) AS invalid_time_rows
FROM product_schedules;

SELECT
    count(*) FILTER (WHERE p.product_type = 'supplement' AND s.product_id IS NULL)
        AS supported_products_without_schedule,
    count(*) FILTER (
        WHERE p.product_type = 'supplement'
          AND cardinality(s.reminder_times) IS DISTINCT FROM p.dose_times_per_day
    ) AS schedule_slot_count_mismatch_rows,
    count(*) FILTER (
        WHERE p.product_type = 'supplement'
          AND cardinality(s.reminder_times) IS DISTINCT FROM (
              SELECT count(DISTINCT value) FROM unnest(s.reminder_times) AS value
          )
    ) AS duplicate_reminder_time_rows,
    count(*) FILTER (
        WHERE p.product_type = 'supplement'
          AND EXISTS (
              SELECT 1 FROM unnest(s.reminder_times) AS value
              WHERE value !~ '^(?:[01][0-9]|2[0-3]):[0-5][0-9]$'
          )
    ) AS invalid_reminder_time_rows,
    count(*) FILTER (
        WHERE p.product_type = 'supplement'
          AND cardinality(s.weekdays) IS DISTINCT FROM (
              SELECT count(DISTINCT value) FROM unnest(s.weekdays) AS value
          )
    ) AS duplicate_weekday_rows
FROM products p
LEFT JOIN product_schedules s
  ON s.product_id = p.id
 AND s.user_id = p.user_id
 AND s.workspace_id = p.workspace_id;

SELECT 'day_cycle_orphan' AS anomaly, count(*) AS row_count
FROM day_cycle_versions history
LEFT JOIN products p ON p.id = history.product_id
WHERE p.id IS NULL
UNION ALL
SELECT 'day_cycle_tenant_mismatch', count(*)
FROM day_cycle_versions history
JOIN products p ON p.id = history.product_id
WHERE history.user_id <> p.user_id OR history.workspace_id <> p.workspace_id
UNION ALL
SELECT 'day_cycle_out_of_target_range', count(*)
FROM day_cycle_versions history
WHERE history.cycle_days NOT BETWEEN 1 AND 365
   OR history.take_days NOT BETWEEN 1 AND history.cycle_days
ORDER BY anomaly;

SELECT
    count(DISTINCT product_id) AS products_with_day_cycle_history,
    count(*) AS day_cycle_history_rows,
    max(history_count) AS maximum_versions_per_product
FROM (
    SELECT product_id, count(*) AS history_count
    FROM day_cycle_versions
    GROUP BY product_id
) history_counts;

SELECT count(*) AS workspace_timezone_coverage_violations
FROM workspaces w
LEFT JOIN workspace_timezone_versions tz
  ON tz.id = w.current_timezone_version_id
 AND tz.user_id = w.owner_user_id
 AND tz.workspace_id = w.id
WHERE w.current_timezone_version_id IS NULL OR tz.id IS NULL OR tz.effective_to IS NOT NULL;

SELECT
    count(*) FILTER (WHERE rs.status = 'confirmed' AND rs.product_id IS NULL) AS confirmed_without_product,
    count(*) FILTER (WHERE rs.status <> 'confirmed' AND rs.product_id IS NOT NULL) AS unconfirmed_with_product
FROM recognition_sets rs;

SELECT role, count(*) AS job_count,
       count(*) FILTER (WHERE ocr_text <> '') AS with_ocr_evidence,
       count(*) FILTER (WHERE status IN ('failed', 'cancelled')) AS terminal_without_candidate
FROM recognition_jobs
GROUP BY role
ORDER BY role;

SELECT count(*) AS unreferenced_active_file_count
FROM files f
WHERE f.status = 'active'
  AND NOT EXISTS (SELECT 1 FROM recognition_files rf WHERE rf.file_id = f.id);

SELECT count(*) AS workspace_timezone_pointer_column_count
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'workspaces'
  AND column_name = 'current_timezone_version_id';

SELECT required.table_name,
       to_regclass(current_schema() || '.' || required.table_name) IS NOT NULL AS present
FROM (VALUES
    ('product_profile_versions'),
    ('ingredient_profile_versions'),
    ('ingredient_profile_items'),
    ('product_media_links'),
    ('product_deletion_jobs')
) AS required(table_name)
ORDER BY required.table_name;

SELECT required.table_name,
       to_regclass(current_schema() || '.' || required.table_name) IS NOT NULL AS present
FROM (VALUES
    ('capture_drafts'),
    ('capture_slots'),
    ('capture_slot_versions'),
    ('capture_recognition_jobs'),
    ('capture_recognition_attempts'),
    ('file_links'),
    ('recognition_evidence'),
    ('recognition_candidates'),
    ('confirmation_drafts'),
    ('r1_capture_backfill_state')
) AS required(table_name)
ORDER BY required.table_name;

SELECT 'capture_drafts' AS entity,count(*) AS row_count FROM capture_drafts
UNION ALL SELECT 'capture_slots',count(*) FROM capture_slots
UNION ALL SELECT 'capture_slot_versions',count(*) FROM capture_slot_versions
UNION ALL SELECT 'capture_recognition_jobs',count(*) FROM capture_recognition_jobs
UNION ALL SELECT 'capture_recognition_attempts',count(*) FROM capture_recognition_attempts
UNION ALL SELECT 'file_links',count(*) FROM file_links
UNION ALL SELECT 'recognition_evidence',count(*) FROM recognition_evidence
UNION ALL SELECT 'recognition_candidates',count(*) FROM recognition_candidates
UNION ALL SELECT 'confirmation_drafts',count(*) FROM confirmation_drafts
ORDER BY entity;

SELECT
    count(*) FILTER (WHERE capture_draft_id IS NULL) AS recognition_sets_not_mapped,
    count(*) FILTER (WHERE status IN ('processing','awaiting_confirmation')) AS active_legacy_sets
FROM recognition_sets;

SELECT
    count(*) FILTER (WHERE version.id IS NULL) AS legacy_roles_without_slot_version,
    count(*) FILTER (WHERE target_job.id IS NULL) AS legacy_roles_without_target_job,
    count(*) FILTER (WHERE link.id IS NULL) AS legacy_roles_without_file_link
FROM recognition_jobs legacy
LEFT JOIN capture_slot_versions version ON version.id=legacy.capture_slot_version_id
LEFT JOIN capture_recognition_jobs target_job ON target_job.legacy_recognition_job_id=legacy.id
LEFT JOIN file_links link
  ON link.capture_slot_version_id=version.id AND link.purpose='capture_slot_source';

SELECT cycle_state,attempt_count,processed_count,mapped_count,unchanged_count,
       quarantined_count,source_count,source_snapshot_hash,last_progress_at,cycle_completed_at
FROM r1_capture_backfill_state
WHERE singleton;

SELECT required.table_name,
       to_regclass(current_schema() || '.' || required.table_name) IS NOT NULL AS present
FROM (VALUES
    ('intake_status_facts'),
    ('r1_intake_inventory_backfill_state')
) AS required(table_name)
ORDER BY required.table_name;

SELECT
    count(*) FILTER (WHERE aggregate_version IS NULL) AS batches_without_version,
    count(*) FILTER (WHERE lifecycle_state IS NULL) AS batches_without_lifecycle,
    count(*) FILTER (WHERE unit_snapshot IS NULL) AS batches_without_unit_snapshot,
    count(*) FILTER (WHERE received_quantity IS NULL) AS batches_without_received_quantity,
    count(*) FILTER (WHERE expiry_precision IS NULL) AS batches_without_expiry_precision,
    count(*) FILTER (WHERE currency IS NULL) AS batches_without_currency
FROM inventory_batches;

SELECT
    count(*) FILTER (WHERE aggregate_version IS NULL) AS intakes_without_version,
    count(*) FILTER (WHERE occurred_at IS NULL) AS intakes_without_occurred_at,
    count(*) FILTER (WHERE timezone_version_id IS NULL) AS intakes_without_timezone,
    count(*) FILTER (WHERE product_profile_version_id IS NULL) AS intakes_without_product_snapshot,
    count(*) FILTER (WHERE ingredient_profile_version_id IS NULL) AS intakes_without_ingredient_snapshot,
    count(*) FILTER (WHERE history_completeness IS NULL) AS intakes_without_completeness
FROM intake_records;

SELECT
    count(*) FILTER (WHERE ledger_kind IS NULL) AS events_without_ledger_kind,
    count(*) FILTER (WHERE source_type IS NULL OR source_id IS NULL) AS events_without_source_identity,
    count(*) FILTER (WHERE posted_at IS NULL) AS events_without_posted_at,
    count(*) FILTER (WHERE batch_version_before IS NULL OR balance_after IS NULL) AS events_without_balance_snapshot,
    count(*) FILTER (WHERE ledger_kind='intake_undo' AND compensates_event_id IS NULL) AS undo_without_compensation
FROM inventory_events;

SELECT
    count(*) FILTER (WHERE user_id IS NULL OR workspace_id IS NULL OR product_id IS NULL) AS allocations_without_owner,
    count(*) FILTER (WHERE inventory_event_id IS NULL) AS allocations_without_event,
    count(*) FILTER (WHERE batch_version_as_allocated IS NULL) AS allocations_without_batch_version,
    count(*) FILTER (WHERE batch_balance_before IS NULL OR batch_balance_after IS NULL) AS allocations_without_balance_snapshot,
    count(*) FILTER (WHERE unit_snapshot IS NULL) AS allocations_without_unit_snapshot
FROM intake_allocations;

SELECT cycle_state,phase,attempt_count,processed_count,source_count,
       source_snapshot_hash,cycle_started_at,last_progress_at,last_completed_at,updated_at
FROM r1_intake_inventory_backfill_state
WHERE singleton;

SELECT required.table_name,
       to_regclass(current_schema() || '.' || required.table_name) IS NOT NULL AS present
FROM (VALUES
    ('inventory_risk_projection_sets'),
    ('inventory_risk_projections'),
    ('reminder_preferences'),
    ('reminder_preference_versions'),
    ('reminder_window_preferences'),
    ('product_reminder_overrides'),
    ('reminder_events'),
    ('reminder_targets'),
    ('reminder_materialization_cursors')
) AS required(table_name)
ORDER BY required.table_name;

SELECT configuration_state,count(*) AS workspace_count
FROM reminder_preferences
GROUP BY configuration_state
ORDER BY configuration_state;

SELECT 'inventory_risk_projection_sets' AS entity,count(*) AS row_count
FROM inventory_risk_projection_sets
UNION ALL SELECT 'inventory_risk_projections',count(*) FROM inventory_risk_projections
UNION ALL SELECT 'reminder_preference_versions',count(*) FROM reminder_preference_versions
UNION ALL SELECT 'reminder_window_preferences',count(*) FROM reminder_window_preferences
UNION ALL SELECT 'product_reminder_overrides',count(*) FROM product_reminder_overrides
UNION ALL SELECT 'reminder_events',count(*) FROM reminder_events
UNION ALL SELECT 'reminder_targets',count(*) FROM reminder_targets
UNION ALL SELECT 'reminder_materialization_cursors',count(*) FROM reminder_materialization_cursors
ORDER BY entity;

SELECT count(*) AS migrated_preferences_that_imply_authorization
FROM reminder_preferences preference
WHERE preference.configuration_state<>'needs_confirmation'
   OR preference.aggregate_version<>0
   OR preference.current_version_id IS NOT NULL;

COMMIT;
