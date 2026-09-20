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
    ('product_deletion_jobs'),
    ('product_plans'),
    ('schedule_versions'),
    ('dose_slots'),
    ('plan_state_intervals'),
    ('scheduled_occurrences'),
    ('r1_product_plan_backfill_state')
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
UNION ALL SELECT 'product_plans', count(*) FROM product_plans
UNION ALL SELECT 'schedule_versions', count(*) FROM schedule_versions
UNION ALL SELECT 'dose_slots', count(*) FROM dose_slots
UNION ALL SELECT 'plan_state_intervals', count(*) FROM plan_state_intervals
UNION ALL SELECT 'scheduled_occurrences', count(*) FROM scheduled_occurrences
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

WITH source_outcomes AS (
    SELECT p.id,
           EXISTS (
               SELECT 1 FROM product_plans plan
               WHERE plan.product_id=p.id AND plan.user_id=p.user_id AND plan.workspace_id=p.workspace_id
           ) AS mapped,
           EXISTS (
               SELECT 1 FROM migration_quarantines q
               WHERE q.source_id=p.id AND q.state='open'
                 AND q.reason_code IN (
                     'schedule_missing','schedule_tenant_mismatch','product_profile_missing',
                     'workspace_timezone_missing','day_cycle_tenant_mismatch','schedule_rule_out_of_range',
                     'schedule_invalid_local_time','schedule_duplicate_local_time',
                     'schedule_slot_count_mismatch','target_schedule_conflict','target_plan_state_conflict'
                 )
           ) AS quarantined
    FROM products p
    WHERE p.product_type='supplement'
)
SELECT count(*) AS plan_source_outcome_exclusivity_violations
FROM source_outcomes
WHERE mapped = quarantined;

SELECT count(*) AS unsupported_product_plan_violations
FROM products p
WHERE p.product_type IN ('otc','prescription')
  AND EXISTS (SELECT 1 FROM product_plans plan WHERE plan.product_id=p.id);

SELECT count(*) AS plan_tenant_relationship_violations
FROM (
    SELECT plan.id
    FROM product_plans plan
    JOIN products p ON p.id=plan.product_id
    WHERE plan.user_id<>p.user_id OR plan.workspace_id<>p.workspace_id
    UNION ALL
    SELECT sv.id
    FROM schedule_versions sv
    JOIN product_plans plan ON plan.id=sv.product_plan_id
    LEFT JOIN product_profile_versions profile
      ON profile.id=sv.product_profile_version_id
     AND profile.product_id=sv.product_id
     AND profile.user_id=sv.user_id
     AND profile.workspace_id=sv.workspace_id
    LEFT JOIN workspace_timezone_versions tz
      ON tz.id=sv.timezone_version_id
     AND tz.user_id=sv.user_id
     AND tz.workspace_id=sv.workspace_id
    WHERE sv.product_id<>plan.product_id OR sv.user_id<>plan.user_id OR sv.workspace_id<>plan.workspace_id
       OR profile.id IS NULL OR tz.id IS NULL
    UNION ALL
    SELECT slot.id
    FROM dose_slots slot
    JOIN schedule_versions sv ON sv.id=slot.schedule_version_id
    WHERE slot.product_plan_id<>sv.product_plan_id OR slot.product_id<>sv.product_id
       OR slot.user_id<>sv.user_id OR slot.workspace_id<>sv.workspace_id
) violations;

SELECT count(*) AS current_schedule_pointer_violations
FROM product_plans plan
LEFT JOIN schedule_versions sv
  ON sv.id=plan.current_schedule_version_id
 AND sv.product_plan_id=plan.id
 AND sv.product_id=plan.product_id
 AND sv.user_id=plan.user_id
 AND sv.workspace_id=plan.workspace_id
 AND sv.version_state='active'
WHERE plan.current_schedule_version_id IS NULL OR sv.id IS NULL;

SELECT count(*) AS schedule_interval_overlap_violations
FROM schedule_versions left_version
JOIN schedule_versions right_version
  ON right_version.product_plan_id=left_version.product_plan_id
 AND right_version.id>left_version.id
 AND right_version.version_state='active'
 AND left_version.version_state='active'
 AND daterange(right_version.effective_from,right_version.effective_to,'[)')
     && daterange(left_version.effective_from,left_version.effective_to,'[)');

SELECT count(*) AS plan_state_interval_overlap_violations
FROM plan_state_intervals left_interval
JOIN plan_state_intervals right_interval
  ON right_interval.product_plan_id=left_interval.product_plan_id
 AND right_interval.id>left_interval.id
 AND tstzrange(right_interval.effective_from,right_interval.effective_to,'[)')
     && tstzrange(left_interval.effective_from,left_interval.effective_to,'[)');

SELECT count(*) AS plan_without_open_state_interval
FROM product_plans plan
WHERE NOT EXISTS (
    SELECT 1 FROM plan_state_intervals state
    WHERE state.product_plan_id=plan.id AND state.effective_to IS NULL
);

WITH current_schedule AS (
    SELECT p.id AS product_id,p.user_id,p.workspace_id,p.dose_quantity,p.dose_times_per_day,
           p.with_food,ps.reminder_times,plan.id AS plan_id,sv.id AS schedule_id
    FROM products p
    JOIN product_schedules ps
      ON ps.product_id=p.id AND ps.user_id=p.user_id AND ps.workspace_id=p.workspace_id
    JOIN product_plans plan
      ON plan.product_id=p.id AND plan.user_id=p.user_id AND plan.workspace_id=p.workspace_id
    JOIN schedule_versions sv
      ON sv.id=plan.current_schedule_version_id AND sv.product_plan_id=plan.id
    WHERE p.product_type='supplement'
), target_slots AS (
    SELECT current_schedule.*,
           count(slot.id) AS slot_count,
           array_agg(to_char(slot.local_time,'HH24:MI') ORDER BY slot.local_time) AS target_times,
           count(*) FILTER (
               WHERE slot.quantity<>current_schedule.dose_quantity
                  OR slot.meal_relation<>CASE WHEN current_schedule.with_food IS TRUE
                                              THEN 'with_meal' ELSE 'unspecified' END
           ) AS value_mismatch_count
    FROM current_schedule
    LEFT JOIN dose_slots slot ON slot.schedule_version_id=current_schedule.schedule_id
    GROUP BY current_schedule.product_id,current_schedule.user_id,current_schedule.workspace_id,
             current_schedule.dose_quantity,current_schedule.dose_times_per_day,
             current_schedule.with_food,current_schedule.reminder_times,
             current_schedule.plan_id,current_schedule.schedule_id
)
SELECT count(*) AS current_dose_slot_mapping_violations
FROM target_slots
WHERE slot_count<>dose_times_per_day
   OR target_times IS DISTINCT FROM ARRAY(
       SELECT DISTINCT value FROM unnest(reminder_times) AS value ORDER BY value
   )
   OR value_mismatch_count<>0;

SELECT count(*) AS occurrence_natural_key_duplicate_violations
FROM (
    SELECT workspace_id,product_id,schedule_version_id,local_date,dose_slot_id
    FROM scheduled_occurrences
    GROUP BY workspace_id,product_id,schedule_version_id,local_date,dose_slot_id
    HAVING count(*)<>1
) duplicates;

SELECT count(*) AS occurrence_deterministic_id_violations
FROM scheduled_occurrences occurrence
WHERE occurrence.id<>r1_scheduled_occurrence_id(
    occurrence.schedule_version_id,occurrence.local_date,occurrence.dose_slot_id
);

SELECT count(*) AS occurrence_snapshot_violations
FROM scheduled_occurrences occurrence
JOIN dose_slots slot ON slot.id=occurrence.dose_slot_id
JOIN schedule_versions sv ON sv.id=occurrence.schedule_version_id
WHERE occurrence.planned_quantity<>slot.quantity
   OR occurrence.meal_relation_snapshot<>slot.meal_relation
   OR occurrence.slot_label_snapshot<>slot.label
   OR occurrence.iana_timezone<>sv.iana_timezone
   OR occurrence.timezone_version_id<>sv.timezone_version_id;

SELECT cycle_state,attempt_count,processed_count,mapped_count,unchanged_count,
       appended_count,quarantined_count,source_count,source_snapshot_hash,
       last_progress_at,last_completed_at
FROM r1_product_plan_backfill_state
WHERE singleton;

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
