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

COMMIT;
