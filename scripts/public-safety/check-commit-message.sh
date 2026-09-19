#!/usr/bin/env bash
# Screen the typed commit-message input selected by the lifecycle dispatcher.
set -euo pipefail

here=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
: "${RHINO_GATE_MESSAGE:?RHINO_GATE_MESSAGE is required}"

exec "$here/public-safety.sh" --surface commit --text "$RHINO_GATE_MESSAGE"
