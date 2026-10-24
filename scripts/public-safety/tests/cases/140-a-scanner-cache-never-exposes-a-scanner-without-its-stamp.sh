# Scenario: A scanner cache never exposes a scanner without its stamp.
#
# The wrapper used to extract the pinned scanner straight into its final cache
# directory and stamp it afterwards. A first run that overlapped another, or one
# that died in between, left an executable with no record of what it was, and the
# cache-hit branch then refused it on every later run.
#
# Offline. The download is a fake curl that writes dummy bytes. The digest tools
# answer the approved digest for the tarball alone and delegate everything else.
# The fake tar copies the suite's stub scanner into place and can then hold, which
# keeps a real run paused between extraction and stamping. Every wait is bounded,
# and the only process a case signals is one it started itself.

# Sets the per-scenario state. Each scenario gets its own cache and its own fake-tar
# bookkeeping, so none can observe another scenario's extraction.
use_scenario() {
	SCN="$CASE_TMP/$1"
	CACHE="$SCN/cache"
	PARENT="$CACHE/trufflehog/3.97.1"
	FINAL="$PARENT/$PLAT"
	rm -rf "$SCN"
	mkdir -p "$SCN" || return 1
	export FAKE_SCENARIO="$SCN"
}

# Installs the fake download and digest tools. Each answers only the call the
# wrapper makes; anything else reaches the real tool found on the original PATH.
install_fakes() {
	mkdir -p "$BIN" || return 1

	cat >"$BIN/curl" <<-'SH'
		#!/usr/bin/env bash
		# Fake curl: records that a download was attempted and writes dummy bytes to -o.
		: >"$FAKE_SCENARIO/curl-called"
		out=""
		while [[ $# -gt 0 ]]; do
			if [[ "$1" == -o ]]; then
				out=${2:-}
				shift
			fi
			shift
		done
		[[ -n "$out" ]] || exit 2
		printf 'not a real archive\n' >"$out"
	SH

	cat >"$BIN/tar" <<-'SH'
		#!/usr/bin/env bash
		# Fake tar: answers the wrapper's one extraction and hands every other call to the real tar.
		if [[ "$1" != -xzf || "$3" != -C || "$5" != trufflehog ]]; then
			exec env PATH="$FAKE_ORIG_PATH" tar "$@"
		fi
		dest=$4
		case "${FAKE_TAR_MODE:-normal}" in
		tar-fail)
			printf 'partial' >"$dest/trufflehog"
			exit 1
			;;
		dangling)
			ln -s "$CASE_TMP/absent" "$dest/trufflehog"
			exit 0
			;;
		esac
		cp "$STUB" "$dest/trufflehog" || exit 1
		if [[ "${FAKE_TAR_MODE:-normal}" == hold ]] && mkdir "$FAKE_SCENARIO/first-extraction" 2>/dev/null; then
			# The first extraction stops here: the binary is in place and the wrapper has
			# not stamped it yet. It resumes on release, and exits when the wrapper that
			# started it is gone, so nothing waits forever.
			: >"$FAKE_SCENARIO/extracted"
			for ((i = 0; i < 600; i++)); do
				[[ -e "$FAKE_SCENARIO/release" ]] && exit 0
				kill -0 "$PPID" 2>/dev/null || exit 1
				sleep 0.1
			done
			exit 1
		fi
		exit 0
	SH

	cat >"$BIN/shasum" <<-'SH'
		#!/usr/bin/env bash
		# Fake digest tool. The pinned tarball gets the approved digest, or an override;
		# every other file is digested by the real tool of the same name.
		name=${0##*/}
		last=""
		for arg; do
			last=$arg
		done
		case "$last" in
		*/trufflehog.tar.gz)
			printf '%s  %s\n' "${FAKE_TARBALL_DIGEST:-$FAKE_WANT}" "$last"
			exit 0
			;;
		esac
		exec env PATH="$FAKE_ORIG_PATH" "$name" "$@"
	SH
	cp "$BIN/shasum" "$BIN/sha256sum" || return 1

	cat >"$BIN/mv" <<-'SH'
		#!/usr/bin/env bash
		# Fake mv. With FAKE_MV_RACE=1, the first publish of a staging directory loses a
		# race: a complete winning run publishes the entry just before this rename.
		case "$*" in
		*.stage.*)
			if [[ "${FAKE_MV_RACE:-0}" == 1 ]] && mkdir "$FAKE_SCENARIO/mv-raced" 2>/dev/null; then
				"$WRAPPER" --surface release --shapes "$SHAPES" --file "$CLEAN" \
					>"$FAKE_SCENARIO/winner-output" 2>&1
			fi
			;;
		esac
		exec env PATH="$FAKE_ORIG_PATH" mv "$@"
	SH
	chmod +x "$BIN/curl" "$BIN/tar" "$BIN/shasum" "$BIN/sha256sum" "$BIN/mv" || return 1
}

# Runs the wrapper once against this scenario's cache and prints its output.
# Arguments: the fake tar mode, and optionally a tarball digest override.
wrapper_run() {
	FAKE_TAR_MODE="$1" FAKE_TARBALL_DIGEST="${2:-}" PATH="$BIN:$PATH" PUBLIC_SAFETY_CACHE="$CACHE" \
		"$WRAPPER" --surface release --shapes "$SHAPES" --file "$CLEAN" 2>&1
}

# Starts a run in the background whose first extraction is held. Sets HELD_PID to
# the wrapper's own PID, which is the only process this case ever signals.
start_held_run() {
	FAKE_TAR_MODE=hold PATH="$BIN:$PATH" PUBLIC_SAFETY_CACHE="$CACHE" \
		"$WRAPPER" --surface release --shapes "$SHAPES" --file "$CLEAN" >"$SCN/held-output" 2>&1 &
	HELD_PID=$!
}

# Waits, bounded, for a file to appear while the given process is alive.
wait_for_file() {
	local i
	for ((i = 0; i < 100; i++)); do
		[[ -e "$1" ]] && return 0
		kill -0 "$2" 2>/dev/null || return 1
		sleep 0.1
	done
	return 1
}

# Waits, bounded, for a background process this case started. It is signalled only
# if it overruns the bound.
reap_bounded() {
	local pid=$1 i
	for ((i = 0; i < 300; i++)); do
		kill -0 "$pid" 2>/dev/null || break
		sleep 0.1
	done
	if kill -0 "$pid" 2>/dev/null; then
		kill -KILL "$pid" 2>/dev/null
		wait "$pid" 2>/dev/null
		return 124
	fi
	wait "$pid"
}

# A cache path that holds a binary without its stamp is the exposure this case exists to catch.
exposed() {
	[[ -e "$1/trufflehog" && ! -e "$1/trufflehog.sha256" ]]
}

# Staging directories are the only other entries a run may leave in the version directory.
staged_entries() {
	find "$1" -mindepth 1 -maxdepth 1 -name '.stage.*' 2>/dev/null
}

note() {
	printf '    %s\n' "$*" >&2
	FAILS=$((FAILS + 1))
}

expect_exit() {
	assert_exit "$1" "$2" "$3" || FAILS=$((FAILS + 1))
}

# The cache path holds a scanner with its stamp, and the stamp is that scanner's digest.
check_entry() {
	local label=$1
	if [[ ! -e "$FINAL/trufflehog" || ! -e "$FINAL/trufflehog.sha256" ]]; then
		note "$label: the cache path does not hold a scanner with its stamp"
		return 0
	fi
	if [[ "$(digest_file "$FINAL/trufflehog")" != "$(cat "$FINAL/trufflehog.sha256")" ]]; then
		note "$label: the stamp does not match the published scanner"
	fi
	return 0
}

digest_file() {
	if command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d' ' -f1
	else
		sha256sum "$1" | cut -d' ' -f1
	fi
}

# Two first-time runs overlap. The first is paused between extraction and stamping;
# the second must complete, and the first must then use the published entry.
overlap_scenario() {
	local rc_a rc_b out
	use_scenario overlap || {
		note "cannot prepare the overlap scenario"
		return 1
	}
	start_held_run
	if wait_for_file "$SCN/extracted" "$HELD_PID"; then
		if exposed "$FINAL"; then
			note "paused between extraction and stamping, a scanner is at the cache path without its stamp"
		fi
	else
		note "the held run never reached extraction"
	fi

	out=$(wrapper_run normal)
	rc_b=$?
	expect_exit 0 "$rc_b" "exit code of the overlapping run"
	if [[ "$rc_b" -ne 0 ]]; then
		printf '    the overlapping run printed: %s\n' "$(printf '%s' "$out" | tail -1)" >&2
	fi

	: >"$SCN/release"
	reap_bounded "$HELD_PID"
	rc_a=$?
	expect_exit 0 "$rc_a" "exit code of the paused run"

	check_entry overlap
	if [[ "$(ls -A "$PARENT" 2>/dev/null)" != "$PLAT" ]]; then
		note "overlap: the version directory holds more than the one published entry"
	fi
	if [[ -n "$(staged_entries "$PARENT")" ]]; then
		note "overlap: a staging directory was left behind"
	fi
}

# Two runs race on the publish itself. The winner publishes after this run has
# checked that the entry is absent and before this run renames. The rename then
# lands inside the winner's entry, so this run must discard that copy and keep the
# winner's entry exactly as it was published.
rename_race_scenario() {
	local out rc
	use_scenario rename-race || {
		note "cannot prepare the rename-race scenario"
		return 1
	}
	export FAKE_MV_RACE=1
	out=$(wrapper_run normal)
	rc=$?
	export FAKE_MV_RACE=0
	expect_exit 0 "$rc" "exit code of the run that loses the rename race"
	if [[ ! -e "$SCN/mv-raced" ]]; then
		note "rename race: the race was never injected, so the scenario proved nothing"
	fi
	check_entry rename-race
	if [[ "$(ls -A "$FINAL" 2>/dev/null)" != $'trufflehog\ntrufflehog.sha256' ]]; then
		note "rename race: the winner's entry holds more than its scanner and stamp"
	fi
	if [[ "$(ls -A "$PARENT" 2>/dev/null)" != "$PLAT" ]]; then
		note "rename race: the version directory holds more than the one published entry"
	fi
	if [[ -n "$(staged_entries "$PARENT")" ]]; then
		note "rename race: a staging directory was left behind"
	fi
}

# A run is killed between extraction and stamping. Nothing may be exposed at the
# cache path, and a fresh run must still bootstrap cleanly.
killed_scenario() {
	local out rc
	use_scenario killed || {
		note "cannot prepare the killed-run scenario"
		return 1
	}
	start_held_run
	if ! wait_for_file "$SCN/extracted" "$HELD_PID"; then
		note "the run to be killed never reached extraction"
	fi

	# SIGKILL reaches only the wrapper this case started. Its held tar is left alone:
	# it exits on its own once the wrapper that started it is gone.
	kill -KILL "$HELD_PID" 2>/dev/null
	wait "$HELD_PID" 2>/dev/null

	if exposed "$FINAL"; then
		note "a killed run left a scanner at the cache path without its stamp"
	fi

	out=$(wrapper_run normal)
	rc=$?
	expect_exit 0 "$rc" "exit code of a fresh run after the killed run"
	check_entry killed
}

# A run is terminated while paused between extraction and stamping. It must refuse,
# publish nothing, and leave no staging directory behind.
terminated_scenario() {
	local rc
	use_scenario terminated || {
		note "cannot prepare the terminated-run scenario"
		return 1
	}
	start_held_run
	if ! wait_for_file "$SCN/extracted" "$HELD_PID"; then
		note "the run to be terminated never reached extraction"
	fi

	# The wrapper runs its trap only after the foreground extraction returns, so the
	# hold is released after the signal.
	kill -TERM "$HELD_PID" 2>/dev/null
	: >"$SCN/release"
	reap_bounded "$HELD_PID"
	rc=$?
	expect_exit 2 "$rc" "exit code of a terminated run"
	assert_contains "scan-error interrupted" "$(cat "$SCN/held-output")" "refusal of a terminated run" ||
		FAILS=$((FAILS + 1))
	if [[ -e "$FINAL" ]]; then
		note "a terminated run published a scanner"
	fi
	if [[ -n "$(staged_entries "$PARENT")" ]]; then
		note "a terminated run left its staging directory"
	fi
}

# Extraction fails after it began. The refusal must leave no staging directory and
# no entry at the cache path.
faults_scenario() {
	local mode out rc
	for mode in tar-fail dangling; do
		use_scenario "after-extraction-$mode" || {
			note "cannot prepare the $mode scenario"
			return 1
		}
		out=$(wrapper_run "$mode")
		rc=$?
		expect_exit 2 "$rc" "exit code when extraction fails ($mode)"
		assert_contains "scan-error" "$out" "refusal when extraction fails ($mode)" ||
			FAILS=$((FAILS + 1))
		if [[ -e "$FINAL" ]]; then
			note "a failed extraction ($mode) published a scanner"
		fi
		if [[ -n "$(staged_entries "$PARENT")" ]]; then
			note "a failed extraction ($mode) left its staging directory"
		fi
	done
}

# The tarball digest gate is not weakened: a mismatch refuses before extraction.
digest_mismatch_scenario() {
	local out rc
	use_scenario digest-mismatch || {
		note "cannot prepare the digest-mismatch scenario"
		return 1
	}
	out=$(wrapper_run normal 0000000000000000000000000000000000000000000000000000000000000000)
	rc=$?
	expect_exit 2 "$rc" "exit code for a tarball digest mismatch"
	assert_contains "digest mismatch" "$out" "refusal of a tarball digest mismatch" ||
		FAILS=$((FAILS + 1))
	if [[ -n "$(staged_entries "$PARENT")" ]] || [[ -e "$FINAL" ]]; then
		note "a digest mismatch left a staging directory or a cache entry"
	fi
}

# A binary with no stamp at the cache path is refused, not re-downloaded.
unstamped_scenario() {
	local out rc
	use_scenario unstamped || {
		note "cannot prepare the unstamped scenario"
		return 1
	}
	mkdir -p "$FINAL" || return 1
	cp "$STUB" "$FINAL/trufflehog" || return 1
	chmod +x "$FINAL/trufflehog" || return 1

	out=$(wrapper_run normal)
	rc=$?
	expect_exit 2 "$rc" "exit code for a scanner with no stamp at the cache path"
	assert_contains "carries no record" "$out" "refusal of an unstamped scanner" ||
		FAILS=$((FAILS + 1))
	if [[ -e "$SCN/curl-called" ]]; then
		note "an unstamped scanner sent the run to the network"
	fi
}

run() {
	FAILS=0
	unset PUBLIC_SAFETY_SCANNER
	BIN="$CASE_TMP/fake-bin"
	SHAPES="$CASE_TMP/shapes.txt"
	CLEAN="$CASE_TMP/clean.txt"
	write_synthetic_shapes "$SHAPES"
	printf 'nothing to see\n' >"$CLEAN"

	PLAT=$("$WRAPPER" --print-platform) || {
		echo "    this host is outside the pinned platform matrix" >&2
		return 1
	}
	WANT=$("$WRAPPER" --print-pin | awk -v p="$PLAT" '$1 == p { print $2 }')
	if [[ -z "$WANT" ]]; then
		echo "    no approved digest is recorded for this host" >&2
		return 1
	fi
	export FAKE_WANT="$WANT" FAKE_ORIG_PATH="$PATH" CASE_TMP SHAPES CLEAN FAKE_MV_RACE=0
	install_fakes || return 1

	overlap_scenario
	rename_race_scenario
	killed_scenario
	terminated_scenario
	faults_scenario
	digest_mismatch_scenario
	unstamped_scenario

	[[ "$FAILS" -eq 0 ]]
}
