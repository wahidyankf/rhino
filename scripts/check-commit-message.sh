#!/usr/bin/env bash
# Commitlint consumes the typed message text from either Git's hook file or an
# immutable pull-request range; it never receives an arbitrary pathname.
set -euo pipefail

: "${RHINO_GATE_MESSAGE:?RHINO_GATE_MESSAGE is required}"
printf '%s\n' "$RHINO_GATE_MESSAGE" | npm exec -- commitlint
