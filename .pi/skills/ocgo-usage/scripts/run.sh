#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
# Run the ocgo-usage CLI (installed at ~/.local/bin/ocgo-usage, on PATH).
set -euo pipefail

if ! command -v ocgo-usage >/dev/null 2>&1; then
  echo "ocgo-usage: binary not found on PATH — install it first:" >&2
  echo '  go build -o ~/.local/bin/ocgo-usage .' >&2
  echo '  (from a checkout of the ocgo-usage repo)' >&2
  exit 2
fi

exec ocgo-usage "$@"