\set ON_ERROR_STOP on

BEGIN TRANSACTION READ ONLY;

SELECT required.table_name,
       to_regclass(current_schema() || '.' || required.table_name) IS NOT NULL AS present
FROM (VALUES
    ('workspace_timezone_versions'),
    ('client_actions'),
    ('domain_changes'),
    ('domain_change_consumer_receipts'),
    ('projection_revisions'),
    ('migration_quarantines')
) AS required(table_name)
ORDER BY required.table_name;

SELECT count(*) AS workspace_timezone_coverage_violations
FROM workspaces w
LEFT JOIN workspace_timezone_versions tz
  ON tz.id = w.current_timezone_version_id
 AND tz.workspace_id = w.id
 AND tz.user_id = w.owner_user_id
WHERE w.current_timezone_version_id IS NULL
   OR tz.id IS NULL
   OR tz.effective_to IS NOT NULL;

SELECT count(*) AS initial_timezone_contract_violations
FROM workspace_timezone_versions
WHERE business_version = 1
  AND (iana_timezone <> 'Etc/UTC'
       OR confirmation_state <> 'needs_confirmation'
       OR source <> 'legacy_unspecified');

SELECT count(*) AS unsupported_product_without_open_quarantine
FROM products p
LEFT JOIN migration_quarantines q
  ON q.source_table = 'products'
 AND q.source_id = p.id
 AND q.reason_code = 'unsupported_product_type'
 AND q.state = 'open'
WHERE p.product_type IN ('otc', 'prescription')
  AND q.source_id IS NULL;

SELECT count(*) AS supported_product_wrongly_quarantined
FROM products p
JOIN migration_quarantines q
  ON q.source_table = 'products'
 AND q.source_id = p.id
 AND q.reason_code = 'unsupported_product_type'
 AND q.state = 'open'
WHERE p.product_type = 'supplement';

SELECT count(*) AS timezone_multi_current_violations
FROM (
    SELECT workspace_id
    FROM workspace_timezone_versions
    WHERE effective_to IS NULL
    GROUP BY workspace_id
    HAVING count(*) <> 1
) violations;

SELECT 'client_actions' AS object, count(*) AS row_count FROM client_actions
UNION ALL SELECT 'domain_changes', count(*) FROM domain_changes
UNION ALL SELECT 'domain_change_consumer_receipts', count(*) FROM domain_change_consumer_receipts
UNION ALL SELECT 'projection_revisions', count(*) FROM projection_revisions
UNION ALL SELECT 'migration_quarantines', count(*) FROM migration_quarantines
ORDER BY object;

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

COMMIT;
