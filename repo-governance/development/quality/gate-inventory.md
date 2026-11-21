# Gate Inventory

This is the one table of every check RHINO runs, as rule C19 of the
[Test and Gate Contract](checks/test-and-gate-contract.md) requires. Each row is a gate declared in `repo-config.yml`
under `gates.entries`. A gate on two surfaces has one row per surface. Hooks and workflows call a surface and copy no
command.

Scope follows rule C7. A commit or a changed file is checked as a changed unit. The quick gate is global, because it
includes the static behaviour check.

| Gate                       | Surface      | Scope         | Command                                  | Tests         |
| -------------------------- | ------------ | ------------- | ---------------------------------------- | ------------- |
| public-safety-tree         | pull-request | global        | `check.sh` over the tree                 | none          |
| public-safety-range        | pre-push     | global        | `check.sh` over the pushed range         | none          |
| public-safety-range        | pull-request | global        | `check.sh` over the review range         | none          |
| public-safety-message      | pull-request | global        | `check-commit-message.sh` over messages  | none          |
| remote-dispatch-refusal    | pull-request | global        | `check-remote-dispatch-refusal.sh`       | none          |
| commit-message             | commit-msg   | changed files | `check-commit-message.sh`                | none          |
| commit-message             | pull-request | changed files | `check-commit-message.sh` over range     | none          |
| commit-history             | pull-request | changed files | `check-commit-range.sh` per commit       | none          |
| format-staged              | pre-commit   | changed files | `format-staged.sh`, applies the index    | none          |
| format-staged              | pull-request | changed files | `format-staged.sh`, verify-clean replay  | none          |
| quick                      | pull-request | global        | `./scripts/test-quick.sh` under HIPPO    | unit + static |
| quick                      | scheduled    | global        | `./scripts/test-quick.sh`, unfiltered    | unit + static |
| test-integration           | pull-request | project       | `npm run test:integration`               | integration   |
| test-e2e                   | scheduled    | project       | `npm run test:e2e`                       | e2e           |
| public-safety-tests        | pull-request | global        | `public-safety/tests/run.sh`             | unit          |
| repo-config                | pull-request | global        | `repo-config validate`                   | none          |
| word-budget                | pull-request | global        | `governance word-budget validate`        | none          |
| directory-map              | pull-request | global        | `governance directory-map validate`      | none          |
| quality-gates              | pull-request | global        | `governance quality-gates validate`      | none          |
| harness-adapters           | pull-request | global        | `harness adapters validate`, shell tests | none          |
| check-hippo-consumer-tests | pull-request | global        | `scripts/check-hippo-consumer.test.sh`   | integration   |
| internal-link              | pull-request | global        | `md internal-link validate`              | none          |
| mermaid                    | pull-request | global        | `md mermaid validate`                    | none          |
| markdown-line-length       | pull-request | global        | `check-markdown-line-length.sh`          | none          |
| md-naming                  | pull-request | global        | `md naming validate`                     | none          |
| heading-hierarchy          | pull-request | global        | `md heading-hierarchy validate`          | none          |
| convention-emoji           | pull-request | global        | `convention emoji validate`              | none          |

Each `go run ./cmd/rhino <command>` in the registry is shown here without its prefix. The commands are `repo-config`,
`governance`, `harness`, `md`, and `convention`, and the registry holds the full argv.

## Workflow-Only Checks

These run in the pull-request workflow, outside the registry, because their input is not a repository gate input.

- **Public safety job** (global): the branch name, and the pull-request title and body. Event text has no registry
  input.
- **Pinned HIPPO bootstrap job** (changed files): the pinned bootstrap and the consumer policy. It runs only when the
  consumer boundary or the workflow changes, so an ordinary pull request pays nothing for it.
- **PR leak review** (not in CI): the judgement review that runs and passes before an agent push (C11).

## Exempt Layers and Infeasible Checks

- **Scenario exemptions**: a scenario may skip the integration or end-to-end level, but only with a reason in a
  `# Exemption(<level>): <reason>; alternative-proof: <target>` comment. `behaviour coverage validate` checks each one.
  The unit level is never exempt.
- **Network denial**: `test:quick` runs its unit suite under `unshare -rn`, which denies the network, where the host
  allows user namespaces. Where the host refuses them, the suite runs without denial, and the static lint ban on network
  packages still applies.
