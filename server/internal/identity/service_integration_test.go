package identity

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"suppq.local/server/migrations"
)

type recordingMailer struct {
	mu    sync.Mutex
	codes map[string]string
}

func (sender *recordingMailer) SendLoginCode(_ context.Context, email, code string) error {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	sender.codes[email] = code
	return nil
}
func (sender *recordingMailer) code(email string) string {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	return sender.codes[email]
}

func TestIdentityInvitationAndDemoLifecycle(t *testing.T) {
	base := os.Getenv("SUPPQ_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("SUPPQ_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	adminPool, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer adminPool.Close()
	schema := fmt.Sprintf("suppq_identity_test_%d", time.Now().UnixNano())
	if _, err = adminPool.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = adminPool.Exec(context.Background(), `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`) })
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	testURL := parsed.String()
	if err = migrations.Up(ctx, testURL); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	sender := &recordingMailer{codes: map[string]string{}}
	service := New(pool, sender, Config{Pepper: "integration-test-pepper-with-at-least-32-characters", SessionTTL: 30 * 24 * time.Hour, DemoTTL: 24 * time.Hour, EmailCodeTTL: 10 * time.Minute})

	demoOne, err := service.EnsureSession(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	demoTwo, err := service.EnsureSession(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if demoOne.Actor.UserID == demoTwo.Actor.UserID || demoOne.Actor.WorkspaceID == demoTwo.Actor.WorkspaceID {
		t.Fatal("anonymous sessions must be isolated")
	}

	if err = service.BootstrapAdmin(ctx, "admin@example.test"); err != nil {
		t.Fatal(err)
	}
	if err = service.RequestLoginCode(ctx, "admin@example.test", ""); err != nil {
		t.Fatal(err)
	}
	adminSession, err := service.VerifyLoginCode(ctx, "admin@example.test", sender.code("admin@example.test"), "", "admin-password-123", demoOne.Token)
	if err != nil {
		t.Fatal(err)
	}
	if adminSession.Actor.Role != "admin" {
		t.Fatalf("expected admin, got %q", adminSession.Actor.Role)
	}

	created, err := service.CreateInvitation(ctx, adminSession.Actor, CreateInvitationInput{Kind: "generic_code", MaxUses: 1, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.RequestLoginCode(ctx, "person@example.test", created.Secret); err != nil {
		t.Fatal(err)
	}
	memberSession, err := service.VerifyLoginCode(ctx, "person@example.test", sender.code("person@example.test"), created.Secret, "member-password-123", demoTwo.Token)
	if err != nil {
		t.Fatal(err)
	}
	if memberSession.Actor.Kind != "registered" || memberSession.Actor.WorkspaceKind != "registered" {
		t.Fatalf("unexpected registered actor: %+v", memberSession.Actor)
	}
	passwordSession, err := service.PasswordLogin(ctx, "person@example.test", "member-password-123", "")
	if err != nil {
		t.Fatal(err)
	}
	if passwordSession.Actor.UserID != memberSession.Actor.UserID {
		t.Fatal("password and email-code identities must resolve to one user")
	}
	if err = service.RequestPasswordReset(ctx, "person@example.test"); err != nil {
		t.Fatal(err)
	}
	if err = service.ConfirmPasswordReset(ctx, "person@example.test", sender.code("person@example.test"), "member-new-password-456"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ActorForToken(ctx, passwordSession.Token, false); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("password reset must invalidate existing sessions, got %v", err)
	}
	if _, err = service.PasswordLogin(ctx, "person@example.test", "member-password-123", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password must fail, got %v", err)
	}
	if _, err = service.PasswordLogin(ctx, "person@example.test", "member-new-password-456", ""); err != nil {
		t.Fatalf("new password login failed: %v", err)
	}
	if err = service.RequestLoginCode(ctx, "second@example.test", created.Secret); !errors.Is(err, ErrInvitationInvalid) {
		t.Fatalf("expected exhausted invitation, got %v", err)
	}

	bound, err := service.CreateInvitation(ctx, adminSession.Actor, CreateInvitationInput{Kind: "email_bound", Email: "bound@example.test", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.RequestLoginCode(ctx, "wrong@example.test", bound.Secret); !errors.Is(err, ErrInvitationInvalid) {
		t.Fatalf("expected bound-email rejection, got %v", err)
	}
	if err = service.RequestLoginCode(ctx, "bound@example.test", bound.Secret); err != nil {
		t.Fatal(err)
	}

	concurrent, err := service.CreateInvitation(ctx, adminSession.Actor, CreateInvitationInput{Kind: "generic_code", MaxUses: 1, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	demoA, _ := service.EnsureSession(ctx, "")
	demoB, _ := service.EnsureSession(ctx, "")
	if err = service.RequestLoginCode(ctx, "race-a@example.test", concurrent.Secret); err != nil {
		t.Fatal(err)
	}
	if err = service.RequestLoginCode(ctx, "race-b@example.test", concurrent.Secret); err != nil {
		t.Fatal(err)
	}
	type attempt struct{ err error }
	results := make(chan attempt, 2)
	go func() {
		_, verifyErr := service.VerifyLoginCode(ctx, "race-a@example.test", sender.code("race-a@example.test"), concurrent.Secret, "", demoA.Token)
		results <- attempt{err: verifyErr}
	}()
	go func() {
		_, verifyErr := service.VerifyLoginCode(ctx, "race-b@example.test", sender.code("race-b@example.test"), concurrent.Secret, "", demoB.Token)
		results <- attempt{err: verifyErr}
	}()
	successes, rejected := 0, 0
	for range 2 {
		result := <-results
		if result.err == nil {
			successes++
		} else if errors.Is(result.err, ErrInvitationInvalid) {
			rejected++
		} else {
			t.Fatalf("unexpected concurrent claim error: %v", result.err)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("expected one successful and one rejected concurrent claim, got %d and %d", successes, rejected)
	}

	deleted, err := service.CleanupExpiredDemos(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 3 {
		t.Fatalf("expected three login-origin demos deleted, got %d", deleted)
	}
	var remaining int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE kind='demo_ephemeral'`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("expected only the failed concurrent claimant demo to remain, got %d", remaining)
	}
}
