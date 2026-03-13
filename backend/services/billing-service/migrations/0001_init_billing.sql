-- M2 billing-service pricing and wallet schema
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
