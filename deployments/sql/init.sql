create table tenants (
    id                  bigserial primary key,
    tenant_code         varchar(64) not null unique,
    name                varchar(128) not null,
    status              smallint not null default 1,
    created_at          timestamptz not null default now(),
    updated_at          timestamptz not null default now()
);

create table host_groups (
    id                  bigserial primary key,
    tenant_id           bigint not null references tenants(id),
    parent_id           bigint null references host_groups(id),
    group_code          varchar(64) not null,
    name                varchar(128) not null,
    path                varchar(512) not null,
    created_at          timestamptz not null default now(),
    updated_at          timestamptz not null default now(),
    unique (tenant_id, group_code)
);

create index idx_host_groups_tenant_parent on host_groups(tenant_id, parent_id);

create table hosts (
    id                  bigserial primary key,
    tenant_id           bigint not null references tenants(id),
    host_uid            varchar(64) not null unique,
    hostname            varchar(128) not null,
    primary_ip          inet not null,
    os_type             varchar(32) not null,
    arch                varchar(32) not null,
    region              varchar(32) not null,
    az                  varchar(32),
    env                 varchar(32),
    role                varchar(64),
    asset_id            varchar(64),
    status              smallint not null default 0,
    registered_at       timestamptz not null default now(),
    last_register_at    timestamptz,
    created_at          timestamptz not null default now(),
    updated_at          timestamptz not null default now()
);

create index idx_hosts_tenant_region on hosts(tenant_id, region);
create index idx_hosts_hostname on hosts(tenant_id, hostname);
create index idx_hosts_primary_ip on hosts(primary_ip);

create table host_group_rel (
    tenant_id           bigint not null references tenants(id),
    group_id            bigint not null references host_groups(id),
    host_id             bigint not null references hosts(id),
    primary_group       boolean not null default false,
    created_at          timestamptz not null default now(),
    primary key (group_id, host_id)
);

create index idx_host_group_rel_host on host_group_rel(host_id);

create table host_labels (
    id                  bigserial primary key,
    host_id             bigint not null references hosts(id) on delete cascade,
    label_key           varchar(64) not null,
    label_value         varchar(256) not null,
    created_at          timestamptz not null default now(),
    unique (host_id, label_key)
);

create index idx_host_labels_kv on host_labels(label_key, label_value);

create table host_inventory (
    id                  bigserial primary key,
    host_id             bigint not null unique references hosts(id) on delete cascade,
    cpu_model           varchar(256),
    cpu_cores           integer,
    mem_total_mb        integer,
    disk_total_gb       integer,
    kernel_version      varchar(128),
    virtualization      varchar(32),
    cloud_provider      varchar(32),
    instance_type       varchar(64),
    bios_serial         varchar(128),
    extra               jsonb not null default '{}'::jsonb,
    updated_at          timestamptz not null default now()
);

create table agent_instances (
    id                      bigserial primary key,
    host_id                 bigint not null references hosts(id) on delete cascade,
    agent_id                varchar(64) not null,
    version                 varchar(32) not null,
    state                   smallint not null default 0,
    config_version          bigint not null default 0,
    heartbeat_interval_sec  integer not null default 5,
    metric_interval_sec     integer not null default 5,
    last_seen_at            timestamptz,
    last_seq                bigint not null default 0,
    last_ip                 inet,
    cert_sn                 varchar(128),
    capabilities            jsonb not null default '[]'::jsonb,
    created_at              timestamptz not null default now(),
    updated_at              timestamptz not null default now(),
    unique (host_id, agent_id)
);

create index idx_agent_instances_seen on agent_instances(last_seen_at);
create index idx_agent_instances_state on agent_instances(state);

create table host_status_current (
    host_id                 bigint primary key references hosts(id) on delete cascade,
    agent_state             smallint not null default 0,
    reachability_state      smallint not null default 0,
    service_state           smallint not null default 0,
    overall_state           smallint not null default 0,
    severity                smallint not null default 0,
    cpu_usage_pct           numeric(6,2),
    mem_used_pct            numeric(6,2),
    disk_used_pct           numeric(6,2),
    load1                   numeric(8,2),
    net_rx_bps              bigint,
    net_tx_bps              bigint,
    last_agent_seen_at      timestamptz,
    last_metric_at          timestamptz,
    last_probe_at           timestamptz,
    open_alert_count        integer not null default 0,
    maintenance_until       timestamptz,
    version                 bigint not null default 0,
    updated_at              timestamptz not null default now()
);

create index idx_host_status_current_state on host_status_current(overall_state, updated_at desc);
create index idx_host_status_current_agent on host_status_current(agent_state, last_agent_seen_at desc);

create table probe_policies (
    id                  bigserial primary key,
    tenant_id           bigint not null references tenants(id),
    name                varchar(128) not null,
    probe_type          varchar(32) not null,
    interval_sec        integer not null,
    timeout_ms          integer not null,
    retries             integer not null default 1,
    success_rule        varchar(32) not null default '1of1',
    enabled             boolean not null default true,
    region_scope        jsonb not null default '[]'::jsonb,
    spec                jsonb not null default '{}'::jsonb,
    created_at          timestamptz not null default now(),
    updated_at          timestamptz not null default now()
);

create index idx_probe_policies_tenant_enabled on probe_policies(tenant_id, enabled);

create table probe_targets (
    id                  bigserial primary key,
    tenant_id           bigint not null references tenants(id),
    target_type         varchar(32) not null,
    host_id             bigint null references hosts(id) on delete cascade,
    service_name        varchar(128),
    target_value        varchar(512) not null,
    enabled             boolean not null default true,
    labels              jsonb not null default '{}'::jsonb,
    created_at          timestamptz not null default now(),
    updated_at          timestamptz not null default now()
);

create index idx_probe_targets_host on probe_targets(host_id);
create index idx_probe_targets_enabled on probe_targets(enabled);

create table probe_jobs (
    id                  bigserial primary key,
    policy_id           bigint not null references probe_policies(id) on delete cascade,
    target_id           bigint not null references probe_targets(id) on delete cascade,
    region_code         varchar(32) not null,
    status              smallint not null default 1,
    next_run_at         timestamptz not null,
    lease_owner         varchar(64),
    lease_expire_at     timestamptz,
    last_result_at      timestamptz,
    created_at          timestamptz not null default now(),
    updated_at          timestamptz not null default now(),
    unique (policy_id, target_id, region_code)
);

create index idx_probe_jobs_schedule on probe_jobs(status, region_code, next_run_at);
create index idx_probe_jobs_lease on probe_jobs(lease_expire_at);

create table probe_results (
    id                  bigserial primary key,
    job_id              bigint not null references probe_jobs(id) on delete cascade,
    target_id           bigint not null references probe_targets(id) on delete cascade,
    worker_id           varchar(64) not null,
    region_code         varchar(32) not null,
    ts                  timestamptz not null,
    success             boolean not null,
    latency_ms          integer,
    status_code         integer,
    error_code          varchar(64),
    error_msg           text,
    observation         jsonb not null default '{}'::jsonb
);

create index idx_probe_results_target_ts on probe_results(target_id, ts desc);
create index idx_probe_results_job_ts on probe_results(job_id, ts desc);

create table alert_rules (
    id                  bigserial primary key,
    tenant_id           bigint not null references tenants(id),
    name                varchar(128) not null,
    scope_type          varchar(32) not null,
    scope_ref           varchar(128),
    expr                text not null,
    severity            smallint not null,
    for_seconds         integer not null default 0,
    enabled             boolean not null default true,
    labels              jsonb not null default '{}'::jsonb,
    channels            jsonb not null default '[]'::jsonb,
    created_at          timestamptz not null default now(),
    updated_at          timestamptz not null default now()
);

create index idx_alert_rules_scope on alert_rules(tenant_id, scope_type, enabled);

create table alert_events (
    id                  bigserial primary key,
    tenant_id           bigint not null references tenants(id),
    rule_id             bigint not null references alert_rules(id),
    host_id             bigint null references hosts(id) on delete set null,
    target_id           bigint null references probe_targets(id) on delete set null,
    fingerprint         varchar(128) not null,
    status              smallint not null,
    severity            smallint not null,
    summary             varchar(256) not null,
    detail              text,
    first_fired_at      timestamptz not null,
    last_fired_at       timestamptz not null,
    resolved_at         timestamptz,
    acked_by            varchar(64),
    acked_at            timestamptz,
    labels              jsonb not null default '{}'::jsonb
);

create index idx_alert_events_status on alert_events(status, severity, last_fired_at desc);
create index idx_alert_events_host on alert_events(host_id, last_fired_at desc);
create index idx_alert_events_fp on alert_events(fingerprint);

create table maintenance_windows (
    id                  bigserial primary key,
    tenant_id           bigint not null references tenants(id),
    title               varchar(128) not null,
    scope_type          varchar(32) not null,
    scope_ref           varchar(128) not null,
    start_at            timestamptz not null,
    end_at              timestamptz not null,
    created_by          varchar(64) not null,
    reason              text,
    enabled             boolean not null default true,
    created_at          timestamptz not null default now()
);

create index idx_maintenance_scope on maintenance_windows(scope_type, scope_ref, start_at, end_at);

create table remote_tasks (
    id                  bigserial primary key,
    tenant_id           bigint not null references tenants(id),
    host_id             bigint not null references hosts(id) on delete cascade,
    task_type           varchar(32) not null,
    payload             jsonb not null default '{}'::jsonb,
    timeout_sec         integer not null default 30,
    status              smallint not null default 0,
    created_by          varchar(64) not null,
    started_at          timestamptz,
    finished_at         timestamptz,
    result_excerpt      text,
    error_msg           text,
    created_at          timestamptz not null default now()
);

create index idx_remote_tasks_host on remote_tasks(host_id, created_at desc);

create table audit_logs (
    id                  bigserial primary key,
    tenant_id           bigint not null references tenants(id),
    actor               varchar(64) not null,
    action              varchar(64) not null,
    resource_type       varchar(32) not null,
    resource_id         varchar(128) not null,
    request_id          varchar(64),
    success             boolean not null,
    detail              jsonb not null default '{}'::jsonb,
    created_at          timestamptz not null default now()
);

create index idx_audit_logs_actor on audit_logs(actor, created_at desc);
create index idx_audit_logs_resource on audit_logs(resource_type, resource_id, created_at desc);
