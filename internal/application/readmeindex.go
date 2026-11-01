// README index validation: every declared tree carries linked indexes.
//
// Three completeness levels, all repository-owned: an index that names its
// directory, one that also links every direct child, and one that requires
// an index in every directory under the tree, each linking its direct
// Markdown files and indexed subdirectories, with every relative link
// resolving. Annotations are exact text a named README must carry.
package application

import (
	"fmt"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// readmeChildren is which indexes of a declared tree must link their direct
// children. The complete child set is an observable tree fact, not a second
// manually maintained list.
type readmeChildren int

const (
	readmeChildrenNone readmeChildren = iota
	readmeChildrenDeclared
	readmeChildrenEvery
)

// readmeAnnotation is exact text a named README must carry.
type readmeAnnotation struct {
	path string
	text string
}

// readmeTree is one declared index tree and its repository-owned facts.
type readmeTree struct {
	path        string
	children    readmeChildren
	annotations []readmeAnnotation
	exclusions  []string
}

func readmeValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("markdown", "readme-index")
	if !ok {
		report := NewReport("readme-index", "directory")
		report.Refusal = strptr("policies.markdown.readme-index is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	trees, refusal := decodeReadmeIndex(mapping)
	if refusal != nil {
		return readmeRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	skipped, err := document.ScanExcludeDirectories()
	if err != nil {
		return readmeRefused(format, domain.ConfigUnusable, err.Error())
	}
	report := NewReport("readme-index", "directory")
	if len(trees) > 0 {
		// One walk answers every shape query below; see shapedTree.
		tree = newShapedTree(tree)
	}
	for _, declared := range trees {
		if !isExactPath(declared.path) {
			return readmeRefused(format, domain.ConfigUnusable,
				"README index tree paths and exclusions must be exact paths relative to the repository")
		}
		for _, exclusion := range declared.exclusions {
			if !isExactPath(exclusion) {
				return readmeRefused(format, domain.ConfigUnusable,
					"README index tree paths and exclusions must be exact paths relative to the repository")
			}
		}
		var indexed bool
		if declared.children == readmeChildrenEvery {
			complete, readErr := readmeComplete(declared, skipped, tree, report)
			if readErr != nil {
				return readmeRefused(format, domain.FileUnreadable, readErr.Error())
			}
			indexed = complete
		} else {
			complete, readErr := readmeRoot(declared, skipped, tree, report)
			if readErr != nil {
				return readmeRefused(format, domain.FileUnreadable, readErr.Error())
			}
			indexed = complete
		}
		if !indexed {
			continue
		}
		for _, annotation := range declared.annotations {
			joined := readmeChildPath(declared.path, annotation.path)
			if joined == "" {
				return readmeRefused(format, domain.ConfigUnusable,
					"README annotations must name exact paths relative to their declared tree")
			}
			text, present, readErr := readmeRead(tree, joined)
			if readErr != nil {
				return readmeRefused(format, domain.FileUnreadable, readErr.Error())
			}
			if !present {
				report.Findings = append(report.Findings,
					NewFinding("missing-readme-annotation", joined, "the configured annotation file is absent"))
				continue
			}
			report.Scanned = append(report.Scanned, joined)
			report.Inspected++
			if !strings.Contains(text, annotation.text) {
				report.Findings = append(report.Findings,
					NewFinding("missing-readme-annotation", joined, "the configured README annotation is absent"))
			}
		}
	}
	return report.Render(format)
}

// decodeReadmeIndex projects the declared trees. The trees are the
// repository's because RHINO ships no directory; the index facts are not, and
// an unknown key is refused rather than read as a fifth.
func decodeReadmeIndex(mapping map[string]any) ([]readmeTree, error) {
	for key := range mapping {
		if key != "trees" {
			return nil, fmt.Errorf("policies.markdown.readme-index names unknown key `%s`", key)
		}
	}
	raw, present := mapping["trees"]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("readme-index.trees names no trees")
	}
	trees := make([]readmeTree, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("readme-index.trees names no trees")
		}
		for key := range entry {
			if key != "path" && key != "require-direct-children" && key != "annotations" && key != "exclusions" {
				return nil, fmt.Errorf("readme-index.trees name unknown key `%s`", key)
			}
		}
		declared := readmeTree{path: entryString(entry, "path")}
		switch value := entry["require-direct-children"]; value {
		case nil:
		case true:
			declared.children = readmeChildrenDeclared
		case false:
		case "every-directory":
			declared.children = readmeChildrenEvery
		default:
			return nil, fmt.Errorf("readme-index.require-direct-children must be a boolean or `every-directory`")
		}
		annotations, err := decodeReadmeAnnotations(entry)
		if err != nil {
			return nil, err
		}
		declared.annotations = annotations
		exclusions, err := decodeReadmeExclusions(entry)
		if err != nil {
			return nil, err
		}
		declared.exclusions = exclusions
		trees = append(trees, declared)
	}
	return trees, nil
}

func entryString(entry map[string]any, key string) string {
	value, _ := entry[key].(string)
	return value
}

func decodeReadmeAnnotations(entry map[string]any) ([]readmeAnnotation, error) {
	raw, present := entry["annotations"]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("readme-index.annotations name no annotation")
	}
	annotations := make([]readmeAnnotation, 0, len(items))
	for _, item := range items {
		member, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("readme-index.annotations name no annotation")
		}
		for key := range member {
			if key != "path" && key != "text" {
				return nil, fmt.Errorf("readme-index.annotations name unknown key `%s`", key)
			}
		}
		annotation := readmeAnnotation{path: entryString(member, "path")}
		text, ok := member["text"].(string)
		if !ok {
			return nil, fmt.Errorf("readme-index.annotations name no text")
		}
		annotation.text = text
		annotations = append(annotations, annotation)
	}
	return annotations, nil
}

func decodeReadmeExclusions(entry map[string]any) ([]string, error) {
	raw, present := entry["exclusions"]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("readme-index.exclusions names no path")
	}
	exclusions := make([]string, 0, len(items))
	for _, item := range items {
		path, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("readme-index.exclusions names no path")
		}
		exclusions = append(exclusions, path)
	}
	return exclusions, nil
}

// readmeRoot checks the declared directory's own index, with its direct
// children when declared completeness requires them. It answers whether that
// index exists, because an absent one leaves no annotation to check.
func readmeRoot(declared readmeTree, skipped []string, tree Tree, report *Report) (bool, error) {
	index := declared.path + "/README.md"
	contents, present, readErr := readmeRead(tree, index)
	if readErr != nil {
		return false, readErr
	}
	if !present {
		report.Inspected++
		report.Findings = append(report.Findings,
			NewFinding("missing-readme-index", index, "the declared directory has no README index"))
		return false, nil
	}
	report.Scanned = append(report.Scanned, index)
	report.Inspected++

	if declared.children == readmeChildrenDeclared {
		linked := readmeTargets(index, contents)
		for _, child := range tree.Children(declared.path) {
			if child == index || readmeExcluded(child, declared.path, declared.exclusions) || readmeSkipped(child, skipped, tree) {
				continue
			}
			if !linked[child] {
				finding := NewFinding("missing-readme-index-child", index, "the declared README index omits a direct child")
				finding.Details = append(finding.Details, TextDetail("child", child))
				report.Findings = append(report.Findings, finding)
			}
		}
	}
	return true, nil
}

// readmeComplete checks the declared directory and every directory under it:
// each carries a README.md linking its direct Markdown files and each direct
// subdirectory holding an index, and every relative link resolves. It answers
// whether the declared directory's own index exists, as readmeRoot does.
func readmeComplete(declared readmeTree, skipped []string, tree Tree, report *Report) (bool, error) {
	indexed := false
	pending := []string{declared.path}
	for len(pending) > 0 {
		directory := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		index := directory + "/README.md"
		var children []string
		for _, child := range tree.Children(directory) {
			if readmeExcluded(child, declared.path, declared.exclusions) || readmeSkipped(child, skipped, tree) {
				continue
			}
			children = append(children, child)
			if tree.IsDirectory(child) {
				pending = append(pending, child)
			}
		}
		contents, present, readErr := readmeRead(tree, index)
		if readErr != nil {
			return false, readErr
		}
		if !present {
			report.Inspected++
			report.Findings = append(report.Findings,
				NewFinding("missing-readme-index", index, "a directory in the declared tree has no README index"))
			continue
		}
		report.Scanned = append(report.Scanned, index)
		report.Inspected++
		if directory == declared.path {
			indexed = true
		}
		linked := readmeTargets(index, contents)
		for _, target := range sortedKeys(linked) {
			if !readmePresent(tree, target) {
				finding := NewFinding("missing-readme-index-target", index, "a README index link resolves to nothing in the repository")
				finding.Details = append(finding.Details, TextDetail("target", target))
				report.Findings = append(report.Findings, finding)
			}
		}
		for _, child := range children {
			if child == index {
				continue
			}
			var required, covered bool
			if tree.IsDirectory(child) {
				required = readmePresent(tree, child+"/README.md")
				covered = linked[child] || linked[child+"/README.md"] || readmePresent(tree, child+".md")
			} else {
				required = strings.HasSuffix(strings.ToLower(child), ".md")
				covered = linked[child]
			}
			if required && !covered {
				finding := NewFinding("missing-readme-index-child", index, "a README index in the declared tree omits a direct child")
				finding.Details = append(finding.Details, TextDetail("child", child))
				report.Findings = append(report.Findings, finding)
			}
		}
	}
	return indexed, nil
}

// readmeRead projects the portable reader: absent is not an error, a file
// that cannot be opened fails the run closed.
func readmeRead(tree Tree, path string) (string, bool, error) {
	text, readErr := tree.Read(path)
	if readErr != nil {
		switch readErr.Kind {
		case "missing":
			return "", false, nil
		case "not-text":
			return "", false, fmt.Errorf("%s: holds no text", path)
		default:
			return "", false, fmt.Errorf("%s: %s", path, readErr.Reason)
		}
	}
	return text, true, nil
}

// readmePresent answers whether the repository holds a path as a file,
// readable or not, or as a directory. A file that cannot be opened might
// have held anything, so it counts as present rather than as missing.
func readmePresent(tree Tree, path string) bool {
	if _, readErr := tree.Read(path); readErr == nil {
		return true
	} else if readErr.Kind != "missing" {
		return true
	}
	return tree.IsDirectory(path)
}

// readmeSkipped answers whether scan exclusion removes a child: a directory
// it names, or anything beneath one.
func readmeSkipped(child string, skipped []string, tree Tree) bool {
	probe := child
	if tree.IsDirectory(child) {
		probe = child + "/README.md"
	}
	return scanExcluded(probe, skipped)
}

func readmeUnder(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+"/")
}

func readmeExcluded(path, root string, exclusions []string) bool {
	relative, ok := strings.CutPrefix(path, root+"/")
	if !ok {
		return false
	}
	for _, exclusion := range exclusions {
		if readmeUnder(relative, exclusion) {
			return true
		}
	}
	return false
}

func readmeChildPath(root, relative string) string {
	if !isExactPath(relative) {
		return ""
	}
	return root + "/" + relative
}

// isExactPath answers whether a path is exact and repository-relative: no
// emptiness, no leading slash, no dot segments, no separators smuggled in a
// segment, and no glob metacharacters.
func isExactPath(path string) bool {
	if strings.TrimSpace(path) == "" || strings.HasPrefix(path, "/") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
		if strings.ContainsRune(part, '\\') || strings.ContainsAny(part, "*?[]{}") {
			return false
		}
	}
	return true
}

// readmeTargets resolves every Markdown link target in an index against the
// index's own directory. Absolute links, schemes, and climbs past the root
// are not repository targets and are skipped rather than reported.
func readmeTargets(source, text string) map[string]bool {
	linked := map[string]bool{}
	tails := strings.Split(text, "](")[1:]
	for _, tail := range tails {
		end := strings.IndexByte(tail, ')')
		if end < 0 {
			continue
		}
		target := tail[:end]
		if fields := strings.Fields(target); len(fields) > 0 {
			target = fields[0]
		} else {
			continue
		}
		if index := strings.IndexAny(target, "?#"); index >= 0 {
			target = target[:index]
		}
		if resolved, ok := readmeResolve(source, target); ok {
			linked[resolved] = true
		}
	}
	return linked
}

func readmeResolve(source, target string) (string, bool) {
	if target == "" || strings.HasPrefix(target, "/") || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
		return "", false
	}
	var parts []string
	if index := strings.LastIndexByte(source, '/'); index >= 0 {
		for _, part := range strings.Split(source[:index], "/") {
			if part != "" {
				parts = append(parts, part)
			}
		}
	}
	for _, part := range strings.Split(target, "/") {
		switch part {
		case "", ".":
		case "..":
			if len(parts) == 0 {
				return "", false
			}
			parts = parts[:len(parts)-1]
		default:
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "/"), true
}

// sortedKeys lists the keys of a target set in ascending order.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sortStrings(out)
	return out
}

// ChildrenOf derives direct children from the file list: the files directly
// inside the directory and the first segment below it for anything deeper.
// Sorted, so two runs produce byte-identical output. Exported because the
// disk adapter answers the Tree port from the same file list.
func ChildrenOf(files []string, directory string) []string {
	prefix := strings.Trim(directory, "/")
	if prefix == "" || prefix == "." {
		prefix = ""
	} else {
		prefix += "/"
	}
	seen := map[string]bool{}
	var out []string
	for _, path := range files {
		rest, ok := strings.CutPrefix(path, prefix)
		if !ok || rest == "" {
			continue
		}
		child := path
		if index := strings.IndexByte(rest, '/'); index >= 0 {
			child = prefix + rest[:index]
		}
		if !seen[child] {
			seen[child] = true
			out = append(out, child)
		}
	}
	sortStrings(out)
	return out
}

// shapedTree answers the shape queries of a Tree from one file list read once.
// The disk adapter answers Children, IsDirectory, and Exists by walking the
// whole repository, so a validator that asks per directory and per link would
// walk it thousands of times. The answers equal ChildrenOf and IsDirectoryOf
// over the same list.
type shapedTree struct {
	Tree
	files       []string
	directories map[string]bool
	children    map[string][]string
}

// newShapedTree reads the file list once and derives every directory and its
// sorted direct children in one pass.
func newShapedTree(tree Tree) *shapedTree {
	files := tree.Files()
	shaped := &shapedTree{
		Tree:        tree,
		files:       files,
		directories: map[string]bool{},
		children:    map[string][]string{},
	}
	seen := map[string]bool{}
	for _, path := range files {
		// parent is the index of the slash ending the directory being listed,
		// or -1 for the root; the child is the next segment below it.
		for parent := -1; ; {
			directory := ""
			if parent >= 0 {
				directory = path[:parent]
				shaped.directories[directory] = true
			}
			rest := path[parent+1:]
			if rest == "" {
				break
			}
			end := len(path)
			if slash := strings.IndexByte(rest, '/'); slash >= 0 {
				end = parent + 1 + slash
			}
			if child := path[:end]; !seen[child] {
				seen[child] = true
				shaped.children[directory] = append(shaped.children[directory], child)
			}
			if end == len(path) {
				break
			}
			parent = end
		}
	}
	for _, listed := range shaped.children {
		sortStrings(listed)
	}
	return shaped
}

func (s *shapedTree) Files() []string { return s.files }

// Children answers as ChildrenOf does over the same file list.
func (s *shapedTree) Children(directory string) []string {
	return append([]string(nil), s.children[shapeKey(directory)]...)
}

// IsDirectory answers as IsDirectoryOf does over the same file list.
func (s *shapedTree) IsDirectory(path string) bool {
	key := shapeKey(path)
	if key == "" {
		return len(s.files) > 0
	}
	return s.directories[key]
}

// shapeKey is the directory spelling ChildrenOf and IsDirectoryOf both reduce
// a path to: no outer slashes, and the root as the empty string.
func shapeKey(path string) string {
	key := strings.Trim(path, "/")
	if key == "." {
		return ""
	}
	return key
}

// IsDirectoryOf answers directory membership from the file list. Exported
// with ChildrenOf for the disk adapter.
func IsDirectoryOf(files []string, path string) bool {
	prefix := strings.Trim(path, "/")
	if prefix == "" || prefix == "." {
		return len(files) > 0
	}
	prefix += "/"
	for _, file := range files {
		if strings.HasPrefix(file, prefix) {
			return true
		}
	}
	return false
}

func readmeRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("readme-index", "directory")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
