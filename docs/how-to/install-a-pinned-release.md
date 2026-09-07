# How to install a pinned release

You want a specific RHINO build on PATH, and you want to know it is the build
you asked for.

Pin a version. A hygiene tool that changes what it enforces between two runs of
the same gate turns a clean repository red without anyone editing it.

## Download and verify

Releases are published from the [repository releases
page](https://github.com/wahidyankf/rhino/releases) with a checksum file. Fetch
the archive for your platform and the checksum, then verify **before**
extracting:

```sh
VERSION=v0.1.0
TARGET=aarch64-apple-darwin   # or x86_64-apple-darwin, x86_64-unknown-linux-gnu, aarch64-unknown-linux-gnu
BASE=https://github.com/wahidyankf/rhino/releases/download/$VERSION

curl -fsSLO "$BASE/rhino-$VERSION-$TARGET.tar.gz"
curl -fsSLO "$BASE/rhino-$VERSION-$TARGET.tar.gz.sha256"

# shasum on macOS, sha256sum on Linux.
shasum -a 256 -c "rhino-$VERSION-$TARGET.tar.gz.sha256"
```

Verifying afterwards tells you what you already ran. Verify first.

## Put it on PATH

```sh
tar -xzf "rhino-$VERSION-$TARGET.tar.gz"
install -m 0755 rhino "$HOME/.local/bin/rhino"
```

Confirm the build is the one you pinned:

```console
$ rhino version
0.1.0

$ rhino version --json
{"schemaVersion":1,"version":"0.1.0","commit":"5ef5ff83ef15eb45885210b9a6fa91419d1c6004"}
```

The commit is embedded at build time, so `--json` tells you exactly which
revision produced _your_ binary — the hash above is whichever revision built the
one this page was written against. A build made outside a repository reports
forty zeros rather than lying about it.

## Build from source instead

RHINO is a single Rust binary with no build-time code generation beyond the
embedded commit.

```sh
git clone https://github.com/wahidyankf/rhino
cd rhino
cargo build --release
./target/release/rhino version
```

The toolchain is pinned in `rust-toolchain.toml`, so `cargo` will fetch the
right compiler rather than fail on whichever one you happen to have.

## Keep the pin visible

Record the version and its checksum somewhere the repository can see — a lock
file, a variable at the top of the gate script, a comment beside the CI step.
An unpinned tool is a policy that changes without a commit.

## Related

- [How to wire RHINO into your gates](./wire-rhino-into-your-gates.md)
- [Command line](../reference/cli.md)
