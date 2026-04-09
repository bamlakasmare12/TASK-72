package repository

import (
	"context"
	"time"

	"wlpr-portal/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, username, email, password_hash, display_name,
			   mfa_enabled, mfa_secret_enc, mfa_recovery_enc,
			   job_family, department, cost_center,
			   is_active, failed_login_count, locked_until,
			   last_login_at, created_at, updated_at
		FROM users WHERE username = $1
	`, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.DisplayName,
		&user.MFAEnabled, &user.MFASecretEnc, &user.MFARecoveryEnc,
		&user.JobFamily, &user.Department, &user.CostCenter,
		&user.IsActive, &user.FailedLoginCount, &user.LockedUntil,
		&user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return user, err
}

func (r *UserRepository) FindByID(ctx context.Context, id int) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, username, email, password_hash, display_name,
			   mfa_enabled, mfa_secret_enc, mfa_recovery_enc,
			   job_family, department, cost_center,
			   is_active, failed_login_count, locked_until,
			   last_login_at, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.DisplayName,
		&user.MFAEnabled, &user.MFASecretEnc, &user.MFARecoveryEnc,
		&user.JobFamily, &user.Department, &user.CostCenter,
		&user.IsActive, &user.FailedLoginCount, &user.LockedUntil,
		&user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return user, err
}

func (r *UserRepository) GetUserRoles(ctx context.Context, userID int) ([]models.Role, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.id, r.name, COALESCE(r.description, '')
		FROM roles r
		JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]models.Role, 0)
	for rows.Next() {
		var role models.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Description); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *UserRepository) GetUserPermissions(ctx context.Context, userID int) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT p.code
		FROM permissions p
		JOIN role_permissions rp ON rp.permission_id = p.id
		JOIN user_roles ur ON ur.role_id = rp.role_id
		WHERE ur.user_id = $1
		ORDER BY p.code
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	perms := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		perms = append(perms, code)
	}
	return perms, rows.Err()
}

func (r *UserRepository) GetUserWithRoles(ctx context.Context, userID int) (*models.UserWithRoles, error) {
	user, err := r.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, err
	}

	roles, err := r.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	perms, err := r.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &models.UserWithRoles{
		User:        *user,
		Roles:       roles,
		Permissions: perms,
	}, nil
}

// CreateUser inserts a new user and returns the created user with ID.
func (r *UserRepository) CreateUser(ctx context.Context, username, email, passwordHash, displayName string, jobFamily, department, costCenter *string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, display_name, job_family, department, cost_center, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE)
		RETURNING id, username, email, password_hash, display_name,
			mfa_enabled, mfa_secret_enc, mfa_recovery_enc,
			job_family, department, cost_center,
			is_active, failed_login_count, locked_until,
			last_login_at, created_at, updated_at
	`, username, email, passwordHash, displayName, jobFamily, department, costCenter).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.DisplayName,
		&user.MFAEnabled, &user.MFASecretEnc, &user.MFARecoveryEnc,
		&user.JobFamily, &user.Department, &user.CostCenter,
		&user.IsActive, &user.FailedLoginCount, &user.LockedUntil,
		&user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	return user, err
}

// AssignRole assigns a role to a user by role name.
func (r *UserRepository) AssignRole(ctx context.Context, userID int, roleName string, grantedBy *int) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id, granted_by)
		SELECT $1, r.id, $3
		FROM roles r WHERE r.name = $2
		ON CONFLICT (user_id, role_id) DO NOTHING
	`, userID, roleName, grantedBy)
	return err
}

// CountUsers returns the total number of registered users.
func (r *UserRepository) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// GetAllUsers returns all users (for admin management).
func (r *UserRepository) GetAllUsers(ctx context.Context) ([]models.UserWithRoles, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, username, email, display_name, mfa_enabled,
			   job_family, department, cost_center,
			   is_active, created_at, updated_at
		FROM users ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]models.UserWithRoles, 0)
	for rows.Next() {
		var u models.User
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.MFAEnabled,
			&u.JobFamily, &u.Department, &u.CostCenter,
			&u.IsActive, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		roles, _ := r.GetUserRoles(ctx, u.ID)
		perms, _ := r.GetUserPermissions(ctx, u.ID)
		users = append(users, models.UserWithRoles{User: u, Roles: roles, Permissions: perms})
	}
	return users, rows.Err()
}

func (r *UserRepository) IncrementFailedLogin(ctx context.Context, userID int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET failed_login_count = failed_login_count + 1,
			locked_until = CASE
				WHEN failed_login_count + 1 >= 5 THEN NOW() + INTERVAL '15 minutes'
				ELSE locked_until
			END,
			updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
}

func (r *UserRepository) ResetFailedLogin(ctx context.Context, userID int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET failed_login_count = 0, locked_until = NULL,
			last_login_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
}

func (r *UserRepository) SetMFASecret(ctx context.Context, userID int, encryptedSecret []byte) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET mfa_secret_enc = $2, updated_at = NOW()
		WHERE id = $1
	`, userID, encryptedSecret)
	return err
}

func (r *UserRepository) EnableMFA(ctx context.Context, userID int, encryptedRecovery []byte) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET mfa_enabled = TRUE, mfa_recovery_enc = $2, updated_at = NOW()
		WHERE id = $1
	`, userID, encryptedRecovery)
	return err
}

func (r *UserRepository) DisableMFA(ctx context.Context, userID int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET mfa_enabled = FALSE, mfa_secret_enc = NULL,
			mfa_recovery_enc = NULL, updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
}

// Session management

func (r *UserRepository) CreateSession(ctx context.Context, session *models.Session) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, status, ip_address, user_agent,
			created_at, last_active_at, expires_at, idle_timeout_s, max_lifetime_s)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, session.ID, session.UserID, session.TokenHash, session.Status,
		session.IPAddress, session.UserAgent,
		session.CreatedAt, session.LastActiveAt, session.ExpiresAt,
		session.IdleTimeoutS, session.MaxLifetimeS)
	return err
}

func (r *UserRepository) GetSession(ctx context.Context, sessionID string) (*models.Session, error) {
	s := &models.Session{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, token_hash, status, COALESCE(ip_address::text, ''),
			   COALESCE(user_agent, ''), created_at, last_active_at, expires_at,
			   idle_timeout_s, max_lifetime_s
		FROM sessions WHERE id = $1
	`, sessionID).Scan(
		&s.ID, &s.UserID, &s.TokenHash, &s.Status, &s.IPAddress,
		&s.UserAgent, &s.CreatedAt, &s.LastActiveAt, &s.ExpiresAt,
		&s.IdleTimeoutS, &s.MaxLifetimeS,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return s, err
}

func (r *UserRepository) TouchSession(ctx context.Context, sessionID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE sessions SET last_active_at = NOW() WHERE id = $1 AND status = 'active'
	`, sessionID)
	return err
}

func (r *UserRepository) RevokeSession(ctx context.Context, sessionID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE sessions SET status = 'revoked' WHERE id = $1
	`, sessionID)
	return err
}

func (r *UserRepository) RevokeAllUserSessions(ctx context.Context, userID int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE sessions SET status = 'revoked'
		WHERE user_id = $1 AND status = 'active'
	`, userID)
	return err
}

func (r *UserRepository) CleanExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE sessions SET status = 'expired'
		WHERE status = 'active' AND (
			expires_at < NOW() OR
			last_active_at + (idle_timeout_s || ' seconds')::interval < NOW()
		)
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Audit logging

func (r *UserRepository) LogAudit(ctx context.Context, userID int, action, module, entityType string, entityID int, ipAddress string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO audit_log (user_id, action, module, entity_type, entity_id, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, action, module, entityType, entityID, ipAddress)
	return err
}

// IsSessionValid checks idle timeout and max lifetime.
func IsSessionValid(session *models.Session) bool {
	now := time.Now()
	if session.Status != "active" {
		return false
	}
	if now.After(session.ExpiresAt) {
		return false
	}
	idleDeadline := session.LastActiveAt.Add(time.Duration(session.IdleTimeoutS) * time.Second)
	if now.After(idleDeadline) {
		return false
	}
	maxDeadline := session.CreatedAt.Add(time.Duration(session.MaxLifetimeS) * time.Second)
	if now.After(maxDeadline) {
		return false
	}
	return true
}
