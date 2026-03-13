package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"llm-gateway/backend/services/billing-service/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) EnsureSchema(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS billing_prices (
    id                  VARCHAR(64) PRIMARY KEY,
    owner_id            VARCHAR(64) NOT NULL,
    provider_id         VARCHAR(64) NOT NULL,
    model               VARCHAR(128) NOT NULL,
    input_price_per_1k  NUMERIC(18,6) NOT NULL,
    output_price_per_1k NUMERIC(18,6) NOT NULL,
    currency            VARCHAR(16) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_id, provider_id, model, currency)
);

CREATE TABLE IF NOT EXISTS billing_wallets (
    owner_id        VARCHAR(64) PRIMARY KEY,
    balance_cents   BIGINT NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS billing_wallet_transactions (
    id                    VARCHAR(64) PRIMARY KEY,
    owner_id              VARCHAR(64) NOT NULL,
    tx_type               VARCHAR(32) NOT NULL,
    amount_cents          BIGINT NOT NULL,
    balance_after_cents   BIGINT NOT NULL,
    reason                TEXT NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_billing_prices_owner ON billing_prices(owner_id);
CREATE INDEX IF NOT EXISTS idx_billing_wallet_tx_owner ON billing_wallet_transactions(owner_id);
CREATE INDEX IF NOT EXISTS idx_billing_wallet_tx_created_at ON billing_wallet_transactions(created_at DESC);
`)
	return err
}

func (r *Repository) UpsertPrice(ctx context.Context, price domain.Price) (domain.Price, error) {
	var out domain.Price
	err := r.pool.QueryRow(ctx, `
INSERT INTO billing_prices (id, owner_id, provider_id, model, input_price_per_1k, output_price_per_1k, currency, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (owner_id, provider_id, model, currency)
DO UPDATE SET
  input_price_per_1k = EXCLUDED.input_price_per_1k,
  output_price_per_1k = EXCLUDED.output_price_per_1k,
  updated_at = EXCLUDED.updated_at
RETURNING id, owner_id, provider_id, model, input_price_per_1k, output_price_per_1k, currency, created_at, updated_at
`, price.ID, price.OwnerID, price.ProviderID, price.Model, price.InputPricePer1K, price.OutputPricePer1K, price.Currency, price.CreatedAt.UTC(), price.UpdatedAt.UTC()).Scan(
		&out.ID,
		&out.OwnerID,
		&out.ProviderID,
		&out.Model,
		&out.InputPricePer1K,
		&out.OutputPricePer1K,
		&out.Currency,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return domain.Price{}, err
	}
	return out, nil
}

func (r *Repository) GetPriceByID(ctx context.Context, id string) (domain.Price, error) {
	var out domain.Price
	err := r.pool.QueryRow(ctx, `
SELECT id, owner_id, provider_id, model, input_price_per_1k, output_price_per_1k, currency, created_at, updated_at
FROM billing_prices
WHERE id = $1
`, strings.TrimSpace(id)).Scan(
		&out.ID,
		&out.OwnerID,
		&out.ProviderID,
		&out.Model,
		&out.InputPricePer1K,
		&out.OutputPricePer1K,
		&out.Currency,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Price{}, domain.ErrPriceNotFound
		}
		return domain.Price{}, err
	}
	return out, nil
}

func (r *Repository) ListPrices(ctx context.Context, ownerID string) ([]domain.Price, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id, owner_id, provider_id, model, input_price_per_1k, output_price_per_1k, currency, created_at, updated_at
FROM billing_prices
WHERE ($1 = '' OR owner_id = $1)
ORDER BY updated_at DESC
`, strings.TrimSpace(ownerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.Price, 0)
	for rows.Next() {
		var out domain.Price
		if err := rows.Scan(
			&out.ID,
			&out.OwnerID,
			&out.ProviderID,
			&out.Model,
			&out.InputPricePer1K,
			&out.OutputPricePer1K,
			&out.Currency,
			&out.CreatedAt,
			&out.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, out)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *Repository) GetWallet(ctx context.Context, ownerID string) (domain.Wallet, error) {
	var wallet domain.Wallet
	err := r.pool.QueryRow(ctx, `
SELECT owner_id, balance_cents, updated_at
FROM billing_wallets
WHERE owner_id = $1
`, strings.TrimSpace(ownerID)).Scan(
		&wallet.OwnerID,
		&wallet.BalanceCents,
		&wallet.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Wallet{}, domain.ErrWalletNotFound
		}
		return domain.Wallet{}, err
	}
	return wallet, nil
}

func (r *Repository) ApplyWalletDelta(ctx context.Context, ownerID string, deltaCents int64, txID string, txType domain.TransactionType, reason string, now time.Time) (domain.Wallet, domain.WalletTransaction, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Wallet{}, domain.WalletTransaction{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ownerID = strings.TrimSpace(ownerID)
	if _, err := tx.Exec(ctx, `
INSERT INTO billing_wallets (owner_id, balance_cents, updated_at)
VALUES ($1, 0, $2)
ON CONFLICT (owner_id) DO NOTHING
`, ownerID, now.UTC()); err != nil {
		return domain.Wallet{}, domain.WalletTransaction{}, err
	}

	var current int64
	if err := tx.QueryRow(ctx, `
SELECT balance_cents
FROM billing_wallets
WHERE owner_id = $1
FOR UPDATE
`, ownerID).Scan(&current); err != nil {
		return domain.Wallet{}, domain.WalletTransaction{}, err
	}

	next := current + deltaCents
	if next < 0 {
		return domain.Wallet{}, domain.WalletTransaction{}, domain.ErrInsufficientBalance
	}

	if _, err := tx.Exec(ctx, `
UPDATE billing_wallets
SET balance_cents = $2, updated_at = $3
WHERE owner_id = $1
`, ownerID, next, now.UTC()); err != nil {
		return domain.Wallet{}, domain.WalletTransaction{}, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO billing_wallet_transactions (id, owner_id, tx_type, amount_cents, balance_after_cents, reason, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`, txID, ownerID, string(txType), deltaCents, next, reason, now.UTC()); err != nil {
		return domain.Wallet{}, domain.WalletTransaction{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Wallet{}, domain.WalletTransaction{}, err
	}

	wallet := domain.Wallet{OwnerID: ownerID, BalanceCents: next, UpdatedAt: now.UTC()}
	txn := domain.WalletTransaction{
		ID:                txID,
		OwnerID:           ownerID,
		Type:              txType,
		AmountCents:       deltaCents,
		BalanceAfterCents: next,
		Reason:            reason,
		CreatedAt:         now.UTC(),
	}
	return wallet, txn, nil
}

func (r *Repository) ListTransactions(ctx context.Context, ownerID string, limit int) ([]domain.WalletTransaction, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id, owner_id, tx_type, amount_cents, balance_after_cents, reason, created_at
FROM billing_wallet_transactions
WHERE owner_id = $1
ORDER BY created_at DESC
LIMIT $2
`, strings.TrimSpace(ownerID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.WalletTransaction, 0)
	for rows.Next() {
		var (
			txType string
			txn    domain.WalletTransaction
		)
		if err := rows.Scan(&txn.ID, &txn.OwnerID, &txType, &txn.AmountCents, &txn.BalanceAfterCents, &txn.Reason, &txn.CreatedAt); err != nil {
			return nil, err
		}
		txn.Type = domain.TransactionType(txType)
		list = append(list, txn)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
