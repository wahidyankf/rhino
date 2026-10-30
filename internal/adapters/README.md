# adapters

Boundary implementations behind application-declared interfaces.

- `cli/` adapts operating-system streams and signals: stdin selection, version stamping, and stream emission with
  closed-pipe mapping to `141`.
- `config/` reads `repo-config.yml` without defaults: schema shape, unknown keys, extension owners, and lifecycle
  composition checks.
- `runtime/` walks the repository read-only: no symlink following, no network, no processes, UTF-8 text only.
