#!/bin/sh
set -eu

# Build the release archives for one pinned commit.
# Mirrors the hippo structural template: clean-checkout gate, toolchain pin
# from go.mod, CGO-free matrix, identity stamped by ldflags, checksums.
# The deterministic archiver is a documented gap: archives carry the commit's
# mtime but member ownership and compression still come from the host.

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY \
	GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR GIT_NAMESPACE GIT_PREFIX

if [ "$#" -ne 3 ]; then
	echo "usage: $0 <version> <commit> <output-dir>" >&2
	exit 1
fi

version=$1
commit=$2
output_dir=$3
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)

if ! printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'; then
	echo "version must be a v-prefixed semantic version" >&2
	exit 1
fi
if [ "${#commit}" -ne 40 ]; then
	echo "commit must be a 40-character lowercase Git object ID" >&2
	exit 1
fi
case "$commit" in
*[!0-9a-f]*)
	echo "commit must be a 40-character lowercase Git object ID" >&2
	exit 1
	;;
esac
if ! git -C "$root" cat-file -e "$commit^{commit}" 2>/dev/null; then
	echo "commit must identify a real Git commit" >&2
	exit 1
fi
head_commit=$(git -C "$root" rev-parse HEAD)
if [ "$commit" != "$head_commit" ]; then
	echo "commit must equal checkout HEAD" >&2
	exit 1
fi
if ! checkout_status=$(git -C "$root" status --porcelain --untracked-files=all); then
	echo "release checkout cleanliness could not be verified" >&2
	exit 1
fi
if [ -n "$checkout_status" ]; then
	echo "release checkout must be clean, including untracked files" >&2
	exit 1
fi

mkdir -p "$output_dir"
output_dir=$(CDPATH='' cd -- "$output_dir" && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/rhino-release.XXXXXX")
cleanup() {
	rm -rf -- "$work"
}
trap 'cleanup' EXIT HUP INT TERM

toolchain=$(sed -n 's/^toolchain[[:space:]][[:space:]]*\(go[^[:space:]]*\)[[:space:]]*$/\1/p' "$root/go.mod")
if [ -z "$toolchain" ]; then
	toolchain=go$(sed -n 's/^go[[:space:]][[:space:]]*\([^[:space:]]*\)[[:space:]]*$/\1/p' "$root/go.mod")
fi
if ! printf '%s\n' "$toolchain" | grep -Eq '^go[0-9]+\.[0-9]+\.[0-9]+$'; then
	echo "go.mod must name an exact Go release to build with" >&2
	exit 1
fi
GOTOOLCHAIN=$toolchain
GOENV=off
export GOTOOLCHAIN GOENV
unset GOFLAGS GOEXPERIMENT GOAMD64 GOARM64 GOFIPS140

modified=$(git -C "$root" show -s --format=%ct "$commit")

for target in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64; do
	goos=${target%_*}
	goarch=${target#*_}
	binary="$work/rhino"
	archive="rhino_${version}_${goos}_${goarch}.tar.gz"
	(
		cd "$root"
		CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -p=1 -trimpath -buildvcs=true \
			-ldflags "-s -w -X github.com/wahidyankf/rhino/internal/application.Version=$version -X github.com/wahidyankf/rhino/internal/application.Commit=$commit" \
			-o "$binary" ./cmd/rhino
	)
	(
		cd "$work"
		MTIME="$modified" tar --sort-name --mtime="@$modified" --owner=0 --group=0 --mode=755 -czf "$output_dir/$archive" rhino
	)
done

(
	cd "$output_dir"
	sha256sum rhino_*.tar.gz >checksums.txt
)

trap - EXIT HUP INT TERM
cleanup
