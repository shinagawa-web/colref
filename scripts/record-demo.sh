#!/usr/bin/env bash
#
# Record the README demo GIF with VHS (https://github.com/charmbracelet/vhs).
#
# Usage:
#   ./scripts/record-demo.sh
#
# Requires vhs on PATH (e.g. `brew install vhs`; see the VHS README for
# Linux packages and manual downloads).
set -euo pipefail

# Always run from the repository root so `demo.tape`'s `export PATH=$PWD:$PATH`
# and relative paths resolve correctly.
cd "$(dirname "$0")/.."

if ! command -v vhs >/dev/null 2>&1; then
  echo "error: vhs is not installed. Install it, e.g. \`brew install vhs\` (see https://github.com/charmbracelet/vhs)." >&2
  exit 1
fi

echo "Building colref binary..."
make build

mkdir -p docs/static

echo "Recording demo.tape -> docs/static/demo.gif ..."
vhs demo.tape

echo "Done. Wrote docs/static/demo.gif"
