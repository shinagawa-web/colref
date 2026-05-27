#!/usr/bin/env sh
set -eu

REPO="shinagawa-web/colref"
BINARY="colref"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

case "$(uname -s)" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="darwin" ;;
  *) echo "error: unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "error: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "${VERSION}" ]; then
  echo "error: could not determine latest release version" >&2
  exit 1
fi

VERSION_NUMBER="${VERSION#v}"
TARBALL="${BINARY}_${VERSION_NUMBER}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

echo "Installing ${BINARY} ${VERSION} (${OS}/${ARCH}) to ${INSTALL_DIR}..."

TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT

curl -fsSL "${BASE_URL}/${TARBALL}" -o "${TMP}/${TARBALL}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP}/checksums.txt"

cd "${TMP}"
EXPECTED=$(grep "${TARBALL}" checksums.txt | awk '{print $1}')
if [ -z "${EXPECTED}" ]; then
  echo "error: ${TARBALL} not found in checksums.txt" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "${TARBALL}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "${TARBALL}" | awk '{print $1}')
else
  echo "warning: sha256sum and shasum not found; skipping checksum verification" >&2
  ACTUAL="${EXPECTED}"
fi
if [ "${ACTUAL}" != "${EXPECTED}" ]; then
  echo "error: checksum mismatch for ${TARBALL}" >&2
  exit 1
fi

tar -xzf "${TARBALL}"

mkdir -p "${INSTALL_DIR}"
if [ -w "${INSTALL_DIR}" ]; then
  mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  echo "Requesting elevated permissions to install to ${INSTALL_DIR}..."
  sudo mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

echo "${BINARY} ${VERSION} installed to ${INSTALL_DIR}/${BINARY}"
