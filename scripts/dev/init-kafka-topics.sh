#!/bin/sh

set -eu

BOOTSTRAP_SERVER="${KAFKA_BOOTSTRAP_SERVER:-kafka:9092}"

echo "Waiting for Kafka at ${BOOTSTRAP_SERVER}..."

until kafka-topics --bootstrap-server "${BOOTSTRAP_SERVER}" --list >/dev/null 2>&1; do
  sleep 2
done

echo "Kafka is ready. Ensuring required topics exist..."

kafka-topics \
  --bootstrap-server "${BOOTSTRAP_SERVER}" \
  --create \
  --if-not-exists \
  --topic repository.sync.requested \
  --partitions 1 \
  --replication-factor 1

echo "Current topics:"
kafka-topics --bootstrap-server "${BOOTSTRAP_SERVER}" --list
