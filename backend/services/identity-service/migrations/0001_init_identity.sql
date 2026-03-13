-- M2 identity-service initial schema
CREATE TABLE IF NOT EXISTS users (
    id              VARCHAR(64) PRIMARY KEY,
    username        VARCHAR(64) NOT NULL UNIQUE,
    display_name    VARCHAR(128) NOT NULL,
    role            VARCHAR(32) NOT NULL,
    parent_id       VARCHAR(64),
    password_hash   TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      VARCHAR(64),
    updated_by      VARCHAR(64)
);

CREATE INDEX IF NOT EXISTS idx_users_parent_id ON users(parent_id);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_single_superuser ON users ((role)) WHERE role = 'superuser';
