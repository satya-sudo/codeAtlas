#!/bin/sh

set -eu

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <service-name> <command> [args...]" >&2
  exit 1
fi

SERVICE_NAME="$1"
shift

ROOT_DIR="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
LOG_DIR="${ROOT_DIR}/logs"
mkdir -p "${LOG_DIR}"

TIMESTAMP="$(date +"%Y%m%d-%H%M%S")"
LOG_FILE="${LOG_DIR}/${SERVICE_NAME}-${TIMESTAMP}.log"

echo "Writing ${SERVICE_NAME} logs to ${LOG_FILE}"

"$@" 2>&1 | tee "${LOG_FILE}"
