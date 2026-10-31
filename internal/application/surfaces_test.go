package application

import (
	"testing"

	"github.com/wahidyankf/rhino/internal/domain"
)

// linkTree is the fake tree a surface walk reads: its files and its links.
type linkTree struct {
	files []string
	links []TreeLink
}

func (f linkTree) Root() string                     { return "" }
func (f linkTree) Files() []string                  { return f.files }
func (f linkTree) Read(string) (string, *ReadError) { return "", nil }
func (f linkTree) RootedAt(string) (Tree, error)    { return f, nil }
func (f linkTree) Children(string) []string         { return nil }
func (f linkTree) IsDirectory(string) bool          { return false }
func (f linkTree) Exists(string) bool               { return false }
func (f linkTree) PassesThroughLink(string) bool    { return false }
func (f linkTree) Links() []TreeLink                { return f.links }

// TestSurfaceFilesFollowsOnlyReachedLinks proves a surface glob follows a link
// it reaches when the target stays inside the root, names the governed file
// under the link path, and reads it from the target. Each refusal names its
// code, and a link no glob reaches is neither followed nor refused.
func TestSurfaceFilesFollowsOnlyReachedLinks(t *testing.T) {
	tests := []struct {
		name     string
		tree     linkTree
		globs    []string
		excluded []string
		want     []reachedFile
		code     domain.ErrorCode
		refused  bool
	}{
		{
			name:  "inside target is followed under the link path",
			tree:  linkTree{files: []string{"held/a.md", "other.md"}, links: []TreeLink{{Path: "linked", Target: "held", Kind: LinkWithin}}},
			globs: []string{"linked/**/*.md"},
			want:  []reachedFile{{path: "held/a.md", source: "held/a.md"}, {path: "linked/a.md", source: "held/a.md"}, {path: "other.md", source: "other.md"}},
		},
		{
			name:  "a link to a single file is followed under its own path",
			tree:  linkTree{files: []string{"held/main.md"}, links: []TreeLink{{Path: "notes.md", Target: "held/main.md", Kind: LinkWithin}}},
			globs: []string{"*.md"},
			want:  []reachedFile{{path: "held/main.md", source: "held/main.md"}, {path: "notes.md", source: "held/main.md"}},
		},
		{
			name:  "a wildcard-first glob reaches any link",
			tree:  linkTree{files: []string{"held/b.md"}, links: []TreeLink{{Path: "any", Target: "held", Kind: LinkWithin}}},
			globs: []string{"**/*.md"},
			want:  []reachedFile{{path: "any/b.md", source: "held/b.md"}, {path: "held/b.md", source: "held/b.md"}},
		},
		{
			name:  "a nested link inside a followed directory is followed too",
			tree:  linkTree{files: []string{"held/deep/x.md"}, links: []TreeLink{{Path: "linked", Target: "held", Kind: LinkWithin}, {Path: "held/inner", Target: "held/deep", Kind: LinkWithin}}},
			globs: []string{"linked/**/*.md"},
			want:  []reachedFile{{path: "held/deep/x.md", source: "held/deep/x.md"}, {path: "linked/deep/x.md", source: "held/deep/x.md"}, {path: "linked/inner/x.md", source: "held/deep/x.md"}},
		},
		{
			name:  "a target outside the root refuses as an escape",
			tree:  linkTree{links: []TreeLink{{Path: "linked", Kind: LinkOutside}}},
			globs: []string{"linked/**/*.md"},
			code:  domain.PathEscapesRoot, refused: true,
		},
		{
			name:  "a target that resolves nowhere refuses as unreadable",
			tree:  linkTree{links: []TreeLink{{Path: "linked", Kind: LinkUnresolved}}},
			globs: []string{"linked/**/*.md"},
			code:  domain.FileUnreadable, refused: true,
		},
		{
			name:  "a link that loops back into itself refuses as unreadable",
			tree:  linkTree{files: []string{"docs/next.md"}, links: []TreeLink{{Path: "docs/loop", Target: "docs", Kind: LinkWithin}}},
			globs: []string{"docs/**/*.md"},
			code:  domain.FileUnreadable, refused: true,
		},
		{
			name:  "an outside link no glob reaches is neither followed nor refused",
			tree:  linkTree{files: []string{"docs/a.md"}, links: []TreeLink{{Path: "elsewhere", Kind: LinkOutside}}},
			globs: []string{"docs/**/*.md"},
			want:  []reachedFile{{path: "docs/a.md", source: "docs/a.md"}},
		},
		{
			name:     "an excluded directory is neither walked nor followed",
			tree:     linkTree{files: []string{"vendor/x.md", "docs/a.md"}, links: []TreeLink{{Path: "vendor/link", Kind: LinkOutside}}},
			globs:    []string{"**/*.md"},
			excluded: []string{"vendor"},
			want:     []reachedFile{{path: "docs/a.md", source: "docs/a.md"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, refusal := surfaceFiles(test.tree, test.globs, test.excluded)
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
