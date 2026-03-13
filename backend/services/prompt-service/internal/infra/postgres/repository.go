package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"llm-gateway/backend/services/prompt-service/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) EnsureSchema(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS prompt_templates (
    id              VARCHAR(64) PRIMARY KEY,
    owner_id        VARCHAR(64) NOT NULL,
    scene           VARCHAR(128) NOT NULL,
    content         TEXT NOT NULL,
    variables_json  JSONB NOT NULL,
    status          VARCHAR(32) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_id, scene)
);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_owner ON prompt_templates(owner_id);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_scene ON prompt_templates(scene);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_status ON prompt_templates(status);
`)
	return err
}

func (r *Repository) CreateTemplate(ctx context.Context, tpl domain.SceneTemplate) error {
	varsJSON, err := json.Marshal(tpl.Variables)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
INSERT INTO prompt_templates (id, owner_id, scene, content, variables_json, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`, tpl.ID, tpl.OwnerID, tpl.Scene, tpl.Content, varsJSON, string(tpl.Status), tpl.CreatedAt.UTC(), tpl.UpdatedAt.UTC())
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrTemplateNameTaken
		}
		return err
	}
	return nil
}

func (r *Repository) GetTemplateByID(ctx context.Context, id string) (domain.SceneTemplate, error) {
	return r.getTemplate(ctx, `
SELECT id, owner_id, scene, content, variables_json, status, created_at, updated_at
FROM prompt_templates
WHERE id = $1
`, strings.TrimSpace(id))
}

func (r *Repository) GetTemplateByOwnerScene(ctx context.Context, ownerID string, scene string) (domain.SceneTemplate, error) {
	return r.getTemplate(ctx, `
SELECT id, owner_id, scene, content, variables_json, status, created_at, updated_at
FROM prompt_templates
WHERE owner_id = $1 AND scene = $2
`, strings.TrimSpace(ownerID), strings.TrimSpace(scene))
}

func (r *Repository) getTemplate(ctx context.Context, query string, args ...any) (domain.SceneTemplate, error) {
	row := r.pool.QueryRow(ctx, query, args...)
	tpl, err := scanTemplate(row)
	if err != nil {
		return domain.SceneTemplate{}, err
	}
	return tpl, nil
}

func (r *Repository) ListTemplates(ctx context.Context, ownerID string) ([]domain.SceneTemplate, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id, owner_id, scene, content, variables_json, status, created_at, updated_at
FROM prompt_templates
WHERE ($1 = '' OR owner_id = $1)
ORDER BY created_at ASC
`, strings.TrimSpace(ownerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.SceneTemplate, 0)
	for rows.Next() {
		tpl, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, tpl)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *Repository) UpdateTemplateStatus(ctx context.Context, id string, status domain.TemplateStatus, updatedAt time.Time) (domain.SceneTemplate, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE prompt_templates
SET status = $2, updated_at = $3
WHERE id = $1
RETURNING id, owner_id, scene, content, variables_json, status, created_at, updated_at
`, strings.TrimSpace(id), string(status), updatedAt.UTC())
	tpl, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SceneTemplate{}, domain.ErrTemplateNotFound
		}
		return domain.SceneTemplate{}, err
	}
	return tpl, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTemplate(row scanner) (domain.SceneTemplate, error) {
	var (
		tpl      domain.SceneTemplate
		status   string
		varsJSON []byte
	)
	err := row.Scan(&tpl.ID, &tpl.OwnerID, &tpl.Scene, &tpl.Content, &varsJSON, &status, &tpl.CreatedAt, &tpl.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SceneTemplate{}, domain.ErrTemplateNotFound
		}
		return domain.SceneTemplate{}, err
	}
	if err := json.Unmarshal(varsJSON, &tpl.Variables); err != nil {
		return domain.SceneTemplate{}, err
	}
	tpl.Status = domain.TemplateStatus(status)
	return tpl, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
