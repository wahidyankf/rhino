# adapters/runtime

The read-only disk tree, the operating-system gate launcher, the disk toolchain runner, the disk environment store, the
disk adapter store, the disk model resolver, and the disk mutation runner. The tree never follows a symlink and never
leaves the root; the launcher starts only declared gate children; the runner starts only declared toolchain probes and
provision vectors, directly and without a shell; the environment store is the path that writes declared environment
targets, staged before installing; the adapter store replaces each declared adapter family atomically, staged before
installing, and refuses any link on the way in; the resolver runs only declared model-listing vectors, bounded and
without a shell, and reports only their standard output. The mutation runner is the only writer of a `type: mutation`
gate: `apply-index` formats a checkout-index snapshot and writes changed blobs back to the index, and `verify-clean`
replays a candidate revision in a detached worktree that is always removed. `apply-index` refuses a selected symbolic
link and a change outside its selection, and both refuse an escaping selection.
