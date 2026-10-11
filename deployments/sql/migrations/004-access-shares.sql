DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='hosts'::regclass AND conname='uq_hosts_tenant_uid') THEN
        ALTER TABLE hosts ADD CONSTRAINT uq_hosts_tenant_uid UNIQUE (tenant_id, host_uid);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS access_shares (
    id varchar(64) PRIMARY KEY,
    tenant_id bigint NOT NULL REFERENCES tenants(id),
    title varchar(128) NOT NULL,
    token_hash varchar(64) NOT NULL UNIQUE,
    fields jsonb NOT NULL,
    expires_at timestamptz NOT NULL,
    created_by bigint NOT NULL,
    created_at timestamptz NOT NULL,
    revoked_at timestamptz,
    UNIQUE (tenant_id,id),
    FOREIGN KEY (tenant_id,created_by) REFERENCES users(tenant_id,id)
);
CREATE INDEX IF NOT EXISTS idx_access_shares_tenant ON access_shares(tenant_id,created_at DESC);
CREATE TABLE IF NOT EXISTS access_share_hosts (
    share_id varchar(64) NOT NULL,
    tenant_id bigint NOT NULL,
    host_uid varchar(64) NOT NULL,
    PRIMARY KEY (share_id,host_uid),
    FOREIGN KEY (tenant_id,share_id) REFERENCES access_shares(tenant_id,id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id,host_uid) REFERENCES hosts(tenant_id,host_uid) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS access_share_sessions (
    token_hash varchar(64) PRIMARY KEY,
    share_id varchar(64) NOT NULL REFERENCES access_shares(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_access_share_sessions_share ON access_share_sessions(share_id,expires_at);
