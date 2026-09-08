# Public Repository Data Safety

Never commit information that must stay private, or that unnecessarily identifies the machine, person, account, or private infrastructure used to develop this repository.

RHINO is public and MIT-licensed. Treat every committed file and commit message as globally readable, clonable, redistributable, cacheable, and durable in history. Deleting it in a later commit does not restore confidentiality. `.gitignore` reduces accidental tracking; it is not a security boundary.

## Prohibited Content

Unless a value is deliberately public and approved for this repository, do not commit:

- credentials or authentication material — passwords, tokens, keys, cookies, recovery codes, populated secret environment values;
- personal or machine identifiers — local operating-system usernames, home-directory names, hostnames, device names, and machine-specific absolute paths;
- private network or infrastructure identifiers — tailnet or VPN names, internal domains, private addresses, hardware addresses, network names, account identifiers, local mount or share paths; or
- logs, screenshots, fixtures, generated files, configuration, or examples containing any of the above.

Public identifiers this repository documents on purpose, such as its canonical GitHub URL, are allowed. So is commit-author identity the owner configured deliberately.

## Safe Representation

Use repository-relative paths wherever possible. Otherwise use an unmistakable placeholder — `<repo-root>`, `<username>` — or a documented environment variable. Examples and fixtures use synthetic values that cannot be mistaken for real credentials or real infrastructure.

Inspect the complete proposed change before every commit: staged content, intended untracked files, generated artifacts, and the message. The [merge preconditions](pull-request-merge.md) require the same review of the exact head being merged, because no tooling checks this.

## If It Already Landed

Do not repeat the value in diagnostics or reports. Treat a credential as compromised and rotate it. Preserve evidence without exposing the value, report the affected scope, and obtain authority before rewriting shared history.
