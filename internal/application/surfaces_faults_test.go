package application

import (
	"testing"

	"github.com/wahidyankf/rhino/internal/domain"
)

// TestSurfaceFilesFollowsLinksOnlyWhereGlobsReach proves a link's contents are
// governed by the glob that names them, not by the link alone. A followed
// directory admits only the files a glob matches, a nested link no glob reaches
// is skipped, and an escape beneath a followed link refuses the whole walk.
func TestSurfaceFilesFollowsLinksOnlyWhereGlobsReach(t *testing.T) {
	tests := []struct {
		name    string
		tree    linkTree
		globs   []string
		want    []reachedFile
		code    domain.ErrorCode
		refused bool
	}{
		{
			name:  "a followed directory admits only the files its glob names beneath the link",
			tree:  linkTree{files: []string{"held/a.md", "held/sub/b.md"}, links: []TreeLink{{Path: "linked", Target: "held", Kind: LinkWithin}}},
			globs: []string{"linked/*.md"},
			want: []reachedFile{
				{path: "held/a.md", source: "held/a.md"},
				{path: "held/sub/b.md", source: "held/sub/b.md"},
				{path: "linked/a.md", source: "held/a.md"},
			},
		},
		{
			name:  "a nested link that no glob reaches is neither followed nor admitted",
			tree:  linkTree{files: []string{"held/deep/x.md"}, links: []TreeLink{{Path: "linked", Target: "held", Kind: LinkWithin}, {Path: "held/inner", Target: "held/deep", Kind: LinkWithin}}},
			globs: []string{"linked/x/*.md"},
			want:  []reachedFile{{path: "held/deep/x.md", source: "held/deep/x.md"}},
		},
		{
			name:    "an escape beneath a followed link refuses the whole walk",
			tree:    linkTree{files: []string{"held/a.md"}, links: []TreeLink{{Path: "linked", Target: "held", Kind: LinkWithin}, {Path: "held/inner", Kind: LinkOutside}}},
			globs:   []string{"linked/**/*.md"},
			code:    domain.PathEscapesRoot,
			refused: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, refusal := surfaceFiles(test.tree, test.globs, nil)
			if test.refused {
				if refusal == nil || refusal.code != test.code {
					t.Fatalf("got refusal %+v, want code %v", refusal, test.code)
				}
				return
			}
			if refusal != nil {
				t.Fatalf("unexpected refusal %+v", refusal)
			}
			if len(got) != len(test.want) {
				t.Fatalf("got %+v, want %+v", got, test.want)
			}
			for index := range got {
				if got[index] != test.want[index] {
					t.Fatalf("got %+v, want %+v", got, test.want)
				}
			}
		})
	}
}
