// Internal-link validation: local targets resolve inside the repository.
package application

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// linkInline matches `[text](destination)`, with an optional title the
// destination stops before. The opening bracket is part of the match so the
// character before it can be inspected: `![alt](src)` is an image, and an
// image is not a document link.
var linkInline = regexp.MustCompile(`\[[^\]]*\]\(\s*<?([^)<>\s]*)>?(?:\s+"[^"]*")?\s*\)`)

// linkDefinition matches `[label]: destination`, the reference-style
// definition. The definition is what carries the target, so it is what gets
// checked.
var linkDefinition = regexp.MustCompile(`^\s{0,3}\[[^\]]+\]:\s*<?([^>\s]+)>?`)

// linkFault is why a local target is not reachable.
type linkFault int

const (
	linkMalformed linkFault = iota
	linkOutside
	linkMissing
)

func (fault linkFault) kind() string {
	switch fault {
	case linkMalformed:
		return "internal-link-malformed"
	case linkOutside:
		return "internal-link-outside-repository"
	default:
		return "internal-link-missing"
	}
}

func (fault linkFault) message(destination string) string {
	readable := strings.ReplaceAll(destination, "\x00", "\\0")
	switch fault {
	case linkMalformed:
		return "`" + readable + "` is not a usable path"
	case linkOutside:
		return "`" + readable + "` resolves outside the repository"
	default:
		return "`" + readable + "` does not exist"
	}
}

// linkDestination is one link target with the line it appears on.
type linkDestination struct {
	line int
	raw  string
}

func internalLinkValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("markdown", "internal-link")
	if !ok {
		report := NewReport("internal-link", "link")
		report.Refusal = strptr("policies.markdown.internal-link is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	excluded, refusal := decodeInternalLink(mapping)
	if refusal != nil {
		return internalLinkRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	for _, pattern := range excluded {
		if _, err := Match(pattern, "probe.md"); err != nil {
			return internalLinkRefused(format, domain.ConfigUnusable,
				"md-internal-link.exclude-sources: "+err.Error())
		}
	}
	// The shared scan exclusions select the corpus before any read: an
	// unreadable file inside an excluded directory never refuses the run.
	// A validator-excluded source stays readable as a link target below.
	shared, err := document.ScanExcludeDirectories()
	if err != nil {
		return internalLinkRefused(format, domain.ConfigUnusable, err.Error())
	}
	report := NewReport("internal-link", "link")
	for _, path := range scanMarkdown(tree.Files(), shared) {
		text, readErr := tree.Read(path)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				continue
			case "not-text":
				return internalLinkRefused(format, domain.FileUnreadable, path+": holds no text")
			default:
				return internalLinkRefused(format, domain.FileUnreadable, path+": "+readErr.Reason)
			}
		}
		// An excluded source is read but not scanned for links. It stays in
		// the corpus as a perfectly valid target for links elsewhere.
		if matchesAny(excluded, path) {
			continue
		}
		for _, destination := range linkDestinations(text) {
			target, ok := linkLocal(destination.raw)
			if !ok {
				continue
			}
			report.Inspected++
			// Inspection continues past a fault: a malformed target in one
			// file must not hide a missing one in the next.
			if fault, failed := resolveLink(tree, path, target); failed {
				finding := NewFinding(fault.kind(), path, fault.message(destination.raw))
				at := destination.line
				finding.Line = &at
				report.Findings = append(report.Findings, finding)
			}
		}
	}
	return report.Render(format)
}

// decodeInternalLink projects the declared exclusion list. The key is
// required in shape but not in presence: a surface that excludes nothing and
// one whose exclusions were forgotten have to look different, except here the
// oracle defaults an absent list to empty, so this port does too.
func decodeInternalLink(mapping map[string]any) ([]string, error) {
	for key := range mapping {
		if key != "exclude-sources" {
			return nil, fmt.Errorf("policies.markdown.internal-link names unknown key `%s`", key)
		}
	}
	raw, present := mapping["exclude-sources"]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("md-internal-link.exclude-sources names no glob")
	}
	var excluded []string
	for _, item := range items {
		pattern, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("md-internal-link.exclude-sources names no glob")
		}
		excluded = append(excluded, pattern)
	}
	return excluded, nil
}

// linkDestinations lists every link destination in a document with its line.
func linkDestinations(text string) []linkDestination {
	var found []linkDestination
	for _, line := range proseLines(text) {
		// Link syntax inside backticks is a quotation of a link, the way a
		// fenced block is: documentation explaining a convention writes
		// `[name](name)` and means the characters, not the destination.
		content := withoutCodeSpans(line.text)
		for _, match := range linkInline.FindAllStringSubmatchIndex(content, -1) {
			if match[0] > 0 && content[match[0]-1] == '!' {
				continue
			}
			found = append(found, linkDestination{line: line.number, raw: content[match[2]:match[3]]})
		}
		if match := linkDefinition.FindStringSubmatch(content); match != nil {
			found = append(found, linkDestination{line: line.number, raw: match[1]})
		}
	}
	return found
}

// linkLocal returns the repository-relative part of a destination, or false
// when the destination is not this repository's to resolve. A same-document
// fragment and a destination with a scheme belong to someone else's server,
// so both are skipped rather than reported.
func linkLocal(destination string) (string, bool) {
	if destination == "" || strings.HasPrefix(destination, "#") || hasLinkScheme(destination) {
		return "", false
	}
	withoutFragment := strings.SplitN(destination, "#", 2)[0]
	path := strings.SplitN(withoutFragment, "?", 2)[0]
	return path, true
}

func hasLinkScheme(destination string) bool {
	position := strings.IndexByte(destination, ':')
	if position < 0 {
		return false
	}
	scheme := destination[:position]
	if scheme == "" || !isAlpha(scheme[0]) {
		return false
	}
	for i := 0; i < len(scheme); i++ {
		character := scheme[i]
		if !isAlpha(character) && !(character >= '0' && character <= '9') && character != '+' && character != '.' && character != '-' {
			return false
		}
	}
	return true
}

func isAlpha(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
}

func resolveLink(tree Tree, source, target string) (linkFault, bool) {
	if strings.ContainsRune(target, 0) {
		return linkMalformed, true
	}
	if strings.HasPrefix(target, "/") {
		// An absolute path is not repository-relative and names a location
		// the repository does not own, wherever it happens to resolve.
		return linkOutside, true
	}
	if target == "" {
		return 0, false
	}
	segments := strings.Split(source, "/")
	segments = segments[:len(segments)-1]
	for _, segment := range strings.Split(target, "/") {
		switch segment {
		case "", ".":
		case "..":
			// Popping past the root is leaving the repository.
			if len(segments) == 0 {
				return linkOutside, true
			}
			segments = segments[:len(segments)-1]
		default:
			segments = append(segments, segment)
		}
	}
	resolved := strings.Join(segments, "/")
	// A target behind a filesystem link is never followed. Where the link
	// leads is not this repository's to answer for, so the target is
	// reported as outside it rather than as missing.
	if tree.PassesThroughLink(resolved) {
		return linkOutside, true
	}
	if resolved == "" || tree.Exists(resolved) {
		return 0, false
	}
	return linkMissing, true
}

// withoutCodeSpans returns a line with its inline code spans removed. A span
// between backticks is shown rather than acted on, the same way a fenced
// block is. The run length has to match, so a line demonstrating backticks
// inside backticks is not cut short, and a run with no partner is a literal
// backtick rather than an opener.
func withoutCodeSpans(line string) string {
	var out strings.Builder
	rest := line
	for {
		start := strings.IndexByte(rest, '`')
		if start < 0 {
			out.WriteString(rest)
			return out.String()
		}
		out.WriteString(rest[:start])
		opened := rest[start:]
		run := 0
		for run < len(opened) && opened[run] == '`' {
			run++
		}
		delimiter := opened[:run]
		body := opened[run:]
		end := strings.Index(body, delimiter)
		if end < 0 {
			out.WriteString(opened)
			return out.String()
		}
		rest = body[end+run:]
	}
}

func internalLinkRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("internal-link", "link")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
