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

	"llm-gateway/backend/services/execution-service/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) EnsureSchema(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS execution_providers (
    id          VARCHAR(64) PRIMARY KEY,
    owner_id    VARCHAR(64) NOT NULL,
    name        VARCHAR(128) NOT NULL,
    protocol    VARCHAR(32) NOT NULL,
    base_url    TEXT NOT NULL,
    api_key     TEXT NOT NULL,
    status      VARCHAR(32) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_id, name)
);

CREATE TABLE IF NOT EXISTS execution_models (
    id              VARCHAR(64) PRIMARY KEY,
    provider_id     VARCHAR(64) NOT NULL REFERENCES execution_providers(id) ON DELETE CASCADE,
    owner_id        VARCHAR(64) NOT NULL,
    name            VARCHAR(128) NOT NULL,
    upstream_model  VARCHAR(128) NOT NULL,
    status          VARCHAR(32) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider_id, name)
);

CREATE INDEX IF NOT EXISTS idx_execution_providers_owner ON execution_providers(owner_id);
CREATE INDEX IF NOT EXISTS idx_execution_providers_status ON execution_providers(status);
CREATE INDEX IF NOT EXISTS idx_execution_models_provider ON execution_models(provider_id);
CREATE INDEX IF NOT EXISTS idx_execution_models_owner ON execution_models(owner_id);
CREATE INDEX IF NOT EXISTS idx_execution_models_status ON execution_models(status);
`)
	return err
}

func (r *Repository) CreateProvider(ctx context.Context, provider domain.Provider) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO execution_providers (id, owner_id, name, protocol, base_url, api_key, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`, provider.ID, provider.OwnerID, provider.Name, provider.Protocol, provider.BaseURL, provider.APIKey, string(provider.Status), provider.CreatedAt.UTC(), provider.UpdatedAt.UTC())
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrProviderNameTaken
		}
		return err
	}
	return nil
}

func (r *Repository) GetProviderByID(ctx context.Context, id string) (domain.Provider, error) {
	var (
		provider domain.Provider
		status   string
	)
	err := r.pool.QueryRow(ctx, `
SELECT id, owner_id, name, protocol, base_url, api_key, status, created_at, updated_at
FROM execution_providers
WHERE id = $1
`, strings.TrimSpace(id)).Scan(
		&provider.ID,
		&provider.OwnerID,
		&provider.Name,
		&provider.Protocol,
		&provider.BaseURL,
		&provider.APIKey,
		&status,
		&provider.CreatedAt,
		&provider.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Provider{}, domain.ErrProviderNotFound
		}
		return domain.Provider{}, err
	}
	provider.Status = domain.ProviderStatus(status)
	return provider, nil
}

func (r *Repository) ListProviders(ctx context.Context, ownerID string) ([]domain.Provider, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id, owner_id, name, protocol, base_url, api_key, status, created_at, updated_at
FROM execution_providers
WHERE ($1 = '' OR owner_id = $1)
ORDER BY created_at ASC
`, strings.TrimSpace(ownerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.Provider, 0)
	for rows.Next() {
		var (
			provider domain.Provider
			status   string
		)
		if err := rows.Scan(
			&provider.ID,
			&provider.OwnerID,
			&provider.Name,
			&provider.Protocol,
			&provider.BaseURL,
			&provider.APIKey,
			&status,
			&provider.CreatedAt,
			&provider.UpdatedAt,
		); err != nil {
			return nil, err
		}
		provider.Status = domain.ProviderStatus(status)
		list = append(list, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *Repository) UpdateProviderStatus(ctx context.Context, id string, status domain.ProviderStatus, updatedAt time.Time) (domain.Provider, error) {
	var (
		provider domain.Provider
		state    string
	)
	err := r.pool.QueryRow(ctx, `
UPDATE execution_providers
SET status = $2, updated_at = $3
WHERE id = $1
RETURNING id, owner_id, name, protocol, base_url, api_key, status, created_at, updated_at
`, strings.TrimSpace(id), string(status), updatedAt.UTC()).Scan(
		&provider.ID,
		&provider.OwnerID,
		&provider.Name,
		&provider.Protocol,
		&provider.BaseURL,
		&provider.APIKey,
		&state,
		&provider.CreatedAt,
		&provider.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Provider{}, domain.ErrProviderNotFound
		}
		return domain.Provider{}, err
	}
	provider.Status = domain.ProviderStatus(state)
	return provider, nil
}

func (r *Repository) CreateModel(ctx context.Context, model domain.Model) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO execution_models (id, provider_id, owner_id, name, upstream_model, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`, model.ID, model.ProviderID, model.OwnerID, model.Name, model.UpstreamModel, string(model.Status), model.CreatedAt.UTC(), model.UpdatedAt.UTC())
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrModelNameTaken
		}
		return err
	}
	return nil
}

func (r *Repository) GetModelByID(ctx context.Context, id string) (domain.Model, error) {
	var (
		model  domain.Model
		status string
	)
	err := r.pool.QueryRow(ctx, `
SELECT id, provider_id, owner_id, name, upstream_model, status, created_at, updated_at
FROM execution_models
WHERE id = $1
`, strings.TrimSpace(id)).Scan(
		&model.ID,
		&model.ProviderID,
		&model.OwnerID,
		&model.Name,
		&model.UpstreamModel,
		&status,
		&model.CreatedAt,
		&model.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Model{}, domain.ErrModelNotFound
		}
		return domain.Model{}, err
	}
	model.Status = domain.ModelStatus(status)
	return model, nil
}

func (r *Repository) ListModels(ctx context.Context, providerID string, ownerID string) ([]domain.Model, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id, provider_id, owner_id, name, upstream_model, status, created_at, updated_at
FROM execution_models
WHERE ($1 = '' OR provider_id = $1)
  AND ($2 = '' OR owner_id = $2)
ORDER BY created_at ASC
`, strings.TrimSpace(providerID), strings.TrimSpace(ownerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.Model, 0)
	for rows.Next() {
		var (
			model  domain.Model
			status string
		)
		if err := rows.Scan(
			&model.ID,
			&model.ProviderID,
			&model.OwnerID,
			&model.Name,
			&model.UpstreamModel,
			&status,
			&model.CreatedAt,
			&model.UpdatedAt,
		); err != nil {
			return nil, err
		}
		model.Status = domain.ModelStatus(status)
		list = append(list, model)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *Repository) UpdateModelStatus(ctx context.Context, id string, status domain.ModelStatus, updatedAt time.Time) (domain.Model, error) {
	var (
		model domain.Model
		state string
	)
	err := r.pool.QueryRow(ctx, `
UPDATE execution_models
SET status = $2, updated_at = $3
WHERE id = $1
RETURNING id, provider_id, owner_id, name, upstream_model, status, created_at, updated_at
`, strings.TrimSpace(id), string(status), updatedAt.UTC()).Scan(
		&model.ID,
		&model.ProviderID,
		&model.OwnerID,
		&model.Name,
		&model.UpstreamModel,
		&state,
		&model.CreatedAt,
		&model.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Model{}, domain.ErrModelNotFound
		}
		return domain.Model{}, err
	}
	model.Status = domain.ModelStatus(state)
	return model, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func nullableTime(v *time.Time) sql.NullTime {
	if v == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: v.UTC(), Valid: true}
}
