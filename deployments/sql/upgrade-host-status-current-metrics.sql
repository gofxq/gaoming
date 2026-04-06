ALTER TABLE host_status_current
    ADD COLUMN IF NOT EXISTS mem_available_bytes bigint,
    ADD COLUMN IF NOT EXISTS swap_used_pct numeric(6,2),
    ADD COLUMN IF NOT EXISTS disk_free_bytes bigint,
    ADD COLUMN IF NOT EXISTS disk_inodes_used_pct numeric(6,2),
    ADD COLUMN IF NOT EXISTS disk_read_iops bigint,
    ADD COLUMN IF NOT EXISTS disk_write_iops bigint,
    ADD COLUMN IF NOT EXISTS net_rx_packets_ps bigint,
    ADD COLUMN IF NOT EXISTS net_tx_packets_ps bigint;
