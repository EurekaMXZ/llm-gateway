package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"llm-gateway/backend/services/routing-service/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) EnsureSchema(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS routing_policies (
    id                  VARCHAR(64) PRIMARY KEY,
    owner_id            VARCHAR(64) NOT NULL,
    custom_model        VARCHAR(128) NOT NULL,
    target_provider_id  VARCHAR(64) NOT NULL,
    target_model        VARCHAR(128) NOT NULL,
    priority            INTEGER NOT NULL,
    condition_json      TEXT NOT NULL DEFAULT '',
    status              VARCHAR(32) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_id, custom_model, priority)
);
CREATE INDEX IF NOT EXISTS idx_routing_policies_owner ON routing_policies(owner_id);
CREATE INDEX IF NOT EXISTS idx_routing_policies_custom_model ON routing_policies(custom_model);
CREATE INDEX IF NOT EXISTS idx_routing_policies_status ON routing_policies(status);
`)
	return err
}

func (r *Repository) CreatePolicy(ctx context.Context, policy domain.Policy) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO routing_policies (id, owner_id, custom_model, target_provider_id, target_model, priority, condition_json, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`,
		policy.ID,
		policy.OwnerID,
		policy.CustomModel,
		policy.TargetProviderID,
		policy.TargetModel,
		policy.Priority,
		policy.ConditionJSON,
		string(policy.Status),
		policy.CreatedAt.UTC(),
		policy.UpdatedAt.UTC(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrPolicyNameTaken
		}
		return err
	}
	return nil
}

func (r *Repository) GetPolicyByID(ctx context.Context, id string) (domain.Policy, error) {
	var (
		policy domain.Policy
		status string
	)
	err := r.pool.QueryRow(ctx, `
SELECT id, owner_id, custom_model, target_provider_id, target_model, priority, condition_json, status, created_at, updated_at
FROM routing_policies
WHERE id = $1
`, strings.TrimSpace(id)).Scan(
		&policy.ID,
		&policy.OwnerID,
		&policy.CustomModel,
		&policy.TargetProviderID,
		&policy.TargetModel,
		&policy.Priority,
		&policy.ConditionJSON,
		&status,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Policy{}, domain.ErrPolicyNotFound
		}
		return domain.Policy{}, err
	}
	policy.Status = domain.PolicyStatus(status)
	return policy, nil
}

func (r *Repository) ListPolicies(ctx context.Context, ownerID string, customModel string) ([]domain.Policy, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id, owner_id, custom_model, target_provider_id, target_model, priority, condition_json, status, created_at, updated_at
FROM routing_policies
WHERE ($1 = '' OR owner_id = $1)
  AND ($2 = '' OR custom_model = $2)
ORDER BY priority ASC, created_at ASC
`, strings.TrimSpace(ownerID), strings.TrimSpace(customModel))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.Policy, 0)
	for rows.Next() {
		var (
			policy domain.Policy
			status string
		)
		if err := rows.Scan(
			&policy.ID,
			&policy.OwnerID,
			&policy.CustomModel,
			&policy.TargetProviderID,
			&policy.TargetModel,
			&policy.Priority,
			&policy.ConditionJSON,
			&status,
			&policy.CreatedAt,
			&policy.UpdatedAt,
		); err != nil {
			return nil, err
		}
		policy.Status = domain.PolicyStatus(status)
		list = append(list, policy)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *Repository) UpdatePolicyStatus(ctx context.Context, id string, status domain.PolicyStatus, updatedAt time.Time) (domain.Policy, error) {
	var (
		policy domain.Policy
		state  string
	)
	err := r.pool.QueryRow(ctx, `
UPDATE routing_policies
SET status = $2, updated_at = $3
WHERE id = $1
RETURNING id, owner_id, custom_model, target_provider_id, target_model, priority, condition_json, status, created_at, updated_at
`, strings.TrimSpace(id), string(status), updatedAt.UTC()).Scan(
		&policy.ID,
		&policy.OwnerID,
		&policy.CustomModel,
		&policy.TargetProviderID,
		&policy.TargetModel,
		&policy.Priority,
		&policy.ConditionJSON,
		&state,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Policy{}, domain.ErrPolicyNotFound
		}
		return domain.Policy{}, err
	}
	policy.Status = domain.PolicyStatus(state)
	return policy, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
