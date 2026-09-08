# Working Tree

What is tracked, what is ignored, and what the validator is allowed to read are three different questions. Answering only the first is how a gate ends up reporting findings against a copy of the repository.

## Ignored

Keep build output (`target/`, `dist/`), coverage output, `node_modules/`, and `local-tmp/` scratch out of history. Nothing generated is committed.

`local-tmp/` is the scratch directory. Audits, reports, and working notes go there. Nothing in it is authoritative, and nothing in it is a rule.

## Worktrees Live Inside the Repository

Task worktrees live at `worktrees/<name>/` so that this repository's own rules apply to them. Only the placeholder is tracked; a worktree never enters history.

That placement has a cost that must be paid explicitly. The validator walks the filesystem rather than the Git index, so a git-ignored directory is still read. An in-repository worktree is a **complete second copy** of the tree: unexcluded, it doubles every counted document, every resolved link, and every mapped directory — and reports the copy's own `CLAUDE.md` as a second always-on instruction source. Measured here, it took the link count from 109 to 218.

The failure mode is not a crash. It is a gate reporting findings that look real.

## Every Consumer Needs Telling Separately

There is no single exclusion list. Each tool that walks the tree needs its own entry, and a missing one is silent:

- `.gitignore` — `/worktrees/*` with `!/worktrees/.gitkeep`.
- `repo-config.yml` — `scan.exclude-directories` covers `worktrees` and `local-tmp`, matched at any depth.
- `.prettierignore` — `worktrees`, measured to be load-bearing: without it a misformatted file inside a worktree is reported; with it, silent. Prettier does not infer this from `.gitignore`.

Adding a new tool that walks the tree means adding its exclusion in the same change. Verify by populating a worktree and confirming every gate reports exactly what it reported before.

## Related

- [Integration path](integration-path.md)
- [Worktree to pull request](../workflows/worktree-to-pull-request.md)
