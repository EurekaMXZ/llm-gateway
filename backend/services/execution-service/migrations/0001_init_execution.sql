-- M2 execution-service provider/model catalog schema
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
