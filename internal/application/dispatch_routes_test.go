package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// leafFlags supplies one valid instance of each flag a leaf accepts, so the leaf
// parses and reaches the dispatch arm for its category. A message file, the
// stdin push flag, and a range parse only with the surface that takes them, and
// routing does not depend on them, so they are not supplied.
func leafFlags(accepts []application.Accepts) []string {
	var out []string
	for _, accept := range accepts {
		switch accept {
		case application.AcceptFile:
			out = append(out, "--file", "README.md")
		case application.AcceptDirectory:
			out = append(out, "--directory", ".")
		case application.AcceptBackupDirectory:
			out = append(out, "--dir", "backup")
		case application.AcceptApply:
			out = append(out, "--apply")
		case application.AcceptForce:
			out = append(out, "--force")
		case application.AcceptSurface:
			out = append(out, "--surface", "pre-commit")
		case application.AcceptJSON:
			out = append(out, "--json")
		}
	}
	return out
}

// TestEveryLeafInTheCommandTableRoutesToItsCore proves each leaf of the closed
// command table parses and reaches a routed dispatch arm. A leaf the dispatch
// does not route panics, so this test names that leaf and fails when a leaf is
// added to the table without a case.
func TestEveryLeafInTheCommandTableRoutesToItsCore(t *testing.T) {
	for _, leaf := range application.Leaves {
		t.Run(strings.Join(leaf.Path, " "), func(t *testing.T) {
			argv := append(append([]string(nil), leaf.Path...), leafFlags(leaf.Accepts)...)
			if _, refusal := application.Parse(argv); refusal != nil {
				t.Fatalf("leaf refused at parse, so it never reaches dispatch: %s", refusal.Message)
			}
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = metadataAgentConfig
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("category `%s` panicked instead of routing: %v", leaf.Category, recovered)
				}
			}()
			application.Execute(tree, argv, nil)
		})
	}
}
