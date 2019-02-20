#!/bin/bash
set -e
LIFECYCLE_PORT="${LIFECYCLE_PORT:-9000}"
TLS_PORT="${TLS_PORT:-9443}"
CONFIG_DIR="${CONFIG_DIR:-/conf}"
TLS_CERT_FILE="${TLS_CERT_FILE:-/var/lib/secrets/cert.crt}"
TLS_KEY_FILE="${TLS_KEY_FILE:-/var/lib/secrets/cert.key}"
CONFIGMAP_LABELS="${CONFIGMAP_LABELS:-app=k8s-sidecar-injector}"
CONFIGMAP_NAMESPACE="${CONFIGMAP_NAMESPACE:-}"
LOG_LEVEL="${LOG_LEVEL:-2}"
echo "k8s-sidecar-injector starting at $(date) with TLS_PORT=${TLS_PORT} CONFIG_DIR=${CONFIG_DIR} TLS_CERT_FILE=${TLS_CERT_FILE} TLS_KEY_FILE=${TLS_KEY_FILE}"
set -x
exec k8s-sidecar-injector \
  --v="${LOG_LEVEL}" \
  --lifecycle-port="${LIFECYCLE_PORT}" \
  --tls-port="${TLS_PORT}" \
  --config-directory="${CONFIG_DIR}" \
  --tls-cert-file="${TLS_CERT_FILE}" \
  --tls-key-file="${TLS_KEY_FILE}" \
  --configmap-labels="${CONFIGMAP_LABELS}" \
  --configmap-namespace="${CONFIGMAP_NAMESPACE}" \
  "$@"
