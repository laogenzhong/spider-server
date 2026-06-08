#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RELEASE_NAME="spider-server-online-linux-amd64"
TARBALL="${SCRIPT_DIR}/${RELEASE_NAME}.tar.gz"
RELEASE_DIR="${SCRIPT_DIR}/${RELEASE_NAME}"
PID_FILE="${RELEASE_DIR}/spider-server.pid"

echo "==> deploy start"
echo "script dir: ${SCRIPT_DIR}"
echo "tarball:    ${TARBALL}"
echo "release:    ${RELEASE_DIR}"

if [[ ! -f "${TARBALL}" ]]; then
  echo "ERROR: tarball not found: ${TARBALL}" >&2
  exit 1
fi

OLD_PID=""
if [[ -f "${PID_FILE}" ]]; then
  OLD_PID="$(tr -d '[:space:]' < "${PID_FILE}" || true)"
fi

if [[ -d "${RELEASE_DIR}" ]]; then
  echo "==> stop existing extracted server"

  if [[ -x "${RELEASE_DIR}/stop.sh" ]]; then
    if ! "${RELEASE_DIR}/stop.sh"; then
      echo "ERROR: stop failed: ${RELEASE_DIR}/stop.sh returned non-zero" >&2
      exit 1
    fi
  elif [[ -n "${OLD_PID}" ]] && kill -0 "${OLD_PID}" >/dev/null 2>&1; then
    echo "stop.sh not found, fallback stop pid=${OLD_PID}"
    if ! kill "${OLD_PID}" >/dev/null 2>&1; then
      echo "ERROR: stop failed: kill ${OLD_PID} returned non-zero" >&2
      exit 1
    fi
    for _ in {1..20}; do
      if ! kill -0 "${OLD_PID}" >/dev/null 2>&1; then
        break
      fi
      sleep 0.25
    done
    if kill -0 "${OLD_PID}" >/dev/null 2>&1; then
      echo "ERROR: stop failed: process still alive pid=${OLD_PID}" >&2
      exit 1
    fi
    rm -f "${PID_FILE}"
  else
    echo "existing release dir found, but no running pid detected"
  fi

  if [[ -n "${OLD_PID}" ]] && kill -0 "${OLD_PID}" >/dev/null 2>&1; then
    echo "ERROR: stop failed: process still alive pid=${OLD_PID}" >&2
    exit 1
  fi
else
  echo "==> no existing extracted server, skip stop"
fi

echo "==> extract package"
tar -xzf "${TARBALL}" -C "${SCRIPT_DIR}"

if [[ ! -x "${RELEASE_DIR}/run.sh" ]]; then
  chmod +x "${RELEASE_DIR}/run.sh"
fi
if [[ -f "${RELEASE_DIR}/stop.sh" && ! -x "${RELEASE_DIR}/stop.sh" ]]; then
  chmod +x "${RELEASE_DIR}/stop.sh"
fi
if [[ -f "${RELEASE_DIR}/spider-server" && ! -x "${RELEASE_DIR}/spider-server" ]]; then
  chmod +x "${RELEASE_DIR}/spider-server"
fi

echo "==> run server"
"${RELEASE_DIR}/run.sh"

echo "==> deploy done"
