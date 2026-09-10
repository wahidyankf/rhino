# Scenario: A malformed shape set makes publication ineligible.
run() {
	local out rc shapes="$CASE_TMP/shapes.txt"
	printf 'nothing to see\n' >"$CASE_TMP/clean.txt"

	attempt() {
		out=$("$WRAPPER" --surface release --shapes "$1" --file "$CASE_TMP/clean.txt" 2>&1)
		rc=$?
	}

	attempt "$CASE_TMP/absent.txt"
	assert_exit 2 "$rc" "exit code for an absent shape set" || return 1
	assert_contains "absent" "$out" "refusal" || return 1

	: >"$CASE_TMP/unreadable.txt"
	chmod 000 "$CASE_TMP/unreadable.txt"
	attempt "$CASE_TMP/unreadable.txt"
	chmod 644 "$CASE_TMP/unreadable.txt"
	assert_exit 2 "$rc" "exit code for an unreadable shape set" || return 1

	printf '# only a comment\n\n' >"$shapes"
	attempt "$shapes"
	assert_exit 2 "$rc" "exit code for an empty shape set" || return 1
	assert_contains "empty" "$out" "refusal" || return 1

	printf 'maintainer-path\n' >"$shapes"
	attempt "$shapes"
	assert_exit 2 "$rc" "exit code for a row with too few fields" || return 1

	printf 'no-such-class regex abc\n' >"$shapes"
	attempt "$shapes"
	assert_exit 2 "$rc" "exit code for an unknown class" || return 1

	printf 'maintainer-path regex [unclosed\n' >"$shapes"
	attempt "$shapes"
	assert_exit 2 "$rc" "exit code for an unusable regular expression" || return 1

	# The refusal names what could not be trusted. It never quotes the row: a
	# malformed shape set in a private repository can still carry a private
	# term, and a diagnostic that echoed it would publish it.
	printf 'maintainer-path regex [unclosed\n' >"$shapes"
	attempt "$shapes"
	assert_absent "[unclosed" "$out" "refusal" || return 1
}
