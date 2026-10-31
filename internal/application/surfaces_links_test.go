package application

import (
	"testing"

	"github.com/wahidyankf/rhino/internal/domain"
)

// TestSurfaceFilesFollowsLinksToTheRootAndLiteralGlobs proves two shapes the
// table above does not reach: a link to the root itself, which loops back into
// itself and refuses as a cycle, and a literal glob that names one exact path
// behind a link.
func TestSurfaceFilesFollowsLinksToTheRootAndLiteralGlobs(t *testing.T) {
	root := linkTree{files: []string{"docs/a.md"}, links: []TreeLink{{Path: "mirror", Target: "", Kind: LinkWithin}}}
	if _, refusal := surfaceFiles(root, []string{"mirror/**/*.md"}, nil); refusal == nil || refusal.code != domain.FileUnreadable {
		t.Fatalf("link to the root not refused as a cycle: %+v", refusal)
	}

	literal := linkTree{files: []string{"held/a.md"}, links: []TreeLink{{Path: "linked", Target: "held", Kind: LinkWithin}}}
	got, refusal := surfaceFiles(literal, []string{"linked/a.md"}, nil)
	if refusal != nil {
		t.Fatalf("unexpected refusal %+v", refusal)
	}
	want := []reachedFile{{path: "held/a.md", source: "held/a.md"}, {path: "linked/a.md", source: "held/a.md"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("literal glob: got %+v, want %+v", got, want)
	}
}
