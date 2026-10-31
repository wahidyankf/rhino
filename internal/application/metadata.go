// Metadata validation: path-selected schemas over handwritten front matter.
//
// Four schemas -- governance, workflow, skill, agent -- selected by the
// artifact's path. The front matter is read here rather than decoded: a YAML
// decoder discards one of two duplicate keys, forgets which line a key was on,
// and cannot tell a folded scalar from a plain one, and all three are things
// this contract has a rule about.
package application

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// The portable capability vocabulary, in its canonical order. Order is part of
// the vocabulary rather than a formatting preference: a list that may be
// written in any order is a list two repositories will write two ways, and
// every diff between them is then about nothing.
var metadataCapabilities = []string{
	"repository-read",
	"repository-write",
	"shell",
	"network",
	"subagent",
}

// The bounds the shared value rules put on the two routing fields.
const (
	metadataDescriptionMin   = 20
	metadataDescriptionMax   = 300
	metadataTriggerMin       = 20
	metadataTriggerMax       = 240
	metadataTriggerSentences = 3
)

// metadataSchema is one of the four canonical artifact families.
type metadataSchema string

const (
	schemaGovernance metadataSchema = "governance"
	schemaWorkflow   metadataSchema = "workflow"
	schemaSkill      metadataSchema = "skill"
	schemaAgent      metadataSchema = "agent"
)

// metadataSurface is one declared surface: a glob plus the schema it enforces.
type metadataSurface struct {
	glob   string
	schema metadataSchema
}

// schemaKeys returns the family's keys, required before optional.
func schemaKeys(kind metadataSchema) (required, optional []string) {
	switch kind {
	case schemaGovernance:
		return []string{"description", "when_to_use"}, nil
	case schemaWorkflow:
		return []string{"name", "description", "when_to_use"}, nil
	case schemaSkill:
		return []string{"name", "description", "when_to_use"}, []string{"compatibility"}
	default:
		return []string{"name", "description", "when_to_use", "tier", "capabilities"},
			[]string{"skills", "constraints", "dispatches"}
	}
}

// schemaPosition reports the canonical position of a key, or false when the
// schema does not declare it.
func schemaPosition(kind metadataSchema, key string) (int, bool) {
	required, optional := schemaKeys(kind)
	for i, known := range required {
		if known == key {
			return i, true
		}
	}
	for i, known := range optional {
		if known == key {
			return len(required) + i, true
		}
	}
	return 0, false
}

func metadataValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("markdown", "metadata")
	if !ok {
		report := NewReport("metadata", "file")
		report.Refusal = strptr("policies.markdown.metadata is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	surfaces, refusal := decodeMetadata(mapping)
	if refusal != nil {
		return metadataRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	for _, surface := range surfaces {
		if _, err := Match(surface.glob, "probe.md"); err != nil {
			return metadataRefused(format, domain.ConfigUnusable,
				"metadata.surfaces: "+err.Error())
		}
	}
	shared, err := document.ScanExcludeDirectories()
	if err != nil {
		return metadataRefused(format, domain.ConfigUnusable, err.Error())
	}
	report := NewReport("metadata", "file")
	for _, path := range scanMarkdown(tree.Files(), shared) {
		// The last matching surface governs: a later declaration is the more
		// specific intent, the same rule every surface list states.
		governed := -1
		for i, surface := range surfaces {
			ok, err := Match(surface.glob, path)
			if err != nil {
				return metadataRefused(format, domain.ConfigUnusable,
					"metadata.surfaces: "+err.Error())
			}
			if ok {
				governed = i
			}
		}
		if governed < 0 {
			continue
		}
		text, readErr := tree.Read(path)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				continue
			case "not-text":
				return metadataRefused(format, domain.FileUnreadable, path+": holds no text")
			default:
				return metadataRefused(format, domain.FileUnreadable, path+": "+readErr.Reason)
			}
		}
		report.Scanned = append(report.Scanned, path)
		report.Inspected++
		report.Findings = append(report.Findings, inspectMetadata(path, text, surfaces[governed].schema)...)
	}
	return report.Render(format)
}

// decodeMetadata projects the declared surfaces. The mapping is the
// repository's because RHINO ships no path; the four schemas are not, and an
// unknown one is refused rather than read as a fifth.
func decodeMetadata(mapping map[string]any) ([]metadataSurface, error) {
	for key := range mapping {
		if key != "surfaces" {
			return nil, fmt.Errorf("policies.markdown.metadata names unknown key `%s`", key)
		}
	}
	rawSurfaces, ok := mapping["surfaces"].([]any)
	if !ok {
		return nil, fmt.Errorf("metadata.surfaces names no surfaces")
	}
	surfaces := make([]metadataSurface, 0, len(rawSurfaces))
	for _, item := range rawSurfaces {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("metadata.surfaces names no surface")
		}
		for key := range entry {
			if key != "glob" && key != "schema" {
				return nil, fmt.Errorf("metadata.surfaces names unknown key `%s`", key)
			}
		}
		glob, _ := entry["glob"].(string)
		if strings.TrimSpace(glob) == "" {
			return nil, fmt.Errorf("metadata.surfaces names no glob")
		}
		rawSchema, _ := entry["schema"].(string)
		schema := metadataSchema(rawSchema)
		switch schema {
		case schemaGovernance, schemaWorkflow, schemaSkill, schemaAgent:
		default:
			if strings.TrimSpace(rawSchema) == "" {
				return nil, fmt.Errorf("metadata.surfaces names no schema")
			}
			return nil, fmt.Errorf("metadata.surfaces names unknown schema `%s`: expected `governance`, `workflow`, `skill`, or `agent`", rawSchema)
		}
		surfaces = append(surfaces, metadataSurface{glob: glob, schema: schema})
	}
	return surfaces, nil
}

// metaAt builds a structural metadata finding: line and column 1, with the
// field when the finding is about one. Column 1 selects the structural
// diagnostic format the standardization contract fixes.
func metaAt(kind, path string, line int, field, message string) Finding {
	finding := NewFinding(kind, path, message)
	at := line
	column := 1
	finding.Line = &at
	finding.Column = &column
	if field != "" {
		name := field
		finding.Field = &name
	}
	return finding
}

// inspectMetadata applies every rule this contract states to one artifact.
func inspectMetadata(path, text string, kind metadataSchema) []Finding {
	var findings []Finding

	entries, malformed, block, unterminated := readMetadata(text)
	if unterminated {
		return []Finding{metaAt("metadata-frontmatter-unterminated", path, 1, "",
			"the front-matter block opens and never closes, so no key in it can be trusted")}
	}

	for _, line := range malformed {
		findings = append(findings, metaAt("metadata-frontmatter-malformed", path, line, "",
			"declares no key; every line inside the block is a declaration"))
	}

	var seen []*metaEntry
	for i := range entries {
		entry := &entries[i]
		var first *metaEntry
		for _, held := range seen {
			if held.key == entry.key {
				first = held
				break
			}
		}
		if first != nil {
			findings = append(findings, metaAt("metadata-duplicate-key", path, entry.line, entry.key,
				fmt.Sprintf("is declared again, and a decoder would silently keep one of the two (first at line %d)", first.line)))
			continue
		}
		seen = append(seen, entry)
		if _, known := schemaPosition(kind, entry.key); !known {
			findings = append(findings, metaAt("metadata-unknown-key", path, entry.line, entry.key,
				"is not a key this artifact's schema declares; the path already supplies every identity the schema omits"))
		}
	}

	findings = append(findings, metadataOrder(path, seen, kind)...)
	findings = append(findings, metadataMissing(path, block, seen, kind)...)
	findings = append(findings, metadataValues(path, seen, kind)...)

	return findings
}

// metadataOrder reports the first key that appears after a key the schema
// puts later. Reported once rather than for every key after it: a document
// with two keys transposed has one defect, and naming the whole tail would
// bury it.
func metadataOrder(path string, entries []*metaEntry, kind metadataSchema) []Finding {
	highest := 0
	for _, entry := range entries {
		position, known := schemaPosition(kind, entry.key)
		if !known {
			continue
		}
		if position < highest {
			return []Finding{metaAt("metadata-key-order", path, entry.line, entry.key,
				"appears after a key the schema orders later; canonical key order is hard-failing")}
		}
		highest = position
	}
	return nil
}

func metadataMissing(path string, block int, entries []*metaEntry, kind metadataSchema) []Finding {
	required, _ := schemaKeys(kind)
	held := map[string]bool{}
	for _, entry := range entries {
		held[entry.key] = true
	}
	var findings []Finding
	for _, key := range required {
		if !held[key] {
			findings = append(findings, metaAt("metadata-required-key-missing", path, block, key,
				"is required by this artifact's schema and is not declared"))
		}
	}
	return findings
}

// metadataListKeys are the keys whose value is a list. Named rather than
// inferred from what a document happened to write, because "this key holds a
// list" is a fact about the schema: a repository writing one as the other has
// made a mistake the validator has to be able to state.
func metadataIsList(key string) bool {
	switch key {
	case "capabilities", "skills", "constraints", "dispatches":
		return true
	}
	return false
}

func metadataValues(path string, entries []*metaEntry, kind metadataSchema) []Finding {
	var findings []Finding
	var description *string

	for _, entry := range entries {
		// A key with nothing behind it fails once, on the absence. Running
		// the value rules over a value that is not there would report the
		// same defect a second time in the vocabulary of a rule it never
		// reached.
		switch entry.value.kind {
		case metaNull:
			findings = append(findings, metaAt("metadata-null-value", path, entry.line, entry.key,
				"is declared as null, which is a key with no answer rather than an answer"))
			continue
		case metaEmpty:
			findings = append(findings, metaAt("metadata-empty-value", path, entry.line, entry.key,
				"is declared with no value; an omitted key and an empty one have to stay different things"))
			continue
		}

		if metadataIsList(entry.key) {
			findings = append(findings, metadataArray(path, entry)...)
			continue
		}

		if entry.value.kind != metaScalar {
			findings = append(findings, metaAt("metadata-scalar-required", path, entry.line, entry.key,
				"is declared as a list; this key holds one value"))
			continue
		}
		normalized := metadataNormalize(entry.value.text)

		switch entry.key {
		case "description":
			// Plain or folded, and never literal: a literal block keeps the
			// author's line breaks, and a description travels through
			// adapters that render it as one line.
			if entry.value.form == metaLiteral {
				findings = append(findings, metaAt("metadata-description-form", path, entry.line, entry.key,
					"is written as a literal block; a description is a plain or folded scalar"))
			}
			held := normalized
			description = &held
			findings = append(findings, metadataLength(path, entry, &held,
				"metadata-description-length", metadataDescriptionMin, metadataDescriptionMax)...)
		case "when_to_use":
			if entry.value.form != metaFolded {
				findings = append(findings, metaAt("metadata-folded-scalar-required", path, entry.line, entry.key,
					"has to be written as a folded scalar, so a multi-sentence trigger reads the same in every adapter"))
			}
			findings = append(findings, metadataLength(path, entry, &normalized,
				"metadata-when-to-use-length", metadataTriggerMin, metadataTriggerMax)...)
			if sentences := metadataSentences(normalized); sentences > metadataTriggerSentences {
				findings = append(findings, metaAt("metadata-when-to-use-sentences", path, entry.line, entry.key,
					fmt.Sprintf("carries %d sentences; a routing trigger states at most %d", sentences, metadataTriggerSentences)))
			}
			if description != nil && *description == normalized {
				findings = append(findings, metaAt("metadata-routing-not-distinct", path, entry.line, entry.key,
					"repeats the description, so it adds no trigger a harness could route on"))
			}
		case "name":
			if !isPortableName(normalized) {
				findings = append(findings, metaAt("metadata-name-format", path, entry.line, entry.key,
					"is not lowercase alphanumeric words joined by single hyphens"))
			} else if identity, ok := metadataIdentity(path, kind); !ok || identity != normalized {
				findings = append(findings, metaAt("metadata-name-path-mismatch", path, entry.line, entry.key,
					"is not the identity its path already states"))
			}
		case "tier":
			findings = append(findings, metadataTier(path, entry, normalized)...)
			// `compatibility` and every unknown key: the shared rules above are
			// all this contract states about them.
		}
	}

	return findings
}

// metadataArray applies the rules every declared array shares, plus the
// closed vocabulary and canonical order the capability list adds.
func metadataArray(path string, entry *metaEntry) []Finding {
	if entry.value.kind != metaList {
		return []Finding{metaAt("metadata-array-required", path, entry.line, entry.key,
			"is declared as a scalar; this key holds a list")}
	}
	if len(entry.value.items) == 0 {
		return []Finding{metaAt("metadata-array-empty", path, entry.line, entry.key,
			"is declared with no members, which is an absent answer written as a present key")}
	}

	var findings []Finding
	var seen []string
	highest := 0
	capabilities := entry.key == "capabilities"

	for _, item := range entry.value.items {
		duplicate := false
		for _, held := range seen {
			if held == item.text {
				duplicate = true
				break
			}
		}
		if duplicate {
			findings = append(findings, metaAt("metadata-array-duplicate", path, item.line, entry.key,
				fmt.Sprintf("lists `%s` twice, and the second says nothing the first did not", item.text)))
			continue
		}
		seen = append(seen, item.text)

		if !capabilities {
			continue
		}
		position := -1
		for i, known := range metadataCapabilities {
			if known == item.text {
				position = i
				break
			}
		}
		if position < 0 {
			findings = append(findings, metaAt("metadata-capability-unknown", path, item.line, entry.key,
				fmt.Sprintf("lists `%s`, which is not in the portable capability vocabulary; harness tool names are not portable", item.text)))
			continue
		}
		if position < highest {
			findings = append(findings, metaAt("metadata-array-order", path, item.line, entry.key,
				fmt.Sprintf("lists `%s` after a capability the vocabulary orders later", item.text)))
			continue
		}
		highest = position
	}

	return findings
}

var metadataTiers = []string{"ultra", "plan", "execution", "fast"}

// metadataTier applies the tier a portable agent declares.
func metadataTier(path string, entry *metaEntry, declared string) []Finding {
	for _, tier := range metadataTiers {
		if tier == declared {
			return nil
		}
	}
	return []Finding{metaAt("metadata-tier-unknown", path, entry.line, entry.key,
		"is not one of the four portable tiers, which name a normal workload rather than a model")}
}

func metadataLength(path string, entry *metaEntry, normalized *string, kind string, low, high int) []Finding {
	counted := utf8.RuneCountInString(*normalized)
	if low <= counted && counted <= high {
		return nil
	}
	return []Finding{metaAt(kind, path, entry.line, entry.key,
		fmt.Sprintf("is %d characters; this field holds %d through %d", counted, low, high))}
}

// metadataIdentity is the identity a path already states, which a declared
// name has to match.
func metadataIdentity(path string, kind metadataSchema) (string, bool) {
	segments := strings.Split(path, "/")
	if len(segments) == 0 {
		return "", false
	}
	file := segments[len(segments)-1]
	segments = segments[:len(segments)-1]
	stem := file
	if dot := strings.LastIndex(file, "."); dot >= 0 {
		stem = file[:dot]
	}
	switch {
	// A skill is its directory: the file inside it is called SKILL.md in
	// every repository, so the file name says nothing.
	case kind == schemaSkill:
		if len(segments) == 0 {
			return "", false
		}
		return segments[len(segments)-1], true
	// A workflow entrypoint README takes its name from the directory it
	// indexes, for the same reason.
	case kind == schemaWorkflow && strings.EqualFold(file, "README.md"):
		if len(segments) == 0 {
			return "", false
		}
		return segments[len(segments)-1], true
	default:
		return stem, true
	}
}

func isPortableName(value string) bool {
	if value == "" {
		return false
	}
	for _, word := range strings.Split(value, "-") {
		if word == "" {
			return false
		}
		for _, character := range word {
			if !(character >= 'a' && character <= 'z') && !(character >= '0' && character <= '9') {
				return false
			}
		}
	}
	return true
}

// metadataNormalize collapses and trims whitespace, which is what
// "normalized" means for every length and equality rule in this contract.
func metadataNormalize(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func metadataSentences(text string) int {
	count := 0
	for _, part := range strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	}) {
		if strings.TrimSpace(part) != "" {
			count++
		}
	}
	if strings.TrimSpace(text) != "" && count == 0 {
		return 1
	}
	return count
}

// metaForm is the form a scalar value was written in. The form is retained
// because the contract has rules about it: a trigger has to be a folded
// scalar and a description may not be a literal block.
type metaForm int

const (
	metaPlain metaForm = iota
	metaFolded
	metaLiteral
)

type metaValueKind int

const (
	metaScalar metaValueKind = iota
	metaList
	metaEmpty
	metaNull
)

// metaItem is one list member with the line it was written on.
type metaItem struct {
	line int
	text string
}

// metaValue is a declared value, kept in the form it was written in.
type metaValue struct {
	kind  metaValueKind
	text  string
	form  metaForm
	items []metaItem
}

// metaEntry is one declared key, and the line a reader will find it on.
type metaEntry struct {
	line  int
	key   string
	value metaValue
}

// readMetadata reads the declarations inside the leading front-matter block.
// Only a leading block is front matter: a `---` further down is a horizontal
// rule, and reading one as an opener would invent a schema violation in the
// middle of a document. It returns the entries, the lines inside the block
// that declared nothing at all, the line the block opens on, and whether the
// block never closed.
func readMetadata(text string) (entries []metaEntry, malformed []int, block int, unterminated bool) {
	lines := strings.Split(text, "\n")
	open := -1
	for i, line := range lines {
		if strings.TrimRightFunc(line, unicode.IsSpace) == "---" {
			open = i
			break
		}
	}
	if open < 0 {
		return nil, nil, 1, false
	}
	for _, line := range lines[:open] {
		if strings.TrimSpace(line) != "" {
			return nil, nil, 1, false
		}
	}
	close := -1
	for i, line := range lines[open+1:] {
		if strings.TrimRightFunc(line, unicode.IsSpace) == "---" {
			close = i
			break
		}
	}
	if close < 0 {
		return nil, nil, 1, true
	}
	body := lines[open+1 : open+1+close]
	offset := open + 2
	index := 0
	for index < len(body) {
		raw := body[index]
		if strings.TrimSpace(raw) == "" {
			index++
			continue
		}
		// Every continuation belongs to the key above it and was consumed
		// there. One that reaches here is orphaned, and so is a line that
		// declares no key at all -- both are text inside a block whose whole
		// content is supposed to be declarations.
		var key, rest string
		var declared bool
		if !startsWithSpace(raw) && !(len(raw) > 0 && raw[0] == '-') {
			key, rest, declared = strings.Cut(raw, ":")
		}
		if !declared {
			malformed = append(malformed, offset+index)
			index++
			continue
		}
		line := offset + index
		rest = strings.TrimSpace(rest)
		index++

		var value metaValue
		if form, ok := metadataBlockForm(rest); ok {
			collected, next := metadataIndented(body, index)
			index = next
			value = metaValue{kind: metaScalar, text: collected, form: form}
		} else if rest == "" {
			items, next := metadataMembers(body, index, offset)
			if next == index {
				value = metaValue{kind: metaEmpty}
			} else {
				index = next
				value = metaValue{kind: metaList, items: items}
			}
		} else if rest == "null" || rest == "~" {
			value = metaValue{kind: metaNull}
		} else if rest == "[]" {
			value = metaValue{kind: metaList}
		} else if rest == "''" || rest == `""` {
			value = metaValue{kind: metaEmpty}
		} else {
			value = metaValue{kind: metaScalar, text: metadataUnquote(rest), form: metaPlain}
		}

		entries = append(entries, metaEntry{
			line:  line,
			key:   strings.TrimSpace(key),
			value: value,
		})
	}

	return entries, malformed, metadataBlockLine(text), false
}

func startsWithSpace(value string) bool {
	for _, first := range value {
		return unicode.IsSpace(first)
	}
	return false
}

// metadataBlockLine is the line the front-matter block opens on, which is
// where a key that is not there is reported.
func metadataBlockLine(text string) int {
	for i, line := range strings.Split(text, "\n") {
		if strings.TrimRightFunc(line, unicode.IsSpace) == "---" {
			return i + 1
		}
	}
	return 1
}

// metadataBlockForm reports the block scalar a value opens, if it opens one.
func metadataBlockForm(rest string) (metaForm, bool) {
	if strings.HasPrefix(rest, ">") {
		return metaFolded, true
	}
	if strings.HasPrefix(rest, "|") {
		return metaLiteral, true
	}
	return metaPlain, false
}

// metadataIndented collects the indented block beneath a folded or literal
// scalar, joined the way a reader reads it.
func metadataIndented(body []string, from int) (string, int) {
	var collected []string
	index := from
	for index < len(body) {
		line := body[index]
		if !startsWithSpace(line) && strings.TrimSpace(line) != "" {
			break
		}
		collected = append(collected, strings.TrimSpace(line))
		index++
	}
	return strings.TrimSpace(strings.Join(collected, " ")), index
}

// metadataMembers collects the `- ` members beneath a key, with the line each
// was written on.
func metadataMembers(body []string, from, offset int) ([]metaItem, int) {
	var items []metaItem
	index := from
	for index < len(body) {
		trimmed := strings.TrimSpace(body[index])
		if trimmed == "" {
			break
		}
		member, ok := strings.CutPrefix(trimmed, "- ")
		if !ok {
			break
		}
		items = append(items, metaItem{line: offset + index, text: metadataUnquote(strings.TrimSpace(member))})
		index++
	}
	return items, index
}

func metadataUnquote(value string) string {
	if inner, ok := strings.CutPrefix(value, `"`); ok {
		if stripped, ok := strings.CutSuffix(inner, `"`); ok {
			return stripped
		}
		return value
	}
	if inner, ok := strings.CutPrefix(value, "'"); ok {
		if stripped, ok := strings.CutSuffix(inner, "'"); ok {
			return stripped
		}
	}
	return value
}

func metadataRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("metadata", "file")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
