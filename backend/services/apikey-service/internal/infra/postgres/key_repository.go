package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"llm-gateway/backend/services/apikey-service/internal/domain"
)

type KeyRepository struct {
	pool *pgxpool.Pool
}

func NewKeyRepository(pool *pgxpool.Pool) *KeyRepository {
	return &KeyRepository{pool: pool}
}

func (r *KeyRepository) EnsureSchema(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS api_keys (
    id            VARCHAR(64) PRIMARY KEY,
    owner_id      VARCHAR(64) NOT NULL,
    name          VARCHAR(128) NOT NULL,
    secret_hash   TEXT NOT NULL UNIQUE,
    status        VARCHAR(32) NOT NULL,
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by    VARCHAR(64),
    updated_by    VARCHAR(64)
);
CREATE TABLE IF NOT EXISTS api_key_model_whitelist (
    api_key_id    VARCHAR(64) NOT NULL,
    model_name    VARCHAR(128) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (api_key_id, model_name)
);
CREATE INDEX IF NOT EXISTS idx_api_keys_owner_id ON api_keys(owner_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);
`)
	return err
}

func (r *KeyRepository) CreateKey(ctx context.Context, key domain.APIKey) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	expiresAt := nullableTime(key.ExpiresAt)
	_, err = tx.Exec(ctx, `
INSERT INTO api_keys (id, owner_id, name, secret_hash, status, expires_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`, key.ID, key.OwnerID, key.Name, key.SecretHash, string(key.Status), expiresAt, key.CreatedAt.UTC(), key.UpdatedAt.UTC())
	if err != nil {
		return err
	}

	for _, model := range key.AllowedModels {
		if _, err := tx.Exec(ctx, `
INSERT INTO api_key_model_whitelist (api_key_id, model_name, created_at)
VALUES ($1, $2, $3)
`, key.ID, model, key.CreatedAt.UTC()); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (r *KeyRepository) UpdateKeyStatus(ctx context.Context, id string, status domain.KeyStatus, updatedAt time.Time) (domain.APIKey, error) {
	var (
		record  domain.APIKey
		expires sql.NullTime
		state   string
	)
	err := r.pool.QueryRow(ctx, `
UPDATE api_keys
SET status = $2, updated_at = $3
WHERE id = $1
RETURNING id, owner_id, name, secret_hash, status, expires_at, created_at, updated_at
`, strings.TrimSpace(id), string(status), updatedAt.UTC()).Scan(
		&record.ID,
		&record.OwnerID,
		&record.Name,
		&record.SecretHash,
		&state,
		&expires,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.APIKey{}, domain.ErrKeyNotFound
		}
		return domain.APIKey{}, err
	}
	record.Status = domain.KeyStatus(state)
	if expires.Valid {
		exp := expires.Time.UTC()
		record.ExpiresAt = &exp
	}

	models, err := r.loadModels(ctx, record.ID)
	if err != nil {
		return domain.APIKey{}, err
	}
	record.AllowedModels = models
	return record, nil
}

func (r *KeyRepository) GetKeyByID(ctx context.Context, id string) (domain.APIKey, error) {
	return r.getKey(ctx, `
SELECT id, owner_id, name, secret_hash, status, expires_at, created_at, updated_at
FROM api_keys
WHERE id = $1
`, strings.TrimSpace(id))
}

func (r *KeyRepository) GetKeyBySecretHash(ctx context.Context, secretHash string) (domain.APIKey, error) {
	return r.getKey(ctx, `
SELECT id, owner_id, name, secret_hash, status, expires_at, created_at, updated_at
FROM api_keys
WHERE secret_hash = $1
`, strings.TrimSpace(secretHash))
}

func (r *KeyRepository) getKey(ctx context.Context, query string, arg string) (domain.APIKey, error) {
	var (
		record  domain.APIKey
		expires sql.NullTime
		state   string
	)
	err := r.pool.QueryRow(ctx, query, arg).Scan(
		&record.ID,
		&record.OwnerID,
		&record.Name,
		&record.SecretHash,
		&state,
		&expires,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.APIKey{}, domain.ErrKeyNotFound
		}
		return domain.APIKey{}, err
	}
	record.Status = domain.KeyStatus(state)
	if expires.Valid {
		exp := expires.Time.UTC()
		record.ExpiresAt = &exp
	}

	models, err := r.loadModels(ctx, record.ID)
	if err != nil {
		return domain.APIKey{}, err
	}
	record.AllowedModels = models
	return record, nil
}

func (r *KeyRepository) loadModels(ctx context.Context, keyID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
SELECT model_name
FROM api_key_model_whitelist
WHERE api_key_id = $1
ORDER BY model_name ASC
`, keyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := make([]string, 0)
	for rows.Next() {
		var model string
		if err := rows.Scan(&model); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return models, nil
}

func nullableTime(v *time.Time) any {
	if v == nil {
		return nil
	}
	t := v.UTC()
	return t
}
