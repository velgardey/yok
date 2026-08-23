-- ClickHouse schema for deployment build logs.
-- Apply once per CLICKHOUSE_DATABASE before first use, e.g.:
--   clickhouse-client --host <host> --database <db> < api/clickhouse/schema.sql

CREATE TABLE IF NOT EXISTS log_events
(
    event_id      UUID,
    deployment_id String,
    log           String,
    timestamp     DateTime DEFAULT now()
)
ENGINE = MergeTree
ORDER BY (deployment_id, timestamp, event_id);
