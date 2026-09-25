package migrations

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

const r1ProductPlansVersion int64 = 202609200002

func TestR1CaptureEvidenceFreshAndCurrentSnapshotUpgrade(t *testing.T) {
	base := os.Getenv("SUPPQ_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("SUPPQ_TEST_DATABASE_URL is not set")
	}

	t.Run("fresh", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyAll(t, ctx, db)
			for _, table := range []string{
				"capture_drafts", "capture_slots", "capture_slot_versions",
				"capture_recognition_jobs", "capture_recognition_attempts",
				"file_links", "recognition_evidence", "recognition_candidates",
				"confirmation_drafts", "r1_capture_backfill_state",
			} {
				var exists bool
				if err := db.QueryRowContext(ctx,
					`SELECT to_regclass(current_schema() || '.' || $1) IS NOT NULL`, table,
				).Scan(&exists); err != nil {
					t.Fatal(err)
				}
				if !exists {
					t.Fatalf("expected E4 table %s", table)
				}
			}
			for _, signature := range []string{
				"r1_backfill_capture_drafts_batch(integer)",
				"r1_begin_capture_attempt(uuid,integer,text,timestamp with time zone,timestamp with time zone)",
				"r1_record_recognition_evidence(uuid,integer,text,text,text,integer,timestamp with time zone)",
				"r1_record_recognition_candidate(uuid,integer,jsonb,jsonb,numeric,text,timestamp with time zone)",
			} {
				var exists bool
				if err := db.QueryRowContext(ctx, `SELECT to_regprocedure($1) IS NOT NULL`, signature).Scan(&exists); err != nil {
					t.Fatal(err)
				}
				if !exists {
					t.Fatalf("expected E4 function %s", signature)
				}
			}
		})
	})

	t.Run("snapshot_backfill_and_target_replacement", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyCaptureBase(t, ctx, db)
			seedCaptureLegacySet(t, ctx, db, "40000000-0000-4000-8000-000000000101", time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC))
			configureGoose(t)
			if err := goose.UpContext(ctx, db, "."); err != nil {
				t.Fatalf("upgrade E3 snapshot through E4: %v", err)
			}

			assertCount(t, ctx, db, `SELECT count(*) FROM capture_drafts`, 1)
			assertCount(t, ctx, db, `SELECT count(*) FROM capture_slots`, 3)
			assertCount(t, ctx, db, `SELECT count(*) FROM capture_slot_versions`, 2)
			assertCount(t, ctx, db, `SELECT count(*) FROM capture_recognition_jobs`, 2)
			assertCount(t, ctx, db, `SELECT count(*) FROM capture_recognition_attempts`, 2)
			assertCount(t, ctx, db, `SELECT count(*) FROM file_links`, 2)
			assertCount(t, ctx, db, `SELECT count(*) FROM recognition_evidence`, 2)
			assertCount(t, ctx, db, `SELECT count(*) FROM recognition_candidates`, 1)
			assertCount(t, ctx, db, `SELECT count(*) FROM capture_slots
				WHERE role='expiry' AND current_slot_version_id IS NULL`, 1)
			assertCount(t, ctx, db, `SELECT count(*) FROM capture_recognition_jobs target
				JOIN recognition_jobs legacy ON legacy.id=target.legacy_recognition_job_id
				JOIN capture_slot_versions version ON version.id=target.capture_slot_version_id
				WHERE legacy.capture_slot_version_id=version.id`, 2)

			before := []int{
				queryCount(t, ctx, db, `SELECT count(*) FROM capture_drafts`),
				queryCount(t, ctx, db, `SELECT count(*) FROM capture_slot_versions`),
				queryCount(t, ctx, db, `SELECT count(*) FROM capture_recognition_jobs`),
				queryCount(t, ctx, db, `SELECT count(*) FROM recognition_candidates`),
			}
			if _, err := db.ExecContext(ctx, `SELECT r1_backfill_capture_drafts()`); err != nil {
				t.Fatal(err)
			}
			after := []int{
				queryCount(t, ctx, db, `SELECT count(*) FROM capture_drafts`),
				queryCount(t, ctx, db, `SELECT count(*) FROM capture_slot_versions`),
				queryCount(t, ctx, db, `SELECT count(*) FROM capture_recognition_jobs`),
				queryCount(t, ctx, db, `SELECT count(*) FROM recognition_candidates`),
			}
			for index := range before {
				if before[index] != after[index] {
					t.Fatalf("E4 backfill was not idempotent: before=%v after=%v", before, after)
				}
			}
			execSeed(t, ctx, db, `INSERT INTO recognition_sets (
				id,user_id,workspace_id,status,created_at,updated_at
			) VALUES (
				'40000000-0000-4000-8000-000000000104',
				'10000000-0000-4000-8000-000000000001',
				'10000000-0000-4000-8000-000000000011',
				'draft','2026-09-20T09:00:00Z','2026-09-20T09:00:00Z'
			)`)
			if _, err := db.ExecContext(ctx, `SELECT r1_backfill_capture_drafts()`); err != nil {
				t.Fatalf("catch up N-1 recognition set: %v", err)
			}
			assertCount(t, ctx, db, `SELECT count(*) FROM capture_drafts
				WHERE source_recognition_set_id='40000000-0000-4000-8000-000000000104'`, 1)
			assertCount(t, ctx, db, `SELECT count(*) FROM capture_slots slot
				JOIN capture_drafts draft ON draft.id=slot.capture_draft_id
				WHERE draft.source_recognition_set_id='40000000-0000-4000-8000-000000000104'`, 3)
			assertCount(t, ctx, db, `SELECT count(*) FROM capture_slot_versions version
				JOIN capture_drafts draft ON draft.id=version.capture_draft_id
				WHERE draft.source_recognition_set_id='40000000-0000-4000-8000-000000000104'`, 0)

			assertIndependentReplacementAndLateResults(t, ctx, db)
			configureGoose(t)
			if err := goose.DownToContext(ctx, db, ".", r1ProductPlansVersion); err == nil {
				t.Fatal("E4 Down accepted target-owned replacement data")
			}
		})
	})

	t.Run("down_before_target_writes", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyCaptureBase(t, ctx, db)
			seedCaptureLegacySet(t, ctx, db, "40000000-0000-4000-8000-000000000101", time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC))
			configureGoose(t)
			if err := goose.UpContext(ctx, db, "."); err != nil {
				t.Fatal(err)
			}
			if err := goose.DownToContext(ctx, db, ".", r1ProductPlansVersion); err != nil {
				t.Fatalf("rollback migration-only E4 data: %v", err)
			}
			var exists bool
			if err := db.QueryRowContext(ctx, `SELECT to_regclass(current_schema() || '.capture_drafts') IS NOT NULL`).Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if exists {
				t.Fatal("E4 rollback left target tables behind")
			}
			assertCount(t, ctx, db, `SELECT count(*) FROM recognition_sets`, 1)
			assertCount(t, ctx, db, `SELECT count(*) FROM recognition_jobs`, 2)
		})
	})

	t.Run("parent_cascade", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyCaptureBase(t, ctx, db)
			seedCaptureCascadeOwner(t, ctx, db)
			configureGoose(t)
			if err := goose.UpContext(ctx, db, "."); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `DELETE FROM users
				WHERE id='50000000-0000-4000-8000-000000000001'`); err != nil {
				t.Fatalf("E4 parent account cascade was blocked: %v", err)
			}
			for _, table := range []string{
				"capture_drafts", "capture_slots", "capture_slot_versions",
				"capture_recognition_jobs", "capture_recognition_attempts",
				"file_links", "recognition_evidence", "recognition_candidates", "confirmation_drafts",
			} {
				assertCount(t, ctx, db, `SELECT count(*) FROM `+table, 0)
			}
		})
	})

	t.Run("reconciliation_scripts", func(t *testing.T) {
		withMigrationSchema(t, base, func(ctx context.Context, db *sql.DB) {
			applyAll(t, ctx, db)
			for _, path := range []string{
				"../../scripts/sql/r1_source_inventory.sql",
				"../../scripts/sql/r1_reconciliation.sql",
			} {
				payload, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read %s: %v", path, err)
				}
				script := strings.ReplaceAll(string(payload), "\\set ON_ERROR_STOP on\n", "")
				if _, err = db.ExecContext(ctx, script); err != nil {
					t.Fatalf("execute %s: %v", path, err)
				}
			}
		})
	})
}

func seedCaptureCascadeOwner(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	createdAt := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	userID := "50000000-0000-4000-8000-000000000001"
	workspaceID := "50000000-0000-4000-8000-000000000011"
	setID := "50000000-0000-4000-8000-000000000101"
	fileID := "50000000-0000-4000-8000-000000000201"
	jobID := "50000000-0000-4000-8000-000000000301"
	execSeed(t, ctx, db, `INSERT INTO users (id,kind,status,role,last_activity_at,created_at)
		VALUES ($1,'registered','active','member',$2,$2)`, userID, createdAt)
	execSeed(t, ctx, db, `INSERT INTO workspaces (id,owner_user_id,kind,name,last_activity_at,created_at)
		VALUES ($1,$2,'registered','capture cascade',$3,$3)`, workspaceID, userID, createdAt)
	execSeed(t, ctx, db, `INSERT INTO workspace_members (workspace_id,user_id,role,created_at)
		VALUES ($1,$2,'owner',$3)`, workspaceID, userID, createdAt)
	execSeed(t, ctx, db, `INSERT INTO recognition_sets (
		id,user_id,workspace_id,status,created_at,updated_at
	) VALUES ($1,$2,$3,'processing',$4,$4)`, setID, userID, workspaceID, createdAt)
	execSeed(t, ctx, db, `INSERT INTO files (
		id,user_id,workspace_id,object_key,original_name,mime_type,byte_size,sha256_hex,created_at
	) VALUES ($1,$2,$3,'capture/cascade','front.png','image/png',100,
		'cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',$4)`, fileID, userID, workspaceID, createdAt)
	execSeed(t, ctx, db, `INSERT INTO recognition_files (recognition_set_id,file_id,role)
		VALUES ($1,$2,'front')`, setID, fileID)
	execSeed(t, ctx, db, `INSERT INTO recognition_jobs (
		id,user_id,workspace_id,recognition_set_id,file_id,role,status,run_after,created_at,updated_at
	) VALUES ($1,$2,$3,$4,$5,'front','queued',$6,$6,$6)`, jobID, userID, workspaceID, setID, fileID, createdAt)
}

func applyCaptureBase(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	applyProductProfileSnapshot(t, ctx, db)
	configureGoose(t)
	if err := goose.UpToContext(ctx, db, ".", r1ProductPlansVersion); err != nil {
		t.Fatalf("upgrade fixture through E3: %v", err)
	}
}

func seedCaptureLegacySet(t *testing.T, ctx context.Context, db *sql.DB, setID string, createdAt time.Time) {
	t.Helper()
	userID := "10000000-0000-4000-8000-000000000001"
	workspaceID := "10000000-0000-4000-8000-000000000011"
	execSeed(t, ctx, db, `INSERT INTO recognition_sets (
		id,user_id,workspace_id,status,created_at,updated_at
	) VALUES ($1,$2,$3,'awaiting_confirmation',$4,$4)`, setID, userID, workspaceID, createdAt)
	fixtures := []struct {
		role, fileID, jobID, status string
		attempt                     int
		withCandidate               bool
	}{
		{"front", "40000000-0000-4000-8000-000000000201", "40000000-0000-4000-8000-000000000301", "partial", 1, true},
		{"facts", "40000000-0000-4000-8000-000000000202", "40000000-0000-4000-8000-000000000302", "failed", 2, false},
	}
	for _, fixture := range fixtures {
		execSeed(t, ctx, db, `INSERT INTO files (
			id,user_id,workspace_id,object_key,original_name,mime_type,byte_size,sha256_hex,created_at
		) VALUES ($1,$2,$3,$4,$5,'image/png',100,$6,$7)`, fixture.fileID, userID, workspaceID,
			"capture/"+fixture.fileID, fixture.role+".png",
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", createdAt)
		execSeed(t, ctx, db, `INSERT INTO recognition_files (recognition_set_id,file_id,role)
			VALUES ($1,$2,$3)`, setID, fixture.fileID, fixture.role)
		var result any
		var completed any
		if fixture.withCandidate {
			result = `{"status":"partial","fields":{"name":"fixture"}}`
			completed = createdAt.Add(time.Minute)
		}
		execSeed(t, ctx, db, `INSERT INTO recognition_jobs (
			id,user_id,workspace_id,recognition_set_id,file_id,role,status,provider,attempt,max_attempts,
			confidence,result,error_code,error_message,run_after,created_at,started_at,completed_at,updated_at,
			ocr_text,ocr_provider,ocr_model,ocr_duration_ms,ocr_completed_at,trace
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'fixture:legacy',$8,3,0.5,$9::jsonb,'','',$10,$10,$10,$11,$10,
			'raw fixture','ocr:fixture','ocr-v1',10,$10,'{}'::jsonb)`, fixture.jobID, userID, workspaceID,
			setID, fixture.fileID, fixture.role, fixture.status, fixture.attempt, result, createdAt, completed)
	}
}

func assertIndependentReplacementAndLateResults(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	const replacementFile = "40000000-0000-4000-8000-000000000209"
	const replacementVersion = "40000000-0000-4000-8000-000000000409"
	const replacementJob = "40000000-0000-4000-8000-000000000509"
	var draftID, slotID, oldVersionID, confirmationID string
	if err := db.QueryRowContext(ctx, `SELECT draft.id,slot.id,slot.current_slot_version_id,draft.current_confirmation_draft_id
		FROM capture_drafts draft JOIN capture_slots slot ON slot.capture_draft_id=draft.id
		WHERE draft.source_recognition_set_id='40000000-0000-4000-8000-000000000101'
		  AND slot.role='facts'`).Scan(&draftID, &slotID, &oldVersionID, &confirmationID); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	execSeed(t, ctx, db, `INSERT INTO files (
		id,user_id,workspace_id,object_key,original_name,mime_type,byte_size,sha256_hex,created_at
	) VALUES ($1,'10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000011',
		'capture/replacement','facts-v2.png','image/png',100,
		'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',$2)`, replacementFile, createdAt)
	execSeed(t, ctx, db, `UPDATE capture_slot_versions SET processing_state='stale',superseded_at=$1,updated_at=$1
		WHERE id=$2`, createdAt, oldVersionID)
	execSeed(t, ctx, db, `INSERT INTO capture_slot_versions (
		id,user_id,workspace_id,capture_draft_id,capture_slot_id,evidence_version,input_kind,
		processing_state,file_id,source,created_at,updated_at
	) VALUES ($1,'10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000011',
		$2,$3,2,'upload','queued',$4,'target_application',$5,$5)`, replacementVersion, draftID, slotID, replacementFile, createdAt)
	execSeed(t, ctx, db, `INSERT INTO capture_recognition_jobs (
		id,user_id,workspace_id,capture_draft_id,capture_slot_version_id,file_id,status,
		current_attempt,max_attempts,provider,source,created_at,updated_at
	) VALUES ($1,'10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000011',
		$2,$3,$4,'queued',0,3,'pending','target_application',$5,$5)`, replacementJob, draftID, replacementVersion, replacementFile, createdAt)
	execSeed(t, ctx, db, `UPDATE capture_slots SET current_slot_version_id=$1,updated_at=$2 WHERE id=$3`, replacementVersion, createdAt, slotID)
	execSeed(t, ctx, db, `INSERT INTO file_links (
		id,user_id,workspace_id,file_id,capture_draft_id,capture_slot_version_id,
		purpose,link_state,source,created_at
	) VALUES (gen_random_uuid(),'10000000-0000-4000-8000-000000000001',
		'10000000-0000-4000-8000-000000000011',$1,$2,$3,
		'capture_slot_source','active','target_application',$4)`, replacementFile, draftID, replacementVersion, createdAt)

	assertCount(t, ctx, db, `SELECT count(*) FROM capture_recognition_jobs
		WHERE id='40000000-0000-4000-8000-000000000509' AND legacy_recognition_job_id IS NULL`, 1)
	assertCount(t, ctx, db, `SELECT count(*) FROM recognition_jobs
		WHERE recognition_set_id='40000000-0000-4000-8000-000000000101'`, 2)
	expectExecFailure(t, ctx, db, "capture recognition job binding was mutable", `UPDATE capture_recognition_jobs
		SET file_id='40000000-0000-4000-8000-000000000202' WHERE id=$1`, replacementJob)
	expectExecFailure(t, ctx, db, "active capture file was deletable", `UPDATE files
		SET status='deleted',deleted_at=$1 WHERE id=$2`, createdAt, replacementFile)

	var before string
	if err := db.QueryRowContext(ctx, `SELECT candidate_payload::text FROM confirmation_drafts WHERE id=$1`, confirmationID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	execSeed(t, ctx, db, `UPDATE recognition_jobs SET status='partial',result='{"late":true}'::jsonb,
		completed_at=$1,updated_at=$1 WHERE id='40000000-0000-4000-8000-000000000302'`, createdAt)
	var merged bool
	if err := db.QueryRowContext(ctx, `SELECT r1_record_recognition_candidate(
		'40000000-0000-4000-8000-000000000302',2,'{"late":true}'::jsonb,'{}'::jsonb,0.8,'late:test',$1
	)`, createdAt).Scan(&merged); err != nil {
		t.Fatal(err)
	}
	if merged {
		t.Fatal("candidate from superseded slot version merged into current confirmation")
	}
	var after string
	if err := db.QueryRowContext(ctx, `SELECT candidate_payload::text FROM confirmation_drafts WHERE id=$1`, confirmationID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("late replacement candidate changed confirmation: %s -> %s", before, after)
	}
	assertCount(t, ctx, db, `SELECT count(*) FROM recognition_candidates
		WHERE legacy_recognition_job_id='40000000-0000-4000-8000-000000000302' AND job_attempt=2`, 1)
	expectExecFailure(t, ctx, db, "recognition candidate was mutable", `UPDATE recognition_candidates
		SET payload='{"mutated":true}'::jsonb WHERE legacy_recognition_job_id='40000000-0000-4000-8000-000000000302'`)
}
