package identity

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"suppq.local/server/internal/mailer"
)

var (
	ErrInvalidCredentials = NewError("invalid_credentials", "邮箱、验证码或密码不正确。")
	ErrInvitationRequired = NewError("invitation_required", "新账户需要有效邀请。")
	ErrInvitationInvalid  = NewError("invitation_invalid", "邀请不存在、已过期、已撤销或已用完。")
	ErrRateLimited        = NewError("rate_limited", "请求过于频繁，请稍后再试。")
	ErrUnauthorized       = NewError("unauthorized", "请先登录。")
	ErrForbidden          = NewError("forbidden", "当前账户无权执行此操作。")
)

type Error struct {
	Code    string
	Message string
}

func NewError(code, message string) *Error { return &Error{Code: code, Message: message} }
func (err *Error) Error() string           { return err.Code }

type Config struct {
	Pepper       string
	SessionTTL   time.Duration
	DemoTTL      time.Duration
	EmailCodeTTL time.Duration
}

type Service struct {
	pool   *pgxpool.Pool
	mailer mailer.LoginCodeSender
	cfg    Config
	now    func() time.Time
}

func New(pool *pgxpool.Pool, sender mailer.LoginCodeSender, cfg Config) *Service {
	return &Service{pool: pool, mailer: sender, cfg: cfg, now: func() time.Time { return time.Now().UTC() }}
}

type Actor struct {
	UserID        string     `json:"userId"`
	Kind          string     `json:"kind"`
	Role          string     `json:"role"`
	Email         string     `json:"email,omitempty"`
	WorkspaceID   string     `json:"workspaceId"`
	WorkspaceKind string     `json:"workspaceKind"`
	WorkspaceName string     `json:"workspaceName"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
}

type SessionResult struct {
	Actor Actor
	Token string
	Fresh bool
}

func (service *Service) EnsureSession(ctx context.Context, token string) (SessionResult, error) {
	if token != "" {
		actor, err := service.ActorForToken(ctx, token, true)
		if err == nil {
			return SessionResult{Actor: actor, Token: token}, nil
		}
		if !errors.Is(err, ErrUnauthorized) {
			return SessionResult{}, err
		}
	}
	return service.createDemoSession(ctx)
}

func (service *Service) ActorForToken(ctx context.Context, token string, touch bool) (Actor, error) {
	if token == "" {
		return Actor{}, ErrUnauthorized
	}
	now := service.now()
	digestValue := digest(service.cfg.Pepper, "session", token)
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return Actor{}, err
	}
	defer tx.Rollback(ctx)

	actor, err := scanActor(tx.QueryRow(ctx, `
		SELECT u.id, u.kind, u.role,
		       COALESCE((SELECT ai.provider_subject FROM auth_identities ai WHERE ai.user_id = u.id AND ai.provider = 'email_code' LIMIT 1), ''),
		       w.id, w.kind, w.name, w.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		JOIN workspaces w ON w.id = s.workspace_id
		WHERE s.token_digest = $1 AND s.invalidated_at IS NULL AND s.expires_at > $2
		  AND u.status = 'active'`, digestValue, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return Actor{}, ErrUnauthorized
	}
	if err != nil {
		return Actor{}, err
	}
	if touch {
		expires := now.Add(service.cfg.SessionTTL)
		if actor.Kind == "demo_ephemeral" {
			expires = now.Add(service.cfg.DemoTTL)
			actor.ExpiresAt = &expires
			if _, err = tx.Exec(ctx, `UPDATE workspaces SET last_activity_at=$1, expires_at=$2 WHERE id=$3`, now, expires, actor.WorkspaceID); err != nil {
				return Actor{}, err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE sessions SET last_activity_at=$1, expires_at=$2 WHERE token_digest=$3`, now, expires, digestValue); err != nil {
			return Actor{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE users SET last_activity_at=$1 WHERE id=$2`, now, actor.UserID); err != nil {
			return Actor{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Actor{}, err
	}
	return actor, nil
}

func scanActor(row pgx.Row) (Actor, error) {
	var actor Actor
	err := row.Scan(&actor.UserID, &actor.Kind, &actor.Role, &actor.Email, &actor.WorkspaceID, &actor.WorkspaceKind, &actor.WorkspaceName, &actor.ExpiresAt)
	return actor, err
}

func (service *Service) createDemoSession(ctx context.Context) (SessionResult, error) {
	now := service.now()
	expires := now.Add(service.cfg.DemoTTL)
	userID, err := newUUID()
	if err != nil {
		return SessionResult{}, err
	}
	workspaceID, err := newUUID()
	if err != nil {
		return SessionResult{}, err
	}
	sessionID, err := newUUID()
	if err != nil {
		return SessionResult{}, err
	}
	token, err := randomToken(32)
	if err != nil {
		return SessionResult{}, err
	}

	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return SessionResult{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO users (id,kind,status,role,last_activity_at,created_at) VALUES ($1,'demo_ephemeral','active','member',$2,$2)`, userID, now); err != nil {
		return SessionResult{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workspaces (id,owner_user_id,kind,name,last_activity_at,expires_at,created_at) VALUES ($1,$2,'demo','演示空间',$3,$4,$3)`, workspaceID, userID, now, expires); err != nil {
		return SessionResult{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workspace_members (workspace_id,user_id,role,created_at) VALUES ($1,$2,'owner',$3)`, workspaceID, userID, now); err != nil {
		return SessionResult{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sessions (id,user_id,workspace_id,token_digest,kind,expires_at,last_activity_at,created_at) VALUES ($1,$2,$3,$4,'demo',$5,$6,$6)`, sessionID, userID, workspaceID, digest(service.cfg.Pepper, "session", token), expires, now); err != nil {
		return SessionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SessionResult{}, err
	}
	return SessionResult{Actor: Actor{UserID: userID, Kind: "demo_ephemeral", Role: "member", WorkspaceID: workspaceID, WorkspaceKind: "demo", WorkspaceName: "演示空间", ExpiresAt: &expires}, Token: token, Fresh: true}, nil
}

func normalizeEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized || len(normalized) > 254 {
		return "", NewError("invalid_email", "请输入有效邮箱地址。")
	}
	return normalized, nil
}

func (service *Service) RequestLoginCode(ctx context.Context, email, invitationSecret string) error {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	now := service.now()
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var recent int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM email_challenges WHERE email_normalized=$1 AND created_at > $2`, normalized, now.Add(-10*time.Minute)).Scan(&recent); err != nil {
		return err
	}
	if recent >= 5 {
		return ErrRateLimited
	}

	var userID string
	lookupErr := tx.QueryRow(ctx, `SELECT user_id FROM auth_identities WHERE provider='email_code' AND provider_subject=$1`, normalized).Scan(&userID)
	existing := lookupErr == nil
	if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
		return lookupErr
	}
	var invitationID *string
	if !existing {
		if strings.TrimSpace(invitationSecret) == "" {
			return ErrInvitationRequired
		}
		id, validateErr := service.validInvitation(ctx, tx, normalized, invitationSecret, false)
		if validateErr != nil {
			return validateErr
		}
		invitationID = &id
	}

	code, err := randomCode()
	if err != nil {
		return err
	}
	challengeID, err := newUUID()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO email_challenges (id,email_normalized,purpose,code_digest,invitation_id,expires_at,created_at) VALUES ($1,$2,'sign_in',$3,$4,$5,$6)`, challengeID, normalized, digest(service.cfg.Pepper, "email-code", code), invitationID, now.Add(service.cfg.EmailCodeTTL), now); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}

	if err = service.mailer.SendLoginCode(ctx, normalized, code); err != nil {
		_, _ = service.pool.Exec(ctx, `UPDATE email_challenges SET consumed_at=$1 WHERE id=$2`, service.now(), challengeID)
		return NewError("email_delivery_failed", "验证码邮件发送失败，请稍后重试。")
	}
	return nil
}

func (service *Service) VerifyLoginCode(ctx context.Context, email, code, invitationSecret, password, currentToken string) (SessionResult, error) {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return SessionResult{}, err
	}
	if len(code) != 6 {
		return SessionResult{}, ErrInvalidCredentials
	}
	var passwordHash string
	if password != "" {
		passwordHash, err = hashPassword(password)
		if err != nil {
			return SessionResult{}, NewError("invalid_password", err.Error())
		}
	}
	now := service.now()
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return SessionResult{}, err
	}
	defer tx.Rollback(ctx)

	var challengeID, codeDigest string
	var invitationID *string
	var expires time.Time
	var attempts int
	err = tx.QueryRow(ctx, `SELECT id,code_digest,invitation_id,expires_at,attempts FROM email_challenges WHERE email_normalized=$1 AND purpose='sign_in' AND consumed_at IS NULL ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, normalized).Scan(&challengeID, &codeDigest, &invitationID, &expires, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return SessionResult{}, err
	}
	if attempts >= 5 || !expires.After(now) || !secureEqual(codeDigest, digest(service.cfg.Pepper, "email-code", code)) {
		_, _ = tx.Exec(ctx, `UPDATE email_challenges SET attempts=attempts+1 WHERE id=$1`, challengeID)
		_ = tx.Commit(ctx)
		return SessionResult{}, ErrInvalidCredentials
	}

	var userID, workspaceID string
	err = tx.QueryRow(ctx, `SELECT ai.user_id,w.id FROM auth_identities ai JOIN users u ON u.id=ai.user_id JOIN workspaces w ON w.owner_user_id=u.id AND w.kind='registered' WHERE ai.provider='email_code' AND ai.provider_subject=$1 AND u.status='active'`, normalized).Scan(&userID, &workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		if invitationID == nil || invitationSecret == "" {
			return SessionResult{}, ErrInvitationRequired
		}
		validID, validateErr := service.validInvitation(ctx, tx, normalized, invitationSecret, true)
		if validateErr != nil || validID != *invitationID {
			return SessionResult{}, ErrInvitationInvalid
		}
		userID, err = newUUID()
		if err != nil {
			return SessionResult{}, err
		}
		workspaceID, err = newUUID()
		if err != nil {
			return SessionResult{}, err
		}
		identityID, idErr := newUUID()
		if idErr != nil {
			return SessionResult{}, idErr
		}
		acceptanceID, idErr := newUUID()
		if idErr != nil {
			return SessionResult{}, idErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO users (id,kind,status,role,last_activity_at,created_at) VALUES ($1,'registered','active','member',$2,$2)`, userID, now); err != nil {
			return SessionResult{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO workspaces (id,owner_user_id,kind,name,last_activity_at,created_at) VALUES ($1,$2,'registered','我的空间',$3,$3)`, workspaceID, userID, now); err != nil {
			return SessionResult{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO workspace_members (workspace_id,user_id,role,created_at) VALUES ($1,$2,'owner',$3)`, workspaceID, userID, now); err != nil {
			return SessionResult{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO auth_identities (id,user_id,provider,provider_subject,verified_at,created_at,updated_at) VALUES ($1,$2,'email_code',$3,$4,$4,$4)`, identityID, userID, normalized, now); err != nil {
			return SessionResult{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE invitations SET use_count=use_count+1 WHERE id=$1`, *invitationID); err != nil {
			return SessionResult{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO invitation_acceptances (id,invitation_id,user_id,email_normalized,accepted_at) VALUES ($1,$2,$3,$4,$5)`, acceptanceID, *invitationID, userID, normalized, now); err != nil {
			return SessionResult{}, err
		}
	} else if err != nil {
		return SessionResult{}, err
	}

	if passwordHash != "" {
		passwordID, idErr := newUUID()
		if idErr != nil {
			return SessionResult{}, idErr
		}
		_, err = tx.Exec(ctx, `INSERT INTO auth_identities (id,user_id,provider,provider_subject,secret_hash,verified_at,created_at,updated_at) VALUES ($1,$2,'email_password',$3,$4,$5,$5,$5) ON CONFLICT (provider,provider_subject) DO UPDATE SET secret_hash=EXCLUDED.secret_hash, updated_at=EXCLUDED.updated_at WHERE auth_identities.user_id=EXCLUDED.user_id`, passwordID, userID, normalized, passwordHash, now)
		if err != nil {
			return SessionResult{}, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE email_challenges SET consumed_at=$1 WHERE id=$2`, now, challengeID); err != nil {
		return SessionResult{}, err
	}
	result, err := service.rotateIntoRegistered(ctx, tx, userID, workspaceID, normalized, currentToken, now)
	if err != nil {
		return SessionResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionResult{}, err
	}
	return result, nil
}

func (service *Service) PasswordLogin(ctx context.Context, email, password, currentToken string) (SessionResult, error) {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return SessionResult{}, ErrInvalidCredentials
	}
	var userID, workspaceID, encoded string
	err = service.pool.QueryRow(ctx, `SELECT ai.user_id,w.id,ai.secret_hash FROM auth_identities ai JOIN users u ON u.id=ai.user_id JOIN workspaces w ON w.owner_user_id=u.id AND w.kind='registered' WHERE ai.provider='email_password' AND ai.provider_subject=$1 AND u.status='active'`, normalized).Scan(&userID, &workspaceID, &encoded)
	if err != nil || !verifyPassword(encoded, password) {
		return SessionResult{}, ErrInvalidCredentials
	}
	now := service.now()
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return SessionResult{}, err
	}
	defer tx.Rollback(ctx)
	result, err := service.rotateIntoRegistered(ctx, tx, userID, workspaceID, normalized, currentToken, now)
	if err != nil {
		return SessionResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionResult{}, err
	}
	return result, nil
}

func (service *Service) RequestPasswordReset(ctx context.Context, email string) error {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return nil
	}
	var exists bool
	if err = service.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_identities WHERE provider='email_password' AND provider_subject=$1)`, normalized).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	}
	now := service.now()
	var recent int
	if err = service.pool.QueryRow(ctx, `SELECT count(*) FROM email_challenges WHERE email_normalized=$1 AND purpose='reset_password' AND created_at > $2`, normalized, now.Add(-10*time.Minute)).Scan(&recent); err != nil {
		return err
	}
	if recent >= 5 {
		return ErrRateLimited
	}
	code, err := randomCode()
	if err != nil {
		return err
	}
	id, err := newUUID()
	if err != nil {
		return err
	}
	_, err = service.pool.Exec(ctx, `INSERT INTO email_challenges (id,email_normalized,purpose,code_digest,expires_at,created_at) VALUES ($1,$2,'reset_password',$3,$4,$5)`, id, normalized, digest(service.cfg.Pepper, "email-code", code), now.Add(service.cfg.EmailCodeTTL), now)
	if err != nil {
		return err
	}
	if err = service.mailer.SendLoginCode(ctx, normalized, code); err != nil {
		_, _ = service.pool.Exec(ctx, `UPDATE email_challenges SET consumed_at=$1 WHERE id=$2`, service.now(), id)
		return NewError("email_delivery_failed", "验证码邮件发送失败，请稍后重试。")
	}
	return nil
}

func (service *Service) ConfirmPasswordReset(ctx context.Context, email, code, password string) error {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return ErrInvalidCredentials
	}
	encoded, err := hashPassword(password)
	if err != nil {
		return NewError("invalid_password", err.Error())
	}
	now := service.now()
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var challengeID, codeDigest string
	var expires time.Time
	var attempts int
	err = tx.QueryRow(ctx, `SELECT id,code_digest,expires_at,attempts FROM email_challenges WHERE email_normalized=$1 AND purpose='reset_password' AND consumed_at IS NULL ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, normalized).Scan(&challengeID, &codeDigest, &expires, &attempts)
	if err != nil || attempts >= 5 || !expires.After(now) || !secureEqual(codeDigest, digest(service.cfg.Pepper, "email-code", code)) {
		if err == nil {
			_, _ = tx.Exec(ctx, `UPDATE email_challenges SET attempts=attempts+1 WHERE id=$1`, challengeID)
			_ = tx.Commit(ctx)
		}
		return ErrInvalidCredentials
	}
	var userID string
	if err = tx.QueryRow(ctx, `UPDATE auth_identities SET secret_hash=$1,updated_at=$2 WHERE provider='email_password' AND provider_subject=$3 RETURNING user_id`, encoded, now, normalized).Scan(&userID); err != nil {
		return ErrInvalidCredentials
	}
	if _, err = tx.Exec(ctx, `UPDATE email_challenges SET consumed_at=$1 WHERE id=$2`, now, challengeID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE sessions SET invalidated_at=$1 WHERE user_id=$2 AND invalidated_at IS NULL`, now, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (service *Service) rotateIntoRegistered(ctx context.Context, tx pgx.Tx, userID, workspaceID, email, currentToken string, now time.Time) (SessionResult, error) {
	if currentToken != "" {
		currentDigest := digest(service.cfg.Pepper, "session", currentToken)
		var oldUserID, oldKind string
		err := tx.QueryRow(ctx, `SELECT s.user_id,u.kind FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_digest=$1 AND s.invalidated_at IS NULL FOR UPDATE`, currentDigest).Scan(&oldUserID, &oldKind)
		if err == nil {
			_, _ = tx.Exec(ctx, `UPDATE sessions SET invalidated_at=$1 WHERE token_digest=$2`, now, currentDigest)
			if oldKind == "demo_ephemeral" {
				jobID, idErr := newUUID()
				if idErr != nil {
					return SessionResult{}, idErr
				}
				_, err = tx.Exec(ctx, `UPDATE users SET status='pending_deletion' WHERE id=$1`, oldUserID)
				if err != nil {
					return SessionResult{}, err
				}
				_, err = tx.Exec(ctx, `INSERT INTO demo_cleanup_jobs (id,user_id,run_after,status,created_at,updated_at) VALUES ($1,$2,$3,'pending',$3,$3) ON CONFLICT (user_id) DO UPDATE SET run_after=EXCLUDED.run_after,status='pending',updated_at=EXCLUDED.updated_at`, jobID, oldUserID, now)
				if err != nil {
					return SessionResult{}, err
				}
			}
		}
	}
	sessionID, err := newUUID()
	if err != nil {
		return SessionResult{}, err
	}
	token, err := randomToken(32)
	if err != nil {
		return SessionResult{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sessions (id,user_id,workspace_id,token_digest,kind,expires_at,last_activity_at,created_at) VALUES ($1,$2,$3,$4,'registered',$5,$6,$6)`, sessionID, userID, workspaceID, digest(service.cfg.Pepper, "session", token), now.Add(service.cfg.SessionTTL), now); err != nil {
		return SessionResult{}, err
	}
	var role string
	if err = tx.QueryRow(ctx, `SELECT role FROM users WHERE id=$1`, userID).Scan(&role); err != nil {
		return SessionResult{}, err
	}
	return SessionResult{Actor: Actor{UserID: userID, Kind: "registered", Role: role, Email: email, WorkspaceID: workspaceID, WorkspaceKind: "registered", WorkspaceName: "我的空间"}, Token: token, Fresh: true}, nil
}

func (service *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := service.pool.Exec(ctx, `UPDATE sessions SET invalidated_at=$1 WHERE token_digest=$2 AND invalidated_at IS NULL`, service.now(), digest(service.cfg.Pepper, "session", token))
	return err
}

func (service *Service) validInvitation(ctx context.Context, tx pgx.Tx, email, secret string, lock bool) (string, error) {
	query := `SELECT id,kind,email_normalized,max_uses,use_count,expires_at,revoked_at FROM invitations WHERE secret_digest=$1`
	if lock {
		query += ` FOR UPDATE`
	}
	var id, kind string
	var boundEmail *string
	var maxUses, useCount int
	var expires time.Time
	var revoked *time.Time
	err := tx.QueryRow(ctx, query, digest(service.cfg.Pepper, "invitation", strings.TrimSpace(secret))).Scan(&id, &kind, &boundEmail, &maxUses, &useCount, &expires, &revoked)
	if err != nil || revoked != nil || !expires.After(service.now()) || useCount >= maxUses {
		return "", ErrInvitationInvalid
	}
	if kind == "email_bound" && (boundEmail == nil || *boundEmail != email) {
		return "", ErrInvitationInvalid
	}
	return id, nil
}

type CreateInvitationInput struct {
	Kind      string    `json:"kind"`
	Email     string    `json:"email,omitempty"`
	MaxUses   int       `json:"maxUses"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Invitation struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	Email     string     `json:"email,omitempty"`
	MaxUses   int        `json:"maxUses"`
	UseCount  int        `json:"useCount"`
	ExpiresAt time.Time  `json:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type CreatedInvitation struct {
	Invitation
	Secret string `json:"secret"`
}

func (service *Service) CreateInvitation(ctx context.Context, actor Actor, input CreateInvitationInput) (CreatedInvitation, error) {
	if actor.Kind != "registered" || actor.Role != "admin" {
		return CreatedInvitation{}, ErrForbidden
	}
	if input.Kind != "generic_code" && input.Kind != "email_bound" {
		return CreatedInvitation{}, NewError("invalid_invitation_kind", "邀请类型无效。")
	}
	if !input.ExpiresAt.After(service.now()) {
		return CreatedInvitation{}, NewError("invalid_expiration", "邀请过期时间必须晚于当前时间。")
	}
	var email string
	var err error
	if input.Kind == "email_bound" {
		email, err = normalizeEmail(input.Email)
		if err != nil {
			return CreatedInvitation{}, err
		}
		input.MaxUses = 1
	} else {
		if input.MaxUses < 1 || input.MaxUses > 1000 {
			return CreatedInvitation{}, NewError("invalid_max_uses", "通用邀请码可用次数必须在 1 到 1000 之间。")
		}
	}
	id, err := newUUID()
	if err != nil {
		return CreatedInvitation{}, err
	}
	raw, err := randomToken(18)
	if err != nil {
		return CreatedInvitation{}, err
	}
	secret := raw
	if input.Kind == "generic_code" {
		secret = "SQ-" + strings.ToUpper(raw[:12])
	}
	now := service.now()
	_, err = service.pool.Exec(ctx, `INSERT INTO invitations (id,kind,secret_digest,email_normalized,max_uses,expires_at,created_by,created_at) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8)`, id, input.Kind, digest(service.cfg.Pepper, "invitation", secret), email, input.MaxUses, input.ExpiresAt, actor.UserID, now)
	if err != nil {
		return CreatedInvitation{}, err
	}
	return CreatedInvitation{Invitation: Invitation{ID: id, Kind: input.Kind, Email: email, MaxUses: input.MaxUses, ExpiresAt: input.ExpiresAt, CreatedAt: now}, Secret: secret}, nil
}

func (service *Service) ListInvitations(ctx context.Context, actor Actor) ([]Invitation, error) {
	if actor.Kind != "registered" || actor.Role != "admin" {
		return nil, ErrForbidden
	}
	rows, err := service.pool.Query(ctx, `SELECT id,kind,COALESCE(email_normalized,''),max_uses,use_count,expires_at,revoked_at,created_at FROM invitations ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Invitation, 0)
	for rows.Next() {
		var item Invitation
		if err = rows.Scan(&item.ID, &item.Kind, &item.Email, &item.MaxUses, &item.UseCount, &item.ExpiresAt, &item.RevokedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (service *Service) RevokeInvitation(ctx context.Context, actor Actor, id string) error {
	if actor.Kind != "registered" || actor.Role != "admin" {
		return ErrForbidden
	}
	tag, err := service.pool.Exec(ctx, `UPDATE invitations SET revoked_at=$1 WHERE id=$2 AND revoked_at IS NULL`, service.now(), id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return NewError("invitation_not_found", "邀请不存在。")
	}
	return nil
}

func (service *Service) BootstrapAdmin(ctx context.Context, email string) error {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	now := service.now()
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, `SELECT user_id FROM auth_identities WHERE provider='email_code' AND provider_subject=$1`, normalized).Scan(&userID)
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE users SET role='admin' WHERE id=$1`, userID)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	userID, err = newUUID()
	if err != nil {
		return err
	}
	workspaceID, err := newUUID()
	if err != nil {
		return err
	}
	identityID, err := newUUID()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO users (id,kind,status,role,last_activity_at,created_at) VALUES ($1,'registered','active','admin',$2,$2)`, userID, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workspaces (id,owner_user_id,kind,name,last_activity_at,created_at) VALUES ($1,$2,'registered','我的空间',$3,$3)`, workspaceID, userID, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO workspace_members (workspace_id,user_id,role,created_at) VALUES ($1,$2,'owner',$3)`, workspaceID, userID, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO auth_identities (id,user_id,provider,provider_subject,verified_at,created_at,updated_at) VALUES ($1,$2,'email_code',$3,$4,$4,$4)`, identityID, userID, normalized, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (service *Service) CleanupExpiredDemos(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	now := service.now()
	if _, err := service.pool.Exec(ctx, `INSERT INTO demo_cleanup_jobs (id,user_id,run_after,status,created_at,updated_at)
		SELECT gen_random_uuid(),u.id,$1,'pending',$1,$1 FROM users u
		WHERE u.kind='demo_ephemeral' AND u.status='active' AND u.last_activity_at < $2
		ON CONFLICT (user_id) DO NOTHING`, now, now.Add(-service.cfg.DemoTTL)); err != nil {
		return 0, err
	}
	rows, err := service.pool.Query(ctx, `SELECT user_id FROM demo_cleanup_jobs WHERE run_after <= $1 AND status IN ('pending','failed') ORDER BY run_after LIMIT $2`, now, limit)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	deleted := 0
	for _, id := range ids {
		tag, deleteErr := service.pool.Exec(ctx, `DELETE FROM users WHERE id=$1 AND kind='demo_ephemeral'`, id)
		if deleteErr != nil {
			_, _ = service.pool.Exec(ctx, `UPDATE demo_cleanup_jobs SET status='failed',attempts=attempts+1,last_error=$1,updated_at=$2 WHERE user_id=$3`, deleteErr.Error(), now, id)
			continue
		}
		deleted += int(tag.RowsAffected())
	}
	return deleted, nil
}

func (service *Service) String() string {
	return fmt.Sprintf("identity service session=%s demo=%s", service.cfg.SessionTTL, service.cfg.DemoTTL)
}
