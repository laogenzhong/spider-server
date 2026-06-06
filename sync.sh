#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NAME="spider-server"
PACKAGE_ENV="online"
TARGET_OS="linux"
TARGET_ARCH="amd64"

OUTPUT_DIR="${ROOT_DIR}/bin"
RELEASE_NAME="${APP_NAME}-${PACKAGE_ENV}-${TARGET_OS}-${TARGET_ARCH}"
TARBALL="${TARBALL:-${OUTPUT_DIR}/${RELEASE_NAME}.tar.gz}"

REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_HOST="${REMOTE_HOST:-110.42.253.62}"
REMOTE_DIR="${REMOTE_DIR:-/root/app/spider/api}"
REMOTE="${REMOTE_USER}@${REMOTE_HOST}"

if [[ ! -f "${TARBALL}" ]]; then
  echo "Tarball not found: ${TARBALL}" >&2
  echo "Run ./build.sh first." >&2
  exit 1
fi

if ! command -v rsync >/dev/null 2>&1; then
  echo "rsync not found. Install rsync before syncing package." >&2
  exit 1
fi

if ! command -v ssh >/dev/null 2>&1; then
  echo "ssh not found. Install ssh before syncing package." >&2
  exit 1
fi

echo "==> Ensure remote dir exists: ${REMOTE}:${REMOTE_DIR}"
ssh "${REMOTE}" "mkdir -p '${REMOTE_DIR}'"

echo "==> Sync package"
rsync -avz --progress "${TARBALL}" "${REMOTE}:${REMOTE_DIR}/"

echo
echo "==> Done"
echo "Uploaded: ${TARBALL}"
echo "Remote:   ${REMOTE}:${REMOTE_DIR}/"
