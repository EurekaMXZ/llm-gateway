-- M2 routing-service policy schema
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
