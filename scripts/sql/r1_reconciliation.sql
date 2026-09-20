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
    ('migration_quarantines'),
    ('product_profile_versions'),
    ('ingredient_profile_versions'),
    ('ingredient_profile_items'),
    ('product_media_links'),
    ('product_deletion_jobs')
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
UNION ALL SELECT 'product_profile_versions', count(*) FROM product_profile_versions
UNION ALL SELECT 'ingredient_profile_versions', count(*) FROM ingredient_profile_versions
UNION ALL SELECT 'ingredient_profile_items', count(*) FROM ingredient_profile_items
UNION ALL SELECT 'product_media_links', count(*) FROM product_media_links
UNION ALL SELECT 'product_deletion_jobs', count(*) FROM product_deletion_jobs
ORDER BY object;

SELECT count(*) AS supported_product_profile_coverage_violations
FROM products p
LEFT JOIN product_profile_versions ppv
  ON ppv.id = p.current_product_profile_version_id
 AND ppv.product_id = p.id
 AND ppv.user_id = p.user_id
 AND ppv.workspace_id = p.workspace_id
 AND ppv.effective_to IS NULL
LEFT JOIN ingredient_profile_versions ipv
  ON ipv.id = p.current_ingredient_profile_version_id
 AND ipv.product_id = p.id
 AND ipv.user_id = p.user_id
 AND ipv.workspace_id = p.workspace_id
 AND ipv.effective_to IS NULL
WHERE p.product_type = 'supplement'
  AND (p.catalog_state IS NULL
       OR p.aggregate_version IS NULL
       OR ppv.id IS NULL
       OR ipv.id IS NULL);

SELECT count(*) AS catalog_state_mapping_violations
FROM products p
WHERE p.product_type = 'supplement'
  AND p.catalog_state IN ('in_cabinet', 'archived')
  AND p.catalog_state IS DISTINCT FROM
      CASE WHEN p.status = 'archived' THEN 'archived' ELSE 'in_cabinet' END;

SELECT count(*) AS unsupported_product_target_fact_violations
FROM products p
WHERE p.product_type IN ('otc', 'prescription')
  AND (p.catalog_state IS NOT NULL
       OR p.aggregate_version IS NOT NULL
       OR p.current_product_profile_version_id IS NOT NULL
       OR p.current_ingredient_profile_version_id IS NOT NULL
       OR EXISTS (SELECT 1 FROM product_profile_versions ppv WHERE ppv.product_id = p.id)
       OR EXISTS (SELECT 1 FROM ingredient_profile_versions ipv WHERE ipv.product_id = p.id)
       OR EXISTS (
           SELECT 1 FROM inventory_batches b
           WHERE b.product_id = p.id AND b.ingredient_profile_version_id IS NOT NULL
       ));

WITH comparable AS (
    SELECT p.id,
           p.name AS legacy_name,
           p.brand AS legacy_brand,
           p.unit AS legacy_management_unit,
           p.updated_at AS legacy_updated_at,
           ppv.name AS profile_name,
           ppv.brand AS profile_brand,
           ppv.management_unit AS profile_management_unit,
           ppv.source_updated_at,
           ppv.name IS DISTINCT FROM p.name
               OR ppv.brand IS DISTINCT FROM p.brand
               OR ppv.management_unit IS DISTINCT FROM p.unit AS has_value_drift
    FROM products p
    JOIN product_profile_versions ppv
      ON ppv.id = p.current_product_profile_version_id
     AND ppv.product_id = p.id
     AND ppv.user_id = p.user_id
     AND ppv.workspace_id = p.workspace_id
     AND ppv.effective_to IS NULL
    WHERE p.product_type = 'supplement'
      -- A target profile with a later source timestamp is intentionally newer
      -- than the still-compatible legacy row and is not a legacy-authoritative
      -- reconciliation failure.
      AND ppv.source_updated_at <= p.updated_at
)
SELECT count(*) FILTER (
           WHERE profile_name IS DISTINCT FROM legacy_name
       ) AS current_product_profile_name_mismatches,
       count(*) FILTER (
           WHERE profile_brand IS DISTINCT FROM legacy_brand
       ) AS current_product_profile_brand_mismatches,
       count(*) FILTER (
           WHERE profile_management_unit IS DISTINCT FROM legacy_management_unit
       ) AS current_product_profile_management_unit_mismatches,
       count(*) FILTER (
           WHERE source_updated_at IS DISTINCT FROM legacy_updated_at
             AND has_value_drift
       ) AS current_product_profile_timestamp_value_drift_mismatches
FROM comparable;

WITH comparable AS (
    SELECT p.ingredient_serving_quantity AS legacy_serving_quantity,
           p.unit AS legacy_serving_unit,
           ipv.serving_quantity AS profile_serving_quantity,
           ipv.serving_unit AS profile_serving_unit
    FROM products p
    JOIN ingredient_profile_versions ipv
      ON ipv.id = p.current_ingredient_profile_version_id
     AND ipv.product_id = p.id
     AND ipv.user_id = p.user_id
     AND ipv.workspace_id = p.workspace_id
     AND ipv.effective_to IS NULL
    WHERE p.product_type = 'supplement'
      AND ipv.source_updated_at <= p.updated_at
)
SELECT count(*) FILTER (
           WHERE profile_serving_quantity IS DISTINCT FROM legacy_serving_quantity
       ) AS current_ingredient_profile_serving_quantity_mismatches,
       count(*) FILTER (
           WHERE profile_serving_unit IS DISTINCT FROM legacy_serving_unit
       ) AS current_ingredient_profile_serving_unit_mismatches
FROM comparable;

WITH current_profiles AS (
    SELECT p.id AS product_id,
           p.user_id,
           p.workspace_id,
           p.updated_at AS legacy_updated_at,
           ipv.id AS profile_id,
           ipv.change_kind,
           ipv.source_updated_at
    FROM products p
    JOIN ingredient_profile_versions ipv
      ON ipv.id = p.current_ingredient_profile_version_id
     AND ipv.product_id = p.id
     AND ipv.user_id = p.user_id
     AND ipv.workspace_id = p.workspace_id
     AND ipv.effective_to IS NULL
    WHERE p.product_type = 'supplement'
), source_collection AS (
    SELECT current_profiles.product_id,
           count(source.id) AS source_count,
           COALESCE(
               array_agg(source.id ORDER BY source.created_at, source.id)
                   FILTER (WHERE source.id IS NOT NULL),
               ARRAY[]::uuid[]
           ) AS source_order
    FROM current_profiles
    LEFT JOIN product_ingredients source
      ON source.product_id = current_profiles.product_id
     AND source.user_id = current_profiles.user_id
     AND source.workspace_id = current_profiles.workspace_id
    GROUP BY current_profiles.product_id
), target_collection AS (
    SELECT current_profiles.product_id,
           count(item.id) AS target_count,
           count(source.id) AS mapped_source_count,
           count(*) FILTER (
               WHERE item.id IS NOT NULL
                 AND (source.id IS NULL
                      OR item.original_key IS DISTINCT FROM source.ingredient_key
                      OR item.original_name IS DISTINCT FROM source.name
                      OR item.label_amount IS DISTINCT FROM source.amount
                      OR item.label_unit IS DISTINCT FROM source.unit)
           ) AS raw_value_mismatch_count,
           COALESCE(min(item.item_order), 0) AS minimum_item_order,
           COALESCE(max(item.item_order), -1) AS maximum_item_order,
           COALESCE(
               array_agg(item.legacy_ingredient_id ORDER BY item.item_order)
                   FILTER (WHERE item.id IS NOT NULL),
               ARRAY[]::uuid[]
           ) AS target_legacy_order
    FROM current_profiles
    LEFT JOIN ingredient_profile_items item
      ON item.ingredient_profile_version_id = current_profiles.profile_id
     AND item.product_id = current_profiles.product_id
     AND item.user_id = current_profiles.user_id
     AND item.workspace_id = current_profiles.workspace_id
    LEFT JOIN product_ingredients source
      ON source.id = item.legacy_ingredient_id
     AND source.product_id = current_profiles.product_id
     AND source.user_id = current_profiles.user_id
     AND source.workspace_id = current_profiles.workspace_id
    GROUP BY current_profiles.product_id
)
SELECT count(*) AS current_ingredient_raw_collection_mismatches
FROM current_profiles
JOIN source_collection USING (product_id)
JOIN target_collection USING (product_id)
WHERE current_profiles.source_updated_at <= current_profiles.legacy_updated_at
  AND (source_collection.source_count <> target_collection.target_count
       OR target_collection.target_count <> target_collection.mapped_source_count
       OR target_collection.raw_value_mismatch_count <> 0
       OR target_collection.minimum_item_order <> 0
       OR target_collection.maximum_item_order <> target_collection.target_count - 1
       OR (current_profiles.change_kind = 'legacy_import'
           AND source_collection.source_order IS DISTINCT FROM target_collection.target_legacy_order));

SELECT count(*) AS supported_batch_profile_binding_violations
FROM inventory_batches b
JOIN products p
  ON p.id = b.product_id
 AND p.user_id = b.user_id
 AND p.workspace_id = b.workspace_id
LEFT JOIN ingredient_profile_versions ipv
  ON ipv.id = b.ingredient_profile_version_id
 AND ipv.product_id = b.product_id
 AND ipv.user_id = b.user_id
 AND ipv.workspace_id = b.workspace_id
WHERE p.product_type = 'supplement'
  AND ipv.id IS NULL;

SELECT count(*) AS profile_tenant_relationship_violations
FROM (
    SELECT ppv.id
    FROM product_profile_versions ppv
    JOIN products p ON p.id = ppv.product_id
    WHERE ppv.user_id <> p.user_id OR ppv.workspace_id <> p.workspace_id
    UNION ALL
    SELECT ipv.id
    FROM ingredient_profile_versions ipv
    JOIN products p ON p.id = ipv.product_id
    WHERE ipv.user_id <> p.user_id OR ipv.workspace_id <> p.workspace_id
    UNION ALL
    SELECT item.id
    FROM ingredient_profile_items item
    JOIN ingredient_profile_versions ipv ON ipv.id = item.ingredient_profile_version_id
    WHERE item.product_id <> ipv.product_id
       OR item.user_id <> ipv.user_id
       OR item.workspace_id <> ipv.workspace_id
) violations;

SELECT count(*) AS profile_interval_overlap_violations
FROM (
    SELECT left_version.id
    FROM product_profile_versions left_version
    JOIN product_profile_versions right_version
      ON right_version.product_id = left_version.product_id
     AND right_version.id > left_version.id
     AND tstzrange(right_version.effective_from, right_version.effective_to, '[)')
         && tstzrange(left_version.effective_from, left_version.effective_to, '[)')
    UNION ALL
    SELECT left_version.id
    FROM ingredient_profile_versions left_version
    JOIN ingredient_profile_versions right_version
      ON right_version.product_id = left_version.product_id
     AND right_version.id > left_version.id
     AND tstzrange(right_version.effective_from, right_version.effective_to, '[)')
         && tstzrange(left_version.effective_from, left_version.effective_to, '[)')
) violations;

SELECT count(*) AS legacy_confirmed_empty_violations
FROM ingredient_profile_versions
WHERE change_kind = 'legacy_import'
  AND profile_status = 'confirmed_empty';

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
