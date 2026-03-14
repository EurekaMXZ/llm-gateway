package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	execapp "llm-gateway/backend/services/execution-service/internal/app"
	"llm-gateway/backend/services/execution-service/internal/domain"
)

const defaultProviderPriority = 100

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
    priority    INTEGER NOT NULL DEFAULT 100,
    status      VARCHAR(32) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_id, name)
);
ALTER TABLE execution_providers
    ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 100;

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
CREATE INDEX IF NOT EXISTS idx_execution_providers_priority ON execution_providers(priority);
CREATE INDEX IF NOT EXISTS idx_execution_models_provider ON execution_models(provider_id);
CREATE INDEX IF NOT EXISTS idx_execution_models_owner ON execution_models(owner_id);
CREATE INDEX IF NOT EXISTS idx_execution_models_status ON execution_models(status);
`)
	return err
}

func (r *Repository) CreateProvider(ctx context.Context, provider domain.Provider) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO execution_providers (id, owner_id, name, protocol, base_url, api_key, priority, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`, provider.ID, provider.OwnerID, provider.Name, provider.Protocol, provider.BaseURL, provider.APIKey, provider.Priority, string(provider.Status), provider.CreatedAt.UTC(), provider.UpdatedAt.UTC())
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
SELECT id, owner_id, name, protocol, base_url, api_key, priority, status, created_at, updated_at
FROM execution_providers
WHERE id = $1
`, strings.TrimSpace(id)).Scan(
		&provider.ID,
		&provider.OwnerID,
		&provider.Name,
		&provider.Protocol,
		&provider.BaseURL,
		&provider.APIKey,
		&provider.Priority,
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
SELECT id, owner_id, name, protocol, base_url, api_key, priority, status, created_at, updated_at
FROM execution_providers
WHERE ($1 = '' OR owner_id = $1)
ORDER BY priority ASC, created_at ASC
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
			&provider.Priority,
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
RETURNING id, owner_id, name, protocol, base_url, api_key, priority, status, created_at, updated_at
`, strings.TrimSpace(id), string(status), updatedAt.UTC()).Scan(
		&provider.ID,
		&provider.OwnerID,
		&provider.Name,
		&provider.Protocol,
		&provider.BaseURL,
		&provider.APIKey,
		&provider.Priority,
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

func (r *Repository) UpdateProviderPriority(ctx context.Context, id string, priority int, updatedAt time.Time) (domain.Provider, error) {
	var (
		provider domain.Provider
		status   string
	)
	err := r.pool.QueryRow(ctx, `
UPDATE execution_providers
SET priority = $2, updated_at = $3
WHERE id = $1
RETURNING id, owner_id, name, protocol, base_url, api_key, priority, status, created_at, updated_at
`, strings.TrimSpace(id), priority, updatedAt.UTC()).Scan(
		&provider.ID,
		&provider.OwnerID,
		&provider.Name,
		&provider.Protocol,
		&provider.BaseURL,
		&provider.APIKey,
		&provider.Priority,
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

func (r *Repository) ResolveExecutionTarget(ctx context.Context, ownerID string, modelName string, providerID string) (execapp.ExecutionTarget, error) {
	ownerID = strings.TrimSpace(ownerID)
	modelName = strings.TrimSpace(modelName)
	providerID = strings.TrimSpace(providerID)
	if ownerID == "" || modelName == "" {
		return execapp.ExecutionTarget{}, domain.ErrInvalidInput
	}

	var (
		target execapp.ExecutionTarget
		pState string
		mState string
	)

	if providerID != "" {
		err := r.pool.QueryRow(ctx, `
SELECT id, owner_id, name, protocol, base_url, api_key, priority, status, created_at, updated_at
FROM execution_providers
WHERE id = $1 AND owner_id = $2
`, providerID, ownerID).Scan(
			&target.Provider.ID,
			&target.Provider.OwnerID,
			&target.Provider.Name,
			&target.Provider.Protocol,
			&target.Provider.BaseURL,
			&target.Provider.APIKey,
			&target.Provider.Priority,
			&pState,
			&target.Provider.CreatedAt,
			&target.Provider.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return execapp.ExecutionTarget{}, domain.ErrProviderNotFound
			}
			return execapp.ExecutionTarget{}, err
		}
		target.Provider.Status = domain.ProviderStatus(pState)
		if target.Provider.Status != domain.ProviderStatusEnabled {
			return execapp.ExecutionTarget{}, domain.ErrProviderDisabled
		}

		err = r.pool.QueryRow(ctx, `
SELECT id, provider_id, owner_id, name, upstream_model, status, created_at, updated_at
FROM execution_models
WHERE provider_id = $1
  AND owner_id = $2
  AND name = $3
  AND status = 'enabled'
ORDER BY created_at ASC
LIMIT 1
`, providerID, ownerID, modelName).Scan(
			&target.Model.ID,
			&target.Model.ProviderID,
			&target.Model.OwnerID,
			&target.Model.Name,
			&target.Model.UpstreamModel,
			&mState,
			&target.Model.CreatedAt,
			&target.Model.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return execapp.ExecutionTarget{}, domain.ErrModelNotFound
			}
			return execapp.ExecutionTarget{}, err
		}
		target.Model.Status = domain.ModelStatus(mState)
		return target, nil
	}

	query := `
SELECT
  p.id, p.owner_id, p.name, p.protocol, p.base_url, p.api_key, p.priority, p.status, p.created_at, p.updated_at,
  m.id, m.provider_id, m.owner_id, m.name, m.upstream_model, m.status, m.created_at, m.updated_at
FROM execution_models m
JOIN execution_providers p ON p.id = m.provider_id
WHERE m.owner_id = $1
	  AND m.name = $2
	  AND m.status = 'enabled'
	  AND p.owner_id = $1
	  AND p.status = 'enabled'
	`
	args := []any{ownerID, modelName}
	query += " ORDER BY p.priority ASC, p.created_at ASC, m.created_at ASC LIMIT 1"

	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&target.Provider.ID,
		&target.Provider.OwnerID,
		&target.Provider.Name,
		&target.Provider.Protocol,
		&target.Provider.BaseURL,
		&target.Provider.APIKey,
		&target.Provider.Priority,
		&pState,
		&target.Provider.CreatedAt,
		&target.Provider.UpdatedAt,
		&target.Model.ID,
		&target.Model.ProviderID,
		&target.Model.OwnerID,
		&target.Model.Name,
		&target.Model.UpstreamModel,
		&mState,
		&target.Model.CreatedAt,
		&target.Model.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return execapp.ExecutionTarget{}, domain.ErrModelNotFound
		}
		return execapp.ExecutionTarget{}, err
	}
	target.Provider.Status = domain.ProviderStatus(pState)
	target.Model.Status = domain.ModelStatus(mState)
	return target, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
