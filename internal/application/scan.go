// Scanning: path selection over the read-only tree.
package application

import (
	"fmt"
	"strings"
)

// Match reports whether a repository-relative path matches a declared glob.
// Case-insensitive; ** spans separators, * and ? stay inside one segment.
// An absolute or climbing pattern never matches: silently governing nothing
// would be worse than refusing, and the compiler reports that earlier.
func Match(pattern, path string) (bool, error) {
	if strings.HasPrefix(pattern, "/") || isClimbing(pattern) {
		return false, fmt.Errorf("`%s` must be relative to the repository root", pattern)
	}
	if err := checkBrackets(pattern); err != nil {
		return false, fmt.Errorf("`%s`: %s", pattern, err.Error())
	}
	patternSegments := strings.Split(strings.ToLower(pattern), "/")
	pathSegments := strings.Split(strings.ToLower(path), "/")
	return matchSegments(patternSegments, pathSegments), nil
}

// checkBrackets refuses a pattern with an unterminated character class. A
// bad pattern that matched nothing would be a policy that quietly governs
// nothing.
func checkBrackets(pattern string) error {
	for i := 0; i < len(pattern); i++ {
		if pattern[i] != '[' {
			continue
		}
		j := i + 1
		if j < len(pattern) && (pattern[j] == '!' || pattern[j] == '^') {
			j++
		}
		if j < len(pattern) && pattern[j] == ']' {
			j++
		}
		for j < len(pattern) && pattern[j] != ']' {
			j++
		}
		if j >= len(pattern) {
			return fmt.Errorf("unterminated character class")
		}
		i = j
	}
	return nil
}

func isClimbing(pattern string) bool {
	for _, segment := range strings.Split(pattern, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func matchSegments(pattern, path []string) bool {
	if len(pattern) == 0 {
		return len(path) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(path); i++ {
			if matchSegments(pattern[1:], path[i:]) {
				return true
			}
		}
		return false
	}
	if len(path) == 0 {
		return false
	}
	if !matchSegment(pattern[0], path[0]) {
		return false
	}
	return matchSegments(pattern[1:], path[1:])
}

func matchSegment(pattern, name string) bool {
	return matchBytes([]byte(pattern), []byte(name))
}

// matchBytes matches one path segment. Character classes were validated by
// checkBrackets, so an opening bracket always closes here.
func matchBytes(pattern, name []byte) bool {
	px, nx := 0, 0
	star := -1
	mark := 0
	for nx < len(name) {
		if px < len(pattern) && pattern[px] == '[' {
			end := classEnd(pattern[px:])
			if end >= 0 && classMatches(pattern[px+1:px+end], name[nx]) {
				px += end + 1
				nx++
				continue
			}
			if star != -1 {
				px = star + 1
				mark++
				nx = mark
				continue
			}
			return false
		}
		if px < len(pattern) && (pattern[px] == '?' || pattern[px] == name[nx]) {
			px++
			nx++
		} else if px < len(pattern) && pattern[px] == '*' {
			star = px
			mark = nx
			px++
		} else if star != -1 {
			px = star + 1
			mark++
			nx = mark
		} else {
			return false
		}
	}
	for px < len(pattern) && pattern[px] == '*' {
		px++
	}
	return px == len(pattern)
}

// classEnd returns the offset of the closing bracket, or -1.
func classEnd(pattern []byte) int {
	i := 1
	if i < len(pattern) && (pattern[i] == '!' || pattern[i] == '^') {
		i++
	}
	if i < len(pattern) && pattern[i] == ']' {
		i++
	}
	for i < len(pattern) {
		if pattern[i] == ']' {
			return i
		}
		i++
	}
	return -1
}

func classMatches(class []byte, char byte) bool {
	negate := false
	if len(class) > 0 && (class[0] == '!' || class[0] == '^') {
		negate = true
		class = class[1:]
	}
	matched := false
	for i := 0; i < len(class); i++ {
		if i+2 < len(class) && class[i+1] == '-' {
			if class[i] <= char && char <= class[i+2] {
				matched = true
			}
			i += 2
			continue
		}
		if class[i] == char {
			matched = true
		}
	}
	if negate {
		return !matched
	}
	return matched
}

// scanExcluded mirrors the oracle's scan.is_excluded: whether any directory
// on the way to a file is one the repository excludes. The file's own name
// is never tested, so a file called `generated` is inspected while everything
// inside a directory called `generated` is not. Names match exactly, at any
// depth: a sibling that merely shares a prefix stays scanned.
func scanExcluded(path string, excluded []string) bool {
	segments := strings.Split(path, "/")
	for _, segment := range segments[:len(segments)-1] {
		for _, name := range excluded {
			if segment == name {
				return true
			}
		}
	}
	return false
}

// scanFiles returns the walked files outside every shared-excluded directory,
// in walk order: the shared selection every scan-based validator starts from.
// An omitted scan section excludes nothing; the walk itself is unchanged.
func scanFiles(files []string, excluded []string) []string {
	if len(excluded) == 0 {
		return files
	}
	var out []string
	for _, file := range files {
		if !scanExcluded(file, excluded) {
			out = append(out, file)
		}
	}
	return out
}

// scanMarkdown returns the Markdown files outside every shared-excluded
// directory, in walk order: the shared corpus every Markdown validator reads.
func scanMarkdown(files []string, excluded []string) []string {
	var out []string
	for _, file := range files {
		if isMarkdown(file) && !scanExcluded(file, excluded) {
			out = append(out, file)
		}
	}
	return out
}

// Select returns the tree files matched by any of the globs, in sorted
// order. A bad pattern refuses the run: it would otherwise govern nothing.
func Select(files []string, key string, patterns []string) ([]string, error) {
	var selected []string
	seen := map[string]bool{}
	for _, pattern := range patterns {
		for _, file := range files {
			ok, err := Match(pattern, file)
			if err != nil {
				return nil, fmt.Errorf("%s: %s", key, err.Error())
			}
			if ok && !seen[file] {
				seen[file] = true
				selected = append(selected, file)
			}
		}
	}
	sortStrings(selected)
	return selected, nil
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
