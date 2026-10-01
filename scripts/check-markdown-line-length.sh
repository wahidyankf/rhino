#!/usr/bin/env bash
# Enforce the 120-character Markdown line length across the whole tree.
# The rule, its exemptions, and why tables count are in
# repo-governance/conventions/markdown-line-length.md; .markdownlint-cli2.jsonc
# enables MD013 alone so this gate judges nothing else.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"
exec "$repo_root/node_modules/.bin/markdownlint-cli2"
