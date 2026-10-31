// Surface links: a declared surface glob follows a link it reaches when the
// link's target stays inside the root. Ported from the oracle's Surfaces::files.
package application

import (
	"sort"
	"strings"

	"github.com/wahidyankf/rhino/internal/domain"
)

// reachedFile is one file a declared surface governs: the path the surface
// names it by, and the path its bytes are read from.
type reachedFile struct {
	path   string
	source string
}

// surfaceRefusal is the refusal a surface walk reports, with its exit code.
type surfaceRefusal struct {
	code   domain.ErrorCode
	reason string
}

// surfaceFiles lists every file the declared globs may govern, in path order:
// each file the walk lists, and each governed file behind a link a glob
// reaches. A link leading outside the root refuses as an escape. A link that
// loops or leads nowhere refuses as unreadable, because a surface that read
// nothing there would report a clean policy it never checked. A link no glob
// reaches is neither followed nor refused.
func surfaceFiles(tree Tree, globs []string, excluded []string) ([]reachedFile, *surfaceRefusal) {
	var walked []string
	for _, path := range tree.Files() {
		if !scanExcluded(path, excluded) {
			walked = append(walked, path)
		}
	}
	var links []TreeLink
	for _, link := range tree.Links() {
		if !scanExcluded(link.Path+"/", excluded) {
			links = append(links, link)
		}
	}
	walk := surfaceWalk{globs: globs, walked: walked, links: links, excluded: excluded}
	reached := make([]reachedFile, 0, len(walked))
	for _, path := range walked {
		reached = append(reached, reachedFile{path: path, source: path})
	}
	for _, link := range links {
		if !walk.reaches(link.Path) {
			continue
		}
		if refusal := walk.follow(link.Path, link, []string{link.Path}, &reached); refusal != nil {
			return nil, refusal
		}
	}
	sort.SliceStable(reached, func(left, right int) bool { return reached[left].path < reached[right].path })
	return reached, nil
}

// surfaceWalk holds what one walk reads: the declared globs, the files the
// walk lists, and the links it may follow.
type surfaceWalk struct {
	globs    []string
	walked   []string
	links    []TreeLink
	excluded []string
}

// follow admits the files a link's target holds, under the path the surface
// reached the link by. chain holds every link already followed on the way
// here: meeting one of them again is a cycle, which would never end.
func (w surfaceWalk) follow(at string, link TreeLink, chain []string, reached *[]reachedFile) *surfaceRefusal {
	switch link.Kind {
	case LinkOutside:
		return &surfaceRefusal{code: domain.PathEscapesRoot,
			reason: at + ": a declared surface reaches a symbolic link that leads outside the repository root"}
	case LinkUnresolved:
		return &surfaceRefusal{code: domain.FileUnreadable,
			reason: at + ": a declared surface reaches a symbolic link whose target cannot be resolved"}
	}
	prefix := ""
	if link.Target != "" {
		prefix = link.Target + "/"
	}
	admit := func(path, source string) {
		if w.governs(path) && !scanExcluded(path, w.excluded) {
			*reached = append(*reached, reachedFile{path: path, source: source})
		}
	}
	for _, source := range w.walked {
		if source == link.Target {
			admit(at, source)
		} else if strings.HasPrefix(source, prefix) {
			admit(at+"/"+source[len(prefix):], source)
		}
	}
	for _, next := range w.links {
		if !strings.HasPrefix(next.Path, prefix) {
			continue
		}
		path := at + "/" + next.Path[len(prefix):]
		if !w.reaches(path) || scanExcluded(path+"/", w.excluded) {
			continue
		}
		if contains(chain, next.Path) {
			return &surfaceRefusal{code: domain.FileUnreadable,
				reason: path + ": a declared surface reaches a symbolic-link cycle"}
		}
		if refusal := w.follow(path, next, append(chain, next.Path), reached); refusal != nil {
			return refusal
		}
	}
	return nil
}

// reaches reports whether any glob could match a path at or under path. It is
// judged by each glob's literal prefix, the segments before its first wildcard,
// because a link's contents are unknown until it is followed.
func (w surfaceWalk) reaches(path string) bool {
	at := strings.Split(path, "/")
	for _, glob := range w.globs {
		segments := strings.Split(glob, "/")
		var literal []string
		for _, segment := range segments {
			if strings.ContainsAny(segment, "*?[{\\") {
				break
			}
			literal = append(literal, segment)
		}
		wild := len(literal) < len(segments)
		common := len(literal)
		if len(at) < common {
			common = len(at)
		}
		agree := true
		for index := 0; index < common; index++ {
			if !strings.EqualFold(literal[index], at[index]) {
				agree = false
				break
			}
		}
		if agree && (wild || len(at) <= len(literal)) {
			return true
		}
	}
	return false
}

// governs reports whether any declared glob matches a path. A glob that cannot
// compile never reaches a link, so the walk never asks it about one, and its
// error is therefore not a case here.
func (w surfaceWalk) governs(path string) bool {
	for _, glob := range w.globs {
		if ok, _ := Match(glob, path); ok {
			return true
		}
	}
	return false
}
