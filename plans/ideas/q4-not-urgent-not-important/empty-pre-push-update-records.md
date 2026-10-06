# Tell a push that publishes nothing from a missing update stream

`gate run --surface pre-push --push-updates-stdin` refuses with exit `2` when standard input holds no update record, but
Git sends none when every ref is already up to date or will be rejected, so a push that would publish nothing is
reported as a failed push.

> Filed 2026-10-06 by a consuming repository under its Upstream Tool Defects standard. Reproduced on released `v0.11.0`
> (`db711617a95ed67700f42d6652c5a9c271a166e6`); trunk `e8bb00cb8fc8c5e36feff5cae84bc637ec20244c` carries the same code,
> since no commit between the two touches `src/v0_4/gates.rs` or `src/main.rs`. Low severity, with a workaround.

## Problem and evidence

### Description

`parse_push_range` in [`src/v0_4/gates.rs`](../../../src/v0_4/gates.rs) returns "received no pre-push update record"
when its input has no line, and the dispatcher refuses the run as `rhino.input.unreadable`. Git runs the `pre-push` hook
even when it has nothing to send and leaves the hook's standard input empty, so the hook fails, Git aborts, and a no-op
push ends in `error: failed to push some refs`. A script that re-pushes defensively sees a failure where nothing went
wrong.

### Steps to reproduce

From an empty directory, with a `rhino` binary on `PATH` and a Git identity configured:

```sh
git init -q --bare remote.git
git init -q -b main work && cd work
cat > repo-config.yml <<'EOF'
schema: rhino/repo-config/v2
gates:
  entries:
    - id: range-check
      type: check
      inputs:
        range: { kind: commit-range }
      command:
        executable: echo
        args:
          - { input: range.base, expand: single }
      run-on:
        pre-push:
          bind:
            range: { source: push-updates, fallback: refs/remotes/origin/main }
        pull-request:
          bind:
            range: { source: explicit-range, range: explicit }
  composition:
    pull-request:
      relation: exact
EOF
git add repo-config.yml && git commit -qm one
git remote add origin ../remote.git && git push -q origin main
printf '#!/bin/sh\nexec rhino gate run --surface pre-push --push-updates-stdin\n' > .git/hooks/pre-push
chmod +x .git/hooks/pre-push
git push origin main; echo "exit $?"
git push --dry-run --force-with-lease origin main; echo "exit $?"
git commit -q --allow-empty -m two && git push origin main; echo "exit $?"
git reset -q --hard HEAD~1 && git push origin main; echo "exit $?"
rhino gate run --surface pre-push --push-updates-stdin < /dev/null; echo "exit $?"
```

### Expected behaviour

The first two pushes publish nothing, so there is nothing to screen. Without the hook Git reports
`Everything up-to-date` and exits `0`. The fourth push is non-fast-forward, and without the hook Git explains why:
`! [rejected] main -> main (non-fast-forward)` and a `hint:` to integrate the remote changes. RHINO already passes a
push that publishes no commit when the record is a deletion: the
[v0.4 configuration](../../../docs/reference/v0-4-configuration.md#lifecycle-gates) says "a deleted ref skips the range
gate because it has no head to inspect", and a contract scenario in
[`v0-4-contract.feature`](../../../specs/behaviours/v0-4-contract.feature) pins it. Nothing documented says an empty
record set is refused.

### Actual behaviour

From the `v0.11.0` binary, with commit IDs elided:

```text
rhino: gate `range-check`: received no pre-push update record
error: failed to push some refs to '../remote.git'
exit 1
rhino: gate `range-check`: received no pre-push update record
error: failed to push some refs to '../remote.git'
exit 1
[gate] range-check passed
To ../remote.git
   <old>..<new>  main -> main
exit 0
rhino: gate `range-check`: received no pre-push update record
error: failed to push some refs to '../remote.git'
exit 1
rhino: gate `range-check`: received no pre-push update record
exit 2
```

Only the third push, which publishes a commit, passes. The fourth hides Git's own rejection reason behind RHINO's. The
fifth, a run by hand with an empty stream, prints the same refusal as the hook runs.

The consumer met it through the documented wiring: its `.husky/pre-push` is
`exec ./rhino gate run --surface pre-push --push-updates-stdin`, and a repeated `git push --force-with-lease` after a
successful push failed with ``rhino: gate `public-safety-range`: received no pre-push update record`` and
`husky - pre-push script failed (code 2)`.

### Environment

macOS on arm64, Apple Git 2.39.5, and the released `v0.11.0` archive.

### Documentation consulted

- Git's [githooks: pre-push][githooks] (read 2026-10-06) says: "Information about what is to be pushed is provided on
  the hook's standard input with lines of the form", followed by
  `<local-ref> SP <local-object-name> SP <remote-ref> SP <remote-object-name> LF`. It does not say what the stream holds
  when nothing is to be pushed; the local `git help githooks` page for 2.39.5 carries the same text.
- Git's [`transport.c`][transport] (master, read 2026-10-06) answers it: `pre_push_hook_feed_stdin` skips
  `REF_STATUS_REJECT_NONFASTFORWARD`, `REF_STATUS_REJECT_REMOTE_UPDATED`, `REF_STATUS_REJECT_STALE`, and
  `REF_STATUS_UPTODATE` refs with the comment "skip refs which won't be pushed", and the hook still runs when every ref
  is skipped. A probe hook counting its input saw `origin ../remote.git` as arguments and zero lines on an up-to-date
  push.
- RHINO's [CLI reference](../../../docs/reference/cli.md) describes `--push-updates-stdin` as "Read Git update records
  from standard input"; it does not mention an empty stream.

### Impact

Low. The failure is closed: nothing unscreened is published. The cost is a false failure on a no-op push, a masked
non-fast-forward or stale-lease rejection, and automation that has to guard its own re-pushes. RHINO's own
[`.husky/pre-push`](../../../.husky/pre-push) uses the same wiring, so its contributors meet it too.

## Why now

No time pressure: the workaround is one comparison before the push. The brief parks the sighting so the next consumer
whose script re-pushes does not rediscover it, and so the tension below is argued once.

## Prior art

Duplicate check, run 2026-10-06 against `wahidyankf/rhino`:

- `gh issue list --repo wahidyankf/rhino --state all --search 'pre-push'`: none; the repository has no issues.
- `gh pr list --repo wahidyankf/rhino --state open`: none open.
- Merged pull requests touching the same message: #80 and #83, below. Neither addresses an empty record set.
- `git grep` over `plans/` for "update record", "pre-push", "no-op push", "up to date", and "up-to-date": only three
  delivery items in [`harden-rhino-validation-edge-cases`](../../backlog/harden-rhino-validation-edge-cases/README.md)
  that expect the pre-push hook to pass, unrelated. `plans/in-progress/` holds no plan, and no idea brief exists.

Precedent inside RHINO, which is the other side of the tension:

- [`wants_stdin`](../../../src/main.rs) reads standard input only when `--push-updates-stdin` or `--file -` selects it,
  so "a run by hand from a terminal ... never waits on it". #83 made that so, because a pre-push run without the flag
  waited for an end-of-file nobody meant to send.
- #80 fixed `rhino --root . gate run --surface pre-push --push-updates-stdin`, which never read standard input and
  refused with "received no pre-push update record". The refusal is what exposed that defect. Had an empty stream
  passed, every push through that form would have published unscreened.
- The [public contract](../../../repo-governance/development/public-contract.md) holds that "an unread tree and a clean
  tree are not the same claim", which is why a check that cannot run reports `2`.

## Proposed direction

The tension: an empty stream from Git means nothing will be published, so there is nothing to screen. An empty stream
from a broken invocation, or from a hook wrapper that consumed or never forwarded standard input, means the screen did
not run. RHINO receives the same zero bytes in both cases.

- **Pass with a notice.** Treat a present but empty stream like a deletion: skip each `push-updates` range gate, say on
  standard error that the push publishes no ref, and exit `0`. Smallest change, and Git then reports its own result. It
  trusts every caller to forward the stream, which is the guarantee #80 showed can fail silently.
- **Pass only on a positive signal.** Keep refusing by default, and let the caller say an empty stream is Git's: an
  added flag that carries Git's remote name and URL, or an added binding key on `push-updates` choosing whether an empty
  record set skips or refuses. Adding a flag or a key is compatible; the cost is wiring every consumer's hook again.
- **Keep refusing, name the case.** Leave exit `2`, and make the message say that Git sends no record when every ref is
  up to date or rejected, so this push would have published nothing. Message wording is not public, so this is a patch.
  It ends the confusion, not the false failure or the masked rejection.

## Scope and non-goals

In scope: what `gate run --surface pre-push --push-updates-stdin` does with a stream that holds no record.

Out of scope: pushes that carry more than one record, which are refused separately ("select one declared ref first");
forwarding Git's hook arguments to gate children, which [`.husky/pre-push`](../../../.husky/pre-push) declines on
purpose.

## Risks and open questions

- Whether a skip on empty input weakens the claim behind exit `0`. A deletion already skips, but a deletion arrives as a
  record RHINO has read; an empty stream proves nothing about the caller.
- Whether any common hook manager hands the hook an empty stream for a push that does publish commits. If one does, the
  first direction lets that push through unscreened.
- Whether changing the exit from `2` to `0` for this input needs a minor or a major version under the
  [public contract](../../../repo-governance/development/public-contract.md).

## Workaround

Do not re-push when nothing changed: compare `git rev-parse HEAD` with the remote-tracking ref first, and skip the push
when they are equal. The consumer's scripts now do this.

## Success and promotion signal

Success: the first two pushes in the reproduction no longer end in `error: failed to push some refs`, or, under the
third direction, the message names the no-op push; and a run whose caller never forwarded the stream is still refused.

Promote when a second consumer meets it, when RHINO's own release or review automation re-pushes an unchanged branch, or
when `parse_push_range` is next touched for any other reason.

[githooks]: https://git-scm.com/docs/githooks#_pre_push
[transport]: https://github.com/git/git/blob/master/transport.c
