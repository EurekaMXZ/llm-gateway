-- M2 prompt-service template schema
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
