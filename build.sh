#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NAME="spider-server"
ADMIN_CLI_NAME="admin-vip-cli"
PACKAGE_ENV="online"
TARGET_OS="linux"
TARGET_ARCH="amd64"

OUTPUT_DIR="${ROOT_DIR}/bin"
RELEASE_NAME="${APP_NAME}-${PACKAGE_ENV}-${TARGET_OS}-${TARGET_ARCH}"
RELEASE_DIR="${OUTPUT_DIR}/${RELEASE_NAME}"
TARBALL="${OUTPUT_DIR}/${RELEASE_NAME}.tar.gz"
NPM_CACHE_DIR="${OUTPUT_DIR}/.npm-cache"

echo "==> Build ${APP_NAME} for ${TARGET_OS}/${TARGET_ARCH} (${PACKAGE_ENV})"

if [[ ! -f "${ROOT_DIR}/config.server.example.yaml" ]]; then
  echo "config.server.example.yaml not found. Online package requires server config." >&2
  exit 1
fi

required_public_pages=(
  "${ROOT_DIR}/public/index.html"
  "${ROOT_DIR}/public/support.html"
  "${ROOT_DIR}/public/privacy.html"
  "${ROOT_DIR}/public/terms/index.html"
)

for page in "${required_public_pages[@]}"; do
  if [[ ! -f "${page}" ]]; then
    echo "${page} not found. Online package requires public pages." >&2
    exit 1
  fi
done

if command -v npm >/dev/null 2>&1; then
  :
else
  echo "npm not found. Online package requires npm to vendor Apple IAP verifier dependencies." >&2
  exit 1
fi

rm -rf "${RELEASE_DIR}" "${TARBALL}"
mkdir -p "${RELEASE_DIR}/apple_iap_verifier" "${RELEASE_DIR}/logs" "${RELEASE_DIR}/public"

cd "${ROOT_DIR}"

CGO_ENABLED=0 GOOS="${TARGET_OS}" GOARCH="${TARGET_ARCH}" \
  go build -trimpath -ldflags "-s -w" -o "${RELEASE_DIR}/${APP_NAME}" ./cmd

CGO_ENABLED=0 GOOS="${TARGET_OS}" GOARCH="${TARGET_ARCH}" \
  go build -trimpath -ldflags "-s -w" -o "${RELEASE_DIR}/${ADMIN_CLI_NAME}.bin" ./cmd/admin_vip_cli

cat > "${RELEASE_DIR}/${ADMIN_CLI_NAME}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if [[ ! -f config.server.yaml ]]; then
  echo "config.server.yaml not found. Release package is incomplete." >&2
  exit 1
fi

exec ./admin-vip-cli.bin -config config.server.yaml "$@"
EOF
echo "==> Included admin-vip-cli with default config.server.yaml"

cp config.server.example.yaml "${RELEASE_DIR}/config.server.example.yaml"
cp config.server.example.yaml "${RELEASE_DIR}/config.server.yaml"
echo "==> Included config.server.example.yaml as config.server.yaml in release package"

cp -R public/. "${RELEASE_DIR}/public/"
echo "==> Included public HTML pages"

cp apple_iap_verifier/verify_transaction.mjs "${RELEASE_DIR}/apple_iap_verifier/"
cp apple_iap_verifier/app_store_api.mjs "${RELEASE_DIR}/apple_iap_verifier/"
cp apple_iap_verifier/package.json "${RELEASE_DIR}/apple_iap_verifier/"
cp apple_iap_verifier/package-lock.json "${RELEASE_DIR}/apple_iap_verifier/"
cp apple_iap_verifier/README.md "${RELEASE_DIR}/apple_iap_verifier/"

echo "==> Install Apple IAP verifier production dependencies"
mkdir -p "${NPM_CACHE_DIR}"
npm ci --omit=dev --cache "${NPM_CACHE_DIR}" --prefix "${RELEASE_DIR}/apple_iap_verifier"

cat > "${RELEASE_DIR}/run.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if [[ ! -f config.server.yaml ]]; then
  echo "config.server.yaml not found. Release package is incomplete." >&2
  exit 1
fi

for page in public/index.html public/support.html public/privacy.html public/terms/index.html; do
  if [[ ! -f "${page}" ]]; then
    echo "${page} not found. Release package is incomplete." >&2
    exit 1
  fi
done

if ! command -v node >/dev/null 2>&1; then
  echo "node not found. Install Node.js on this server before starting spider-server." >&2
  exit 1
fi

if [[ ! -f apple_iap_verifier/verify_transaction.mjs ]]; then
  echo "apple_iap_verifier/verify_transaction.mjs not found. Release package is incomplete." >&2
  exit 1
fi

if [[ ! -f apple_iap_verifier/app_store_api.mjs ]]; then
  echo "apple_iap_verifier/app_store_api.mjs not found. Release package is incomplete." >&2
  exit 1
fi

if [[ ! -d apple_iap_verifier/node_modules/@apple/app-store-server-library ]]; then
  echo "Apple IAP verifier node_modules not found. Release package is incomplete." >&2
  exit 1
fi

(
  cd apple_iap_verifier
  node -e "import('@apple/app-store-server-library').then(() => process.exit(0)).catch((err) => { console.error(err); process.exit(1) })"
)

RUN_ENV="${SPIDER_SERVER_ENV:-online}"
if [[ "${RUN_ENV}" == "online" ]]; then
  LOG_DIR="/root/app/spiderapi/log"
else
  LOG_DIR="log"
fi
mkdir -p "${LOG_DIR}"

PID_FILE="spider-server.pid"
LOG_FILE="${LOG_DIR}/spider-server.out.log"

is_process_alive() {
  local pid="${1:-}"
  if [[ -z "${pid}" ]] || ! kill -0 "${pid}" >/dev/null 2>&1; then
    return 1
  fi

  local state=""
  state="$(ps -o stat= -p "${pid}" 2>/dev/null | awk '{print $1}' || true)"
  if [[ "${state}" == Z* ]]; then
    return 1
  fi
  return 0
}

wait_process_exit() {
  local pid="${1:-}"
  for _ in {1..40}; do
    if ! is_process_alive "${pid}"; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

if [[ -f "${PID_FILE}" ]]; then
  OLD_PID="$(cat "${PID_FILE}")"
  if [[ -n "${OLD_PID}" ]] && is_process_alive "${OLD_PID}"; then
    echo "stopping existing spider-server, pid=${OLD_PID}"
    kill "${OLD_PID}" >/dev/null 2>&1 || true
    if ! wait_process_exit "${OLD_PID}"; then
      echo "existing spider-server did not stop in time, killing pid=${OLD_PID}"
      kill -9 "${OLD_PID}" >/dev/null 2>&1 || true
      if ! wait_process_exit "${OLD_PID}"; then
        echo "existing spider-server still alive after SIGKILL, pid=${OLD_PID}" >&2
        exit 1
      fi
    fi
  fi
  rm -f "${PID_FILE}"
fi

if [[ "${RUN_ENV}" == "online" ]]; then
  export SPIDER_SERVER_CONFIG="${SPIDER_SERVER_CONFIG:-config.server.yaml}"
else
  export SPIDER_SERVER_CONFIG="${SPIDER_SERVER_CONFIG:-config.yaml}"
fi

timestamp_out() {
  while IFS= read -r line; do
    printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "${line}"
  done >> "${LOG_FILE}"
}

nohup ./spider-server > >(timestamp_out) 2>&1 &
PID="$!"
echo "${PID}" > "${PID_FILE}"

echo "spider-server started, pid=${PID}"
echo "log: ${LOG_FILE}"
EOF

cat > "${RELEASE_DIR}/stop.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

PID_FILE="spider-server.pid"

if [[ ! -f "${PID_FILE}" ]]; then
  echo "spider-server is not running: pid file not found"
  exit 0
fi

PID="$(cat "${PID_FILE}")"
is_process_alive() {
  local pid="${1:-}"
  if [[ -z "${pid}" ]] || ! kill -0 "${pid}" >/dev/null 2>&1; then
    return 1
  fi

  local state=""
  state="$(ps -o stat= -p "${pid}" 2>/dev/null | awk '{print $1}' || true)"
  if [[ "${state}" == Z* ]]; then
    return 1
  fi
  return 0
}

wait_process_exit() {
  local pid="${1:-}"
  for _ in {1..40}; do
    if ! is_process_alive "${pid}"; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

if [[ -z "${PID}" ]] || ! is_process_alive "${PID}"; then
  rm -f "${PID_FILE}"
  echo "spider-server is not running"
  exit 0
fi

kill "${PID}"
if ! wait_process_exit "${PID}"; then
  echo "spider-server did not stop in time, killing pid=${PID}"
  kill -9 "${PID}" >/dev/null 2>&1 || true
  if ! wait_process_exit "${PID}"; then
    echo "spider-server still alive after SIGKILL, pid=${PID}" >&2
    exit 1
  fi
fi
rm -f "${PID_FILE}"
echo "spider-server stopped, pid=${PID}"
EOF
chmod +x "${RELEASE_DIR}/run.sh" "${RELEASE_DIR}/stop.sh" "${RELEASE_DIR}/${APP_NAME}" "${RELEASE_DIR}/${ADMIN_CLI_NAME}" "${RELEASE_DIR}/${ADMIN_CLI_NAME}.bin"

COPYFILE_DISABLE=1 LC_ALL=C tar -C "${OUTPUT_DIR}" -czf "${TARBALL}" "${RELEASE_NAME}"

echo
echo "==> Done"
echo "Release dir: ${RELEASE_DIR}"
echo "Tarball:     ${TARBALL}"
echo
echo "Server run:"
echo "  tar -xzf ${RELEASE_NAME}.tar.gz"
echo "  cd ${RELEASE_NAME}"
echo "  ./run.sh"
echo "  ./admin-vip-cli"
