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
VERSION=REPLACE_WITH_RELEASE_TAG
TARGET=aarch64-apple-darwin   # or x86_64-apple-darwin, x86_64-unknown-linux-gnu, aarch64-unknown-linux-gnu
BASE=https://github.com/wahidyankf/rhino/releases/download/$VERSION

curl -fsSLO "$BASE/rhino-$TARGET.tar.gz"
curl -fsSLO "$BASE/checksums.txt"

# One file records every platform's digest, so `--ignore-missing` checks the
# archive you fetched instead of failing over the three you did not.
# shasum on macOS, sha256sum on Linux.
shasum -a 256 --ignore-missing -c checksums.txt
```

Replace `REPLACE_WITH_RELEASE_TAG` with a tag the releases page does not mark
`Pre-release`; the one marked `Latest` is the newest. A pre-release is a
candidate, not a build to pin. A grouped `rhino/repo-config/v2` configuration
needs `v0.4.0` or later.

Verifying afterwards tells you what you already ran. Verify first.

## Put it on PATH

```sh
tar -xzf "rhino-$TARGET.tar.gz"
mkdir -p "$HOME/.local/bin"
install -m 0755 rhino "$HOME/.local/bin/rhino"
```

`$HOME/.local/bin` has to be on your `PATH` for the next command to find
`rhino`. If `command -v rhino` prints nothing, add
`export PATH="$HOME/.local/bin:$PATH"` to your shell's startup file and open a
new shell.

Confirm the build is the one you pinned:

```console
$ rhino version
```

`rhino version --json` reports the same version alongside the forty-character
commit embedded at build time, as
`{"schemaVersion":1,"version":"v0.1.1","commit":"…"}`. The commit is not written
out here on purpose: it is a property of _your_ binary rather than of this page,
and a hash printed beside a version is one a reader will compare against theirs
and find different. Compare it against your lock file instead. A build made
outside a repository reports forty zeros rather than lying about it.

## Build from source instead

RHINO is a single Rust binary. Its checked-in configuration schema derives from
the typed model and is verified separately; a release archive never fetches it
while running.

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
