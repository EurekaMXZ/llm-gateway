-- M2 apikey-service initial schema
CREATE TABLE IF NOT EXISTS api_keys (
    id              VARCHAR(64) PRIMARY KEY,
    owner_id        VARCHAR(64) NOT NULL,
    name            VARCHAR(128) NOT NULL,
    secret_hash     TEXT NOT NULL UNIQUE,
    status          VARCHAR(32) NOT NULL,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      VARCHAR(64),
    updated_by      VARCHAR(64)
);

CREATE TABLE IF NOT EXISTS api_key_model_whitelist (
    api_key_id      VARCHAR(64) NOT NULL,
    model_name      VARCHAR(128) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (api_key_id, model_name)
);

CREATE INDEX IF NOT EXISTS idx_api_keys_owner_id ON api_keys(owner_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);
