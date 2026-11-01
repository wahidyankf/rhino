# bootstrap

Composes production adapters for the command entry point: the standard streams, and six runtime parts wired into the
command boundary:

- `Launcher`: the operating-system gate launcher.
- `Environment`: the disk environment store.
- `Toolchains`: the disk toolchain runner.
- `Adapters`: the disk adapter store.
- `Models`: the disk model resolver.
- `Mutations`: the disk mutation runner.
