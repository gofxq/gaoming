CREATE TABLE IF NOT EXISTS auth_bootstrap_state (
    id smallint PRIMARY KEY CHECK (id = 1),
    completed_at timestamptz NOT NULL
);

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'users'::regclass AND conname = 'uq_users_tenant_id_id') THEN
        ALTER TABLE users ADD CONSTRAINT uq_users_tenant_id_id UNIQUE (tenant_id, id);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS user_passwords (
    user_id bigint PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    tenant_id bigint NOT NULL REFERENCES tenants(id),
    username varchar(64) NOT NULL,
    password_hash varchar(128) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, username),
    FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id) ON DELETE CASCADE
);
