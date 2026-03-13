package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"llm-gateway/backend/services/identity-service/internal/domain"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) EnsureSchema(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS users (
    id            VARCHAR(64) PRIMARY KEY,
    username      VARCHAR(64) NOT NULL UNIQUE,
    display_name  VARCHAR(128) NOT NULL,
    role          VARCHAR(32) NOT NULL,
    parent_id     VARCHAR(64),
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    VARCHAR(64),
    updated_by    VARCHAR(64)
);
CREATE INDEX IF NOT EXISTS idx_users_parent_id ON users(parent_id);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_single_superuser ON users ((role)) WHERE role = 'superuser';
`)
	return err
}

func (r *UserRepository) CreateUser(ctx context.Context, user domain.User) error {
	parent := nullable(user.ParentID)
	createdAt := user.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	_, err := r.pool.Exec(ctx, `
INSERT INTO users (id, username, display_name, role, parent_id, password_hash, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, user.ID, user.Username, user.DisplayName, string(user.Role), parent, user.PasswordHash, createdAt, createdAt)
	if err != nil {
		if violation, ok := uniqueViolationConstraint(err); ok {
			switch violation {
			case "uq_users_single_superuser":
				return domain.ErrSuperuserAlreadyExists
			case "users_username_key":
				return domain.ErrUsernameTaken
			default:
				return domain.ErrUsernameTaken
			}
		}
		return err
	}
	return nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	return r.getUser(ctx, `
SELECT id, username, display_name, role, parent_id, password_hash, created_at
FROM users
WHERE id = $1
`, strings.TrimSpace(id))
}

func (r *UserRepository) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	return r.getUser(ctx, `
SELECT id, username, display_name, role, parent_id, password_hash, created_at
FROM users
WHERE username = $1
`, strings.TrimSpace(username))
}

func (r *UserRepository) SuperuserExists(ctx context.Context) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE role = $1)`, string(domain.RoleSuperuser)).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *UserRepository) getUser(ctx context.Context, query string, arg string) (domain.User, error) {
	var (
		u      domain.User
		role   string
		parent sql.NullString
	)
	err := r.pool.QueryRow(ctx, query, arg).Scan(
		&u.ID,
		&u.Username,
		&u.DisplayName,
		&role,
		&parent,
		&u.PasswordHash,
		&u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, err
	}
	u.Role = domain.Role(role)
	if parent.Valid {
		u.ParentID = parent.String
	}
	return u, nil
}

func nullable(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func uniqueViolationConstraint(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return pgErr.ConstraintName, true
		}
	}
	return "", false
}
