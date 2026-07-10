#!/usr/bin/env bash
# Builds foundry and installs it on PATH so `foundry` works from any directory.
#
# Usage (from a local clone):
#   ./install.sh
#
# Usage (no clone needed):
#   curl -fsSL https://raw.githubusercontent.com/SahidAyala/Foundry/main/install.sh | bash
set -euo pipefail

BIN_NAME="foundry"
INSTALL_DIR="${FOUNDRY_INSTALL_DIR:-/usr/local/bin}"
REPO_URL="${FOUNDRY_REPO_URL:-https://github.com/SahidAyala/Foundry.git}"
REPO_REF="${FOUNDRY_REF:-main}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is not installed or not on PATH" >&2
  exit 1
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT
BUILD_DIR="$WORK_DIR/bin"
mkdir -p "$BUILD_DIR"

if [ -f "$SCRIPT_DIR/go.mod" ] && [ -d "$SCRIPT_DIR/cmd/foundry" ]; then
  SRC_DIR="$SCRIPT_DIR"
else
  if ! command -v git >/dev/null 2>&1; then
    echo "error: git is not installed or not on PATH (needed to fetch the source)" >&2
    exit 1
  fi
  echo "Fetching source from $REPO_URL@$REPO_REF..."
  SRC_DIR="$WORK_DIR/src"
  git clone --quiet --depth 1 --branch "$REPO_REF" "$REPO_URL" "$SRC_DIR"
fi

echo "Building ${BIN_NAME}..."
(cd "$SRC_DIR" && go build -o "$BUILD_DIR/$BIN_NAME" ./cmd/foundry)

mkdir -p "$INSTALL_DIR"

DEST="$INSTALL_DIR/$BIN_NAME"
if [ -w "$INSTALL_DIR" ]; then
  install -m 755 "$BUILD_DIR/$BIN_NAME" "$DEST"
else
  echo "Elevated permissions needed to write to $INSTALL_DIR"
  sudo install -m 755 "$BUILD_DIR/$BIN_NAME" "$DEST"
fi

echo "Installed: $DEST"

if ! command -v "$BIN_NAME" >/dev/null 2>&1; then
  echo
  echo "warning: $INSTALL_DIR is not on your PATH."
  echo "Add this to your shell profile (~/.zshrc, ~/.bashrc, ...):"
  echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
else
  echo "Run 'foundry' from anywhere to start."
fi
