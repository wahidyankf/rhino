# Commit Authorization

Commit and push only when the user has authorized it, or when an approved plan step authorizes it.

## Requirements

- Authorization to make a change is not authorization to commit it. Authorization to commit is not authorization to push.
- Authorization does not cross a repository boundary. Permission to commit here says nothing about any other repository, including one this repository pins.
- Authorization does not persist beyond the scope it was given for. A grant for one change does not cover the next.
- Merging a pull request is authorized separately, by the [pull-request merge convention](pull-request-merge.md) rather than by a prompt at the moment of merging.
- Bypassing a hook is a third, separate permission. See [push-hook verification](push-hook-verification.md); a normal push request never authorizes `--no-verify`.
- Publishing a release is separate again, and follows [release cut](../workflows/release-cut.md).

Choosing how to integrate a change is not authorization either: the [integration path](integration-path.md) says where a change goes, not whether it may go.

## Related

- [Integration path](integration-path.md)
- [Push-hook verification](push-hook-verification.md)
