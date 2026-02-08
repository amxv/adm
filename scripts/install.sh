#!/usr/bin/env bash
# install.sh - Install adm binary
#
# Usage:
#   curl -fsSL <release-url>/install.sh | bash
#
# Environment variables:
#   ADM_VERSION   - Version to install (default: latest from LATEST file)
#   ADM_INSTALL   - Install directory (default: /usr/local/bin or ~/.local/bin)
#   ADM_BASE_URL  - Base URL for release artifacts
#
# The script:
#   1. Detects OS and architecture
#   2. Downloads the matching release archive
#   3. Verifies SHA-256 checksum
#   4. Installs the binary to the target directory

set -euo pipefail

BINARY="adm"

# --- Configuration ---

BASE_URL="${ADM_BASE_URL:-https://github.com/amxv/adm/releases/download}"

# Detect OS.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    darwin|linux) ;;
    *) echo "Error: unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture.
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Resolve version.
VERSION="${ADM_VERSION:-}"
if [[ -z "$VERSION" ]]; then
    LATEST_URL="${BASE_URL}/latest/LATEST"
    VERSION="$(curl -fsSL "$LATEST_URL" 2>/dev/null)" || {
        echo "Error: could not fetch latest version from $LATEST_URL" >&2
        echo "Set ADM_VERSION explicitly, e.g.: ADM_VERSION=v0.1.0 bash install.sh" >&2
        exit 1
    }
fi

# Resolve install directory.
INSTALL_DIR="${ADM_INSTALL:-}"
if [[ -z "$INSTALL_DIR" ]]; then
    if [[ -w /usr/local/bin ]]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi
fi

# --- Download and verify ---

ARCHIVE_NAME="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
ARCHIVE_URL="${BASE_URL}/${VERSION}/${ARCHIVE_NAME}"
CHECKSUMS_URL="${BASE_URL}/${VERSION}/checksums.txt"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Installing ${BINARY} ${VERSION} (${OS}/${ARCH})..."
echo "  Archive: ${ARCHIVE_URL}"

# Download archive and checksums.
curl -fsSL -o "${TMPDIR}/${ARCHIVE_NAME}" "$ARCHIVE_URL" || {
    echo "Error: failed to download ${ARCHIVE_URL}" >&2
    exit 1
}
curl -fsSL -o "${TMPDIR}/checksums.txt" "$CHECKSUMS_URL" || {
    echo "Error: failed to download checksums" >&2
    exit 1
}

# Verify checksum.
EXPECTED=$(grep "${ARCHIVE_NAME}" "${TMPDIR}/checksums.txt" | awk '{print $1}')
if [[ -z "$EXPECTED" ]]; then
    echo "Error: no checksum found for ${ARCHIVE_NAME}" >&2
    exit 1
fi

ACTUAL=$(shasum -a 256 "${TMPDIR}/${ARCHIVE_NAME}" | awk '{print $1}')
if [[ "$EXPECTED" != "$ACTUAL" ]]; then
    echo "Error: checksum mismatch" >&2
    echo "  Expected: ${EXPECTED}" >&2
    echo "  Actual:   ${ACTUAL}" >&2
    exit 1
fi
echo "  Checksum: verified"

# Extract and install.
tar -xzf "${TMPDIR}/${ARCHIVE_NAME}" -C "${TMPDIR}"
EXTRACTED_DIR="${TMPDIR}/${BINARY}_${VERSION}_${OS}_${ARCH}"

install -m 755 "${EXTRACTED_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
echo "  Installed: ${INSTALL_DIR}/${BINARY}"

# Verify.
if command -v "${BINARY}" &>/dev/null; then
    echo "  Version: $(${BINARY} --version 2>&1 | head -1)"
else
    echo ""
    echo "Note: ${INSTALL_DIR} may not be on your PATH."
    echo "Add it with: export PATH=\"${INSTALL_DIR}:\$PATH\""
fi

echo "Done."
