#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NAME="spider-server"
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

if command -v npm >/dev/null 2>&1; then
  :
else
  echo "npm not found. Online package requires npm to vendor Apple IAP verifier dependencies." >&2
  exit 1
fi

rm -rf "${RELEASE_DIR}" "${TARBALL}"
mkdir -p "${RELEASE_DIR}/apple_iap_verifier" "${RELEASE_DIR}/logs"

cd "${ROOT_DIR}"

CGO_ENABLED=0 GOOS="${TARGET_OS}" GOARCH="${TARGET_ARCH}" \
  go build -trimpath -ldflags "-s -w" -o "${RELEASE_DIR}/${APP_NAME}" ./cmd

cp config.server.example.yaml "${RELEASE_DIR}/config.server.example.yaml"
cp config.server.example.yaml "${RELEASE_DIR}/config.server.yaml"
echo "==> Included config.server.example.yaml as config.server.yaml in release package"

cp apple_iap_verifier/verify_transaction.mjs "${RELEASE_DIR}/apple_iap_verifier/"
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

if ! command -v node >/dev/null 2>&1; then
  echo "node not found. Install Node.js on this server before starting spider-server." >&2
  exit 1
fi

if [[ ! -f apple_iap_verifier/verify_transaction.mjs ]]; then
  echo "apple_iap_verifier/verify_transaction.mjs not found. Release package is incomplete." >&2
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

mkdir -p logs

PID_FILE="spider-server.pid"
LOG_FILE="logs/spider-server.out.log"

if [[ -f "${PID_FILE}" ]]; then
  OLD_PID="$(cat "${PID_FILE}")"
  if [[ -n "${OLD_PID}" ]] && kill -0 "${OLD_PID}" >/dev/null 2>&1; then
    echo "spider-server is already running, pid=${OLD_PID}"
    exit 0
  fi
  rm -f "${PID_FILE}"
fi

export SPIDER_SERVER_CONFIG="${SPIDER_SERVER_CONFIG:-config.server.yaml}"
nohup ./spider-server >> "${LOG_FILE}" 2>&1 &
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
if [[ -z "${PID}" ]] || ! kill -0 "${PID}" >/dev/null 2>&1; then
  rm -f "${PID_FILE}"
  echo "spider-server is not running"
  exit 0
fi

kill "${PID}"
rm -f "${PID_FILE}"
echo "spider-server stopped, pid=${PID}"
EOF
chmod +x "${RELEASE_DIR}/run.sh" "${RELEASE_DIR}/stop.sh" "${RELEASE_DIR}/${APP_NAME}"

tar -C "${OUTPUT_DIR}" -czf "${TARBALL}" "${RELEASE_NAME}"

echo
echo "==> Done"
echo "Release dir: ${RELEASE_DIR}"
echo "Tarball:     ${TARBALL}"
echo
echo "Server run:"
echo "  tar -xzf ${RELEASE_NAME}.tar.gz"
echo "  cd ${RELEASE_NAME}"
echo "  ./run.sh"
