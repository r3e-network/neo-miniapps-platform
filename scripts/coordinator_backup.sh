#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: ./scripts/coordinator_backup.sh [options]

Creates a point-in-time tar.gz backup of the MarbleRun Coordinator PVC and saves it locally.
Optionally uploads the backup to S3 (requires AWS CLI credentials).

Options:
  --namespace NAME        Kubernetes namespace (default: marblerun)
  --deployment NAME       Coordinator Deployment name (default: coordinator)
  --pvc NAME              PVC name (default: coordinator-pvc)
  --output-dir PATH       Local output directory (default: ./backups)
  --image IMAGE           Helper image used to mount the PVC (default: alpine:3.20)
  --timeout SECONDS       kubectl wait/rollout timeout (default: 300)
  --s3-uri URI            Upload destination (example: s3://bucket/path/)
  --no-scale              Do not scale the coordinator Deployment down/up (not recommended with RWO PVCs)
  -h, --help              Show this help

Environment variables (optional):
  KUBECTL_CONTEXT         kubectl context to use
  AWS_PROFILE             AWS profile for S3 upload

Examples:
  ./scripts/coordinator_backup.sh --output-dir ./backups
  ./scripts/coordinator_backup.sh --s3-uri s3://my-bucket/marblerun/
EOF
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

NAMESPACE="marblerun"
DEPLOYMENT="coordinator"
PVC="coordinator-pvc"
OUTPUT_DIR="${PROJECT_ROOT}/backups"
IMAGE="alpine:3.20"
TIMEOUT_SECONDS="300"
S3_URI=""
NO_SCALE="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace)
      NAMESPACE="${2:-}"; shift 2 ;;
    --deployment)
      DEPLOYMENT="${2:-}"; shift 2 ;;
    --pvc)
      PVC="${2:-}"; shift 2 ;;
    --output-dir)
      OUTPUT_DIR="${2:-}"; shift 2 ;;
    --image)
      IMAGE="${2:-}"; shift 2 ;;
    --timeout)
      TIMEOUT_SECONDS="${2:-}"; shift 2 ;;
    --s3-uri)
      S3_URI="${2:-}"; shift 2 ;;
    --no-scale)
      NO_SCALE="true"; shift ;;
    -h|--help)
      usage; exit 0 ;;
    *)
      die "Unknown argument: $1 (run --help)" ;;
  esac
done

if [[ -z "$NAMESPACE" || -z "$DEPLOYMENT" || -z "$PVC" || -z "$OUTPUT_DIR" ]]; then
  die "Invalid arguments (run --help)"
fi

require_cmd kubectl
require_cmd date

KUBECTL_ARGS=()
if [[ -n "${KUBECTL_CONTEXT:-}" ]]; then
  KUBECTL_ARGS+=(--context "$KUBECTL_CONTEXT")
fi

backup_pod=""
previous_replicas=""
scaled_down="false"

cleanup() {
  set +e
  if [[ -n "$backup_pod" ]]; then
    kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" delete pod "$backup_pod" --ignore-not-found >/dev/null 2>&1 || true
  fi
  if [[ "$scaled_down" == "true" && "$NO_SCALE" != "true" && -n "$previous_replicas" ]]; then
    kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" scale deployment "$DEPLOYMENT" --replicas="$previous_replicas" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

echo "==> Validating Kubernetes resources..."
kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" get pvc "$PVC" >/dev/null
kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" get deployment "$DEPLOYMENT" >/dev/null

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$OUTPUT_DIR"
backup_file="${OUTPUT_DIR%/}/coordinator-pvc-${timestamp}.tar.gz"
checksum_file="${backup_file}.sha256"

if [[ "$NO_SCALE" != "true" ]]; then
  echo "==> Scaling down deployment/${DEPLOYMENT} to avoid RWO multi-attach..."
  previous_replicas="$(kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" get deployment "$DEPLOYMENT" -o jsonpath='{.spec.replicas}')"
  kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" scale deployment "$DEPLOYMENT" --replicas=0
  scaled_down="true"
  kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" wait --for=delete pod -l app=coordinator --timeout="${TIMEOUT_SECONDS}s" >/dev/null 2>&1 || true
fi

backup_pod="coordinator-backup-${timestamp,,}"
backup_pod="${backup_pod//[^a-z0-9-]/-}"
backup_pod="${backup_pod:0:63}"

echo "==> Creating helper pod ${backup_pod}..."
kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${backup_pod}
  labels:
    app: coordinator-backup
spec:
  restartPolicy: Never
  automountServiceAccountToken: false
  containers:
    - name: backup
      image: ${IMAGE}
      command: ["sh", "-c", "sleep 3600"]
      volumeMounts:
        - name: data
          mountPath: /data
          readOnly: true
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: ${PVC}
EOF

kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" wait --for=condition=Ready "pod/${backup_pod}" --timeout="${TIMEOUT_SECONDS}s" >/dev/null

echo "==> Streaming tar archive to ${backup_file}..."
kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" exec "$backup_pod" -- tar -czf - -C /data . >"$backup_file"

echo "==> Writing checksum ${checksum_file}..."
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "$backup_file" >"$checksum_file"
else
  shasum -a 256 "$backup_file" >"$checksum_file"
fi

echo "==> Backup complete:"
echo "  - ${backup_file}"
echo "  - ${checksum_file}"

if [[ -n "$S3_URI" ]]; then
  require_cmd aws
  echo "==> Uploading to ${S3_URI}..."
  aws s3 cp "$backup_file" "${S3_URI%/}/$(basename "$backup_file")"
  aws s3 cp "$checksum_file" "${S3_URI%/}/$(basename "$checksum_file")"
fi

echo "==> Done."
