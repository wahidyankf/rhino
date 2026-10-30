# adapters/runtime

The read-only disk tree, the operating-system gate launcher, the disk toolchain runner, the disk
environment store, the disk adapter store, and the disk model resolver. The tree never follows a symlink and never leaves the root; the launcher starts only declared
gate children; the runner starts only declared toolchain probes and provision vectors, directly and without a
shell; the environment store is the path that writes declared environment targets, staged before installing; the
adapter store replaces each declared adapter family atomically, staged before installing, and refuses any link on
the way in; the resolver runs only declared model-listing vectors, bounded and without a shell, and reports only
their standard output.
