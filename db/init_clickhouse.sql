-- db/init_clickhouse.sql

CREATE TABLE IF NOT EXISTS service_logs_kafka
(
    timestamp DateTime,
    service   String,
    level     String,
    message   String
) ENGINE = Kafka
SETTINGS kafka_broker_list = 'kafka:9092',
         kafka_topic_list = 'service-logs',
         kafka_group_name = 'clickhouse_logs_group',
         kafka_format = 'JSONEachRow',
         kafka_num_consumers = 1;

CREATE TABLE IF NOT EXISTS service_logs
(
    timestamp DateTime,
    service   String,
    level     String,
    message   String
) ENGINE = MergeTree()
ORDER BY (timestamp, service);

CREATE MATERIALIZED VIEW IF NOT EXISTS service_logs_mv
    TO service_logs
AS
SELECT timestamp, service, level, message
FROM service_logs_kafka;
