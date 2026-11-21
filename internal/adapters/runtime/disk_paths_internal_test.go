package runtime

import "testing"

// The path predicates answer for the inputs a policy decoder never lets reach
// a run: an absolute path, a bare root, and a plain path that stays inside the
// tree. Each runs with the file system double, so no host path is read.
func TestPathPredicatesAnswerForRootedAndPlainInputs(t *testing.T) {
	if !leavesRoot("/absolute") {
		t.Fatal("an absolute path is not reported as leaving the root")
	}
	if isNormalPath("/") {
		t.Fatal("a bare root is reported as a normal path")
	}

	useDoubles(t, repository(&callLog{}, map[string]string{"a/b": "x"}), nil, nil)
	tree := DiskTree{Dir: "/repo"}
	for _, path := range []string{"", ".", "a/b"} {
		if tree.PassesThroughLink(path) {
			t.Fatalf("PassesThroughLink(%q) reported a link on a plain path", path)
		}
	}
}
