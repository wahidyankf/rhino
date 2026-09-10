# Scenario: A shape set carrying a literal private term is malformed.
#
# The generic layer ships in public repositories. A literal row would be the
# private identifier itself, committed, which is the thing the wrapper exists to
# stop. The confidential layer that does hold literals lives in wkf-devbox and
# is never copied here.
run() {
	local out rc shapes="$CASE_TMP/shapes.txt"
	printf 'nothing to see\n' >"$CASE_TMP/clean.txt"

	printf 'private-repository literal some-private-name\n' >"$shapes"
	out=$("$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/clean.txt" 2>&1)
	rc=$?
	assert_exit 2 "$rc" "exit code for a literal row" || return 1
	assert_contains "shapes only" "$out" "refusal" || return 1
	assert_absent "some-private-name" "$out" "refusal" || return 1

	# A credential-shaped pattern is malformed for the same reason: a shape set
	# is read by anyone who clones the repository.
	printf 'maintainer-path regex ghp_0123456789abcdef0123456789abcdef0123\n' >"$shapes"
	out=$("$WRAPPER" --surface release --shapes "$shapes" --file "$CASE_TMP/clean.txt" 2>&1)
	rc=$?
	assert_exit 2 "$rc" "exit code for a credential-shaped pattern" || return 1
}
