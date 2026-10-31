// Directory-map validation: every directory in a declared tree documents its contents.
//
// Within a declared tree, every directory carries a README.md whose
// `## Directory Map` section lists each direct sibling exactly once with a
// resolvable relative target. A map that drifted from the tree is worse than
// no map, because it is believed.
package application

import (
	"fmt"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// directoryMapSection is the one section a mapped README must carry.
const directoryMapSection = "## Directory Map"

// directoryMapReadme is the file a directory documents itself in.
const directoryMapReadme = "README.md"

func directoryMapValidate(document *config.Document, tree Tree, format Format, directory *string) Outcome {
	mapping, ok := document.PolicyGroup("governance", "directory-map")
	if !ok {
		report := NewReport("directory-map", "directory")
		report.Refusal = strptr("policies.governance.directory-map is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	trees, refusal := decodeDirectoryMap(mapping)
	if refusal != nil {
		return directoryMapRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	// A declared tree outside the root, or behind a link, is not an absent
	// tree: skipping it would report a clean map nobody inspected.
	for _, declared := range trees {
		if !isExactPath(declared) {
			return directoryMapRefused(format, domain.ConfigUnusable,
				fmt.Sprintf("policies.governance.directory-map.trees: `%s` is not an exact repository-relative path", declared))
		}
		if tree.PassesThroughLink(declared) {
			return directoryMapRefused(format, domain.FileUnreadable,
				declared+": passes through a symbolic link, which RHINO never follows")
		}
	}
	skipped, err := document.ScanExcludeDirectories()
	if err != nil {
		return directoryMapRefused(format, domain.ConfigUnusable, err.Error())
	}
	// A selected tree replaces the declared ones rather than adding to them,
	// and must actually be a directory: `--directory` naming nothing is a
	// mistake in the invocation, and reporting it as a clean walk of zero
	// directories would be the wrong answer to the wrong question.
	var selected []string
	if directory != nil {
		if tree.PassesThroughLink(*directory) {
			return directoryMapRefused(format, domain.PathEscapesRoot,
				*directory+" passes through a symbolic link, which RHINO never follows")
		}
		if !tree.IsDirectory(*directory) {
			return directoryMapRefused(format, domain.FileMissing,
				*directory+" is not a directory in this repository")
		}
		selected = []string{*directory}
	} else {
		selected = trees
	}
	report := NewReport("directory-map", "directory")
	for _, path := range selected {
		for _, subdirectory := range directoryMapDirectories(tree.Files(), skipped, path) {
			report.Inspected++
			if reason := directoryMapInspect(tree, subdirectory, report); reason != nil {
				return directoryMapRefused(format, domain.FileUnreadable, reason.Error())
			}
		}
	}
	return report.Render(format)
}

// decodeDirectoryMap projects the declared map trees. Unknown keys refuse:
// a policy the tool silently ignored would be a check that never ran. An
// absent tree list holds nothing; a present one must hold mappings.
func decodeDirectoryMap(mapping map[string]any) ([]string, error) {
	for key := range mapping {
		if key != "trees" {
			return nil, fmt.Errorf("policies.governance.directory-map names unknown key `%s`", key)
		}
	}
	raw, present := mapping["trees"]
	if !present || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("directory-map.trees names no trees")
	}
	trees := make([]string, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("directory-map.trees names no trees")
		}
		for key := range entry {
			if key != "path" {
				return nil, fmt.Errorf("directory-map.trees name unknown key `%s`", key)
			}
		}
		path, _ := entry["path"].(string)
		trees = append(trees, path)
	}
	return trees, nil
}

// directoryMapDirectories lists every directory at or under a tree, in path
// order. Derived from the exclusion-filtered file list rather than from a
// directory listing, because the read-only tree exposes files and a directory
// holding nothing is not a directory a README can be missing from.
func directoryMapDirectories(files []string, excluded []string, root string) []string {
	prefix := strings.TrimSuffix(root, "/") + "/"
	slashes := strings.Count(prefix, "/")
	seen := map[string]bool{}
	var out []string
	for _, path := range files {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		if scanExcluded(path, excluded) {
			continue
		}
		segments := strings.Split(path, "/")
		segments = segments[:len(segments)-1]
		for len(segments) >= slashes {
			joined := strings.Join(segments, "/")
			if !seen[joined] {
				seen[joined] = true
				out = append(out, joined)
			}
			segments = segments[:len(segments)-1]
		}
	}
	sortStrings(out)
	return out
}

// directoryMapEntry is one map target with its one-based line number.
type directoryMapEntry struct {
	line   int
	target string
}

// directoryMapInspect checks one directory's map, recording findings. It
// answers nil when the directory was inspected, and the refusal reason when
// its README exists but could not be read: a README that exists and cannot be
// opened is not a missing README, and reporting it as one would tell a
// maintainer to write a file that is already there.
func directoryMapInspect(tree Tree, directory string, report *Report) error {
	readme := directory + "/" + directoryMapReadme
	text, readErr := tree.Read(readme)
	if readErr != nil {
		switch readErr.Kind {
		case "missing":
			report.Findings = append(report.Findings, NewFinding("missing-readme", directory,
				"missing README: a mapped directory has to document its own contents"))
			return nil
		case "not-text":
			return fmt.Errorf("%s: holds no text", readme)
		default:
			return fmt.Errorf("%s: %s", readme, readErr.Reason)
		}
	}
	entries, present := directoryMapEntries(text)
	if !present {
		report.Findings = append(report.Findings, NewFinding("missing-directory-map", readme,
			fmt.Sprintf("missing directory map: a mapped README needs a `%s` section", directoryMapSection)))
		return nil
	}
	// Every entry resolves first, so an unusable one is reported as itself
	// rather than as a sibling that went missing.
	covered := map[string]bool{}
	for _, entry := range entries {
		if sibling, ok := directoryMapResolve(tree, directory, entry.target); ok {
			covered[sibling] = true
			continue
		}
		finding := NewFinding("invalid-map-entry", readme,
			fmt.Sprintf("invalid map entry `%s`: an entry has to name a direct sibling by a relative path that exists",
				strings.ReplaceAll(entry.target, "\x00", "\\0")))
		at := entry.line
		finding.Line = &at
		report.Findings = append(report.Findings, finding)
	}
	for _, sibling := range tree.Children(directory) {
		if sibling == readme || covered[sibling] {
			continue
		}
		report.Findings = append(report.Findings, NewFinding("missing-map-entry", readme,
			fmt.Sprintf("missing map entry for `%s`: every direct sibling appears exactly once", sibling)))
	}
	return nil
}

// directoryMapEntries reads a README's map section, or reports it absent. A
// section with no entries is present but empty: a directory with nothing
// beside its README has an empty map, and that is a complete map rather than
// a missing one.
func directoryMapEntries(text string) ([]directoryMapEntry, bool) {
	var entries []directoryMapEntry
	inside := false
	for index, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "##") {
			if inside {
				break
			}
			inside = trimmed == directoryMapSection
			continue
		}
		if strings.HasPrefix(trimmed, "# ") && inside {
			break
		}
		if !inside {
			continue
		}
		if target, ok := directoryMapListEntry(trimmed); ok {
			entries = append(entries, directoryMapEntry{line: index + 1, target: target})
		}
	}
	if !inside {
		return nil, false
	}
	return entries, true
}

// directoryMapListEntry reads `- [Text](target)`, the one form a map entry takes.
func directoryMapListEntry(line string) (string, bool) {
	rest, ok := strings.CutPrefix(line, "- ")
	if !ok {
		rest, ok = strings.CutPrefix(line, "* ")
		if !ok {
			return "", false
		}
	}
	_, after, ok := strings.Cut(rest, "](")
	if !ok {
		return "", false
	}
	target, _, ok := strings.Cut(after, ")")
	if !ok {
		return "", false
	}
	return strings.TrimSpace(target), true
}

// directoryMapResolve answers the sibling an entry covers, or false when the
// entry is unusable. A map entry is narrower than an ordinary link: it may not
// be absolute, carry a scheme, leave its own directory, or point at something
// that is not there. A map is a claim about *this* directory, and an entry
// that addresses anywhere else is a claim about something the reader did not
// ask about.
func directoryMapResolve(tree Tree, directory, target string) (string, bool) {
	if target == "" || strings.ContainsRune(target, '\x00') || strings.HasPrefix(target, "/") {
		return "", false
	}
	if scheme, _, found := strings.Cut(target, ":"); found && directoryMapIsScheme(scheme) {
		return "", false
	}
	withoutFragment, _, _ := strings.Cut(target, "#")
	path, _, _ := strings.Cut(withoutFragment, "?")
	var segments []string
	for _, segment := range strings.Split(path, "/") {
		switch segment {
		case "", ".":
		case "..":
			return "", false
		default:
			segments = append(segments, segment)
		}
	}
	if len(segments) == 0 {
		return "", false
	}
	first, rest := segments[0], segments[1:]
	// An entry may address a sibling directly, or through that directory's own
	// README -- which is how a map links a subdirectory to something readable.
	sibling := directory + "/" + first
	var addressed string
	switch {
	case len(rest) == 0:
		addressed = sibling
	case len(rest) == 1 && rest[0] == directoryMapReadme:
		addressed = sibling + "/" + directoryMapReadme
	default:
		return "", false
	}
	if !tree.Exists(addressed) {
		return "", false
	}
	return sibling, true
}

// directoryMapIsScheme answers whether a prefix is a URI scheme: nonempty,
// opening with an ASCII letter, then letters, digits, or `+`, `.`, `-`.
func directoryMapIsScheme(prefix string) bool {
	if prefix == "" {
		return false
	}
	for index, character := range prefix {
		switch {
		case index == 0:
			if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z') {
				return false
			}
		case character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '+' || character == '.' || character == '-':
		default:
			return false
		}
	}
	return true
}

func directoryMapRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("directory-map", "directory")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
