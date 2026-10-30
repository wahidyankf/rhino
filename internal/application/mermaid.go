// Mermaid validation: accessibility and legibility of diagrams.
//
// Two questions about one artefact, answered by one command: a diagram nobody
// can read because the contrast is wrong and a diagram nobody can read
// because the label overflows are the same problem to a reader.
//
// Which diagram syntaxes RHINO can parse is a property of the tool; which
// colours and label lengths are acceptable is a property of the repository,
// and every one of them arrives from configuration.
package application

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// Finding kinds the mermaid validator reports.
const (
	mermaidAccessibilityKind = "mermaid-accessibility"
	mermaidLegibilityKind    = "mermaid-legibility"
	mermaidAuthoringKind     = "diagram-authoring-rule"
)

// mermaidKind is a diagram syntax this build can read well enough to judge.
// A syntax absent from this list is skipped whole, not partially inspected.
type mermaidKind int

const (
	kindFlow mermaidKind = iota
	kindClass
	kindState
	kindEntity
	kindRequirement
	kindBlock
	// kindOther is a syntax this build cannot parse that the repository
	// declared allowed. Only the checks that need no grammar apply to it.
	kindOther
)

func kindOf(declaration string) (mermaidKind, bool) {
	switch declaration {
	case "flowchart", "graph":
		return kindFlow, true
	case "classDiagram", "classDiagram-v2":
		return kindClass, true
	case "stateDiagram", "stateDiagram-v2":
		return kindState, true
	case "erDiagram":
		return kindEntity, true
	case "requirementDiagram":
		return kindRequirement, true
	case "block", "block-beta":
		return kindBlock, true
	default:
		return kindFlow, false
	}
}

// mermaidRules is the repository's declared mermaid policy.
type mermaidRules struct {
	authoring           string
	nodeLimit           int
	edgeLimit           int
	fillColors          []string
	edgeColors          []string
	textColors          []string
	exclude             []string
	requireDefaultClass bool
	// allowedTypes is nil when the repository declared no allowlist. A
	// declared empty list still opts in to judging every type.
	allowedTypes         *[]string
	forbidThemeOverrides bool
	canvasColors         []string
}

// diagram is one fenced diagram, with each line carrying the Markdown line
// it came from.
type diagram struct {
	kind        mermaidKind
	declaration string
	lines       []fencedLine
}

// fencedLine is one content line of a fenced block with its one-based
// position in the file a reader will open.
type fencedLine struct {
	number int
	text   string
}

// fencedBlock is one fenced region with its info string.
type fencedBlock struct {
	info  string
	lines []fencedLine
}

func mermaidValidate(document *config.Document, tree Tree, files []string, stdin *string, format Format) Outcome {
	mapping, ok := document.PolicyGroup("markdown", "mermaid")
	if !ok {
		report := NewReport("mermaid", "diagram")
		report.Refusal = strptr("policies.markdown.mermaid is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	rules, refusal := decodeMermaid(mapping)
	if refusal != nil {
		return mermaidRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	for _, pattern := range rules.exclude {
		if _, err := Match(pattern, "probe.md"); err != nil {
			return mermaidRefused(format, domain.FileUnreadable,
				"md-mermaid.exclude: "+err.Error())
		}
	}
	for _, canvas := range rules.canvasColors {
		if !isSixDigitHex(canvas) {
			return mermaidRefused(format, domain.FileUnreadable,
				fmt.Sprintf("md-mermaid.canvas-colors: `%s` is not a six-digit hex color", canvas))
		}
	}
	selected, refused := mermaidSelect(tree, files, stdin, rules.exclude)
	if refused != nil {
		return mermaidRefused(format, refused.code, refused.reason)
	}
	judgeEveryType := rules.allowedTypes != nil
	report := NewReport("mermaid", "diagram")
	for _, document := range selected {
		for _, block := range fencedBlocks(document.text) {
			if strings.TrimSpace(block.info) != "mermaid" {
				continue
			}
			// The form comes before anything about the diagram's content. A
			// repository that authors in plain text has said no Mermaid may
			// appear, and reporting the colours of a diagram that may not be
			// there would be answering the wrong question politely.
			if rules.authoring == "plain-text" {
				report.Inspected++
				line := 1
				if len(block.lines) > 0 {
					line = block.lines[0].number
				}
				finding := NewFinding(mermaidAuthoringKind, document.path,
					"carries a Mermaid diagram, and this repository authors conceptual diagrams in plain text")
				finding.Line = &line
				report.Findings = append(report.Findings, finding)
				continue
			}
			parsed := readDiagram(block, judgeEveryType)
			if parsed == nil {
				continue
			}
			report.Inspected++
			for _, finding := range inspectDiagram(parsed, rules) {
				finding.Path = document.path
				report.Findings = append(report.Findings, finding)
			}
		}
	}
	return report.Render(format)
}

// mermaidRefusal is why a mermaid selection yielded no document.
type mermaidRefusal struct {
	code   domain.ErrorCode
	reason string
}

// selectedDocument is one document the scope chose, read.
type selectedDocument struct {
	path string
	text string
}

// mermaidSelect resolves the narrowed selection or the declared surface.
// A selection naming an excluded path is skipped, not refused: a pre-commit
// run hands over every staged path, and a move into an archive stages files
// the repository has declared out of scope.
func mermaidSelect(tree Tree, files []string, stdin *string, excluded []string) ([]selectedDocument, *mermaidRefusal) {
	if len(files) > 0 {
		var selected []selectedDocument
		for _, path := range files {
			if matchesAny(excluded, path) {
				continue
			}
			if path == "-" {
				if stdin == nil {
					return nil, &mermaidRefusal{domain.FileUnreadable, "-: cannot be read"}
				}
				selected = append(selected, selectedDocument{path: "-", text: *stdin})
				continue
			}
			if tree.PassesThroughLink(path) {
				return nil, &mermaidRefusal{domain.PathEscapesRoot,
					path + ": passes through a symbolic link, which RHINO never follows"}
			}
			text, readErr := tree.Read(path)
			if readErr != nil {
				switch readErr.Kind {
				case "missing":
					return nil, &mermaidRefusal{domain.FileMissing, path + ": does not exist"}
				default:
					return nil, &mermaidRefusal{domain.FileUnreadable, path + ": cannot be read"}
				}
			}
			selected = append(selected, selectedDocument{path: path, text: text})
		}
		return selected, nil
	}
	var selected []selectedDocument
	for _, path := range tree.Files() {
		if !isMarkdown(path) || matchesAny(excluded, path) {
			continue
		}
		text, readErr := tree.Read(path)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				continue
			case "not-text":
				return nil, &mermaidRefusal{domain.FileUnreadable, path + ": holds no text"}
			default:
				return nil, &mermaidRefusal{domain.FileUnreadable, path + ": " + readErr.Reason}
			}
		}
		selected = append(selected, selectedDocument{path: path, text: text})
	}
	return selected, nil
}

// decodeMermaid projects the declared mermaid policy. The required lists
// stay required: a surface that requires nothing and one whose requirements
// were forgotten have to look different in the file.
func decodeMermaid(mapping map[string]any) (mermaidRules, error) {
	var rules mermaidRules
	for key := range mapping {
		switch key {
		case "authoring-rule", "node-label-graphemes", "edge-label-graphemes",
			"fill-colors", "edge-colors", "text-colors", "exclude",
			"require-default-class", "allowed-types", "forbid-theme-overrides",
			"canvas-colors":
		default:
			return rules, fmt.Errorf("policies.markdown.mermaid names unknown key `%s`", key)
		}
	}
	if raw, present := mapping["authoring-rule"]; present {
		rule, ok := raw.(string)
		if !ok || (rule != "rendered" && rule != "plain-text") {
			name := fmt.Sprintf("%v", raw)
			if !ok {
				return rules, fmt.Errorf("policies.markdown.mermaid names no authoring-rule")
			}
			return rules, fmt.Errorf("md-mermaid.authoring-rule: unrecognized rule `%s`: expected `rendered` or `plain-text`", name)
		}
		rules.authoring = rule
	}
	var err error
	if rules.nodeLimit, err = mermaidCount(mapping, "node-label-graphemes"); err != nil {
		return rules, err
	}
	if rules.edgeLimit, err = mermaidCount(mapping, "edge-label-graphemes"); err != nil {
		return rules, err
	}
	if rules.fillColors, err = mermaidColors(mapping, "fill-colors"); err != nil {
		return rules, err
	}
	if rules.edgeColors, err = mermaidColors(mapping, "edge-colors"); err != nil {
		return rules, err
	}
	if rules.textColors, err = mermaidColors(mapping, "text-colors"); err != nil {
		return rules, err
	}
	if raw, present := mapping["exclude"]; present {
		items, ok := raw.([]any)
		if !ok {
			return rules, fmt.Errorf("md-mermaid.exclude names no glob")
		}
		for _, item := range items {
			pattern, ok := item.(string)
			if !ok {
				return rules, fmt.Errorf("md-mermaid.exclude names no glob")
			}
			rules.exclude = append(rules.exclude, pattern)
		}
	}
	if raw, present := mapping["require-default-class"]; present {
		required, ok := raw.(bool)
		if !ok {
			return rules, fmt.Errorf("policies.markdown.mermaid names no require-default-class")
		}
		rules.requireDefaultClass = required
	}
	if raw, present := mapping["allowed-types"]; present {
		items, ok := raw.([]any)
		if !ok {
			return rules, fmt.Errorf("md-mermaid.allowed-types names no type")
		}
		declared := []string{}
		for _, item := range items {
			name, ok := item.(string)
			if !ok {
				return rules, fmt.Errorf("md-mermaid.allowed-types names no type")
			}
			declared = append(declared, name)
		}
		rules.allowedTypes = &declared
	}
	if raw, present := mapping["forbid-theme-overrides"]; present {
		forbid, ok := raw.(bool)
		if !ok {
			return rules, fmt.Errorf("policies.markdown.mermaid names no forbid-theme-overrides")
		}
		rules.forbidThemeOverrides = forbid
	}
	if raw, present := mapping["canvas-colors"]; present {
		items, ok := raw.([]any)
		if !ok {
			return rules, fmt.Errorf("md-mermaid.canvas-colors names no color")
		}
		for _, item := range items {
			color, ok := item.(string)
			if !ok {
				return rules, fmt.Errorf("md-mermaid.canvas-colors names no color")
			}
			rules.canvasColors = append(rules.canvasColors, color)
		}
	}
	return rules, nil
}

func mermaidCount(mapping map[string]any, key string) (int, error) {
	raw, ok := mapping[key].(int)
	if !ok || raw < 0 {
		return 0, fmt.Errorf("policies.markdown.mermaid names no %s", key)
	}
	return raw, nil
}

func mermaidColors(mapping map[string]any, key string) ([]string, error) {
	raw, ok := mapping[key].([]any)
	if !ok {
		return nil, fmt.Errorf("md-mermaid.%s names no color", key)
	}
	colors := make([]string, 0, len(raw))
	for _, item := range raw {
		color, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("md-mermaid.%s names no color", key)
		}
		colors = append(colors, color)
	}
	return colors, nil
}

// fencedBlocks lists every fenced region in a document. Nesting resolves the
// same way as in proseLines: a fence closes only on the same character at
// the same length or longer, so a block demonstrating fenced syntax does not
// truncate the block containing it.
func fencedBlocks(text string) []fencedBlock {
	var blocks []fencedBlock
	var openChar rune
	openLen := 0
	var info string
	var lines []fencedLine
	open := false
	for index, line := range strings.Split(text, "\n") {
		char, length := fenceOf(line)
		switch {
		case !open && length > 0:
			trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
			info = strings.TrimSpace(strings.TrimLeft(trimmed, string(char)))
			lines = nil
			openChar, openLen, open = char, length, true
		case open && length > 0 && char == openChar && length >= openLen:
			blocks = append(blocks, fencedBlock{info: info, lines: lines})
			lines = nil
			openChar, openLen, open = 0, 0, false
		case open:
			lines = append(lines, fencedLine{number: index + 1, text: line})
		}
	}
	return blocks
}

// readDiagram reads a fenced block as a diagram, or declines it. Declining
// covers both a block with no diagram declaration and one whose declaration
// this build cannot parse. Neither is a finding: reporting on syntax the
// tool does not understand would be reporting on its own ignorance.
func readDiagram(block fencedBlock, judgeEveryType bool) *diagram {
	var lines []fencedLine
	inFrontMatter := false
	var kind mermaidKind
	var declaration string
	found := false
	for _, line := range block.lines {
		trimmed := strings.TrimSpace(line.text)
		// Front matter is configuration for the renderer, not diagram
		// source, and the declaration sits after it.
		if trimmed == "---" {
			inFrontMatter = !inFrontMatter
			lines = append(lines, line)
			continue
		}
		if !found && !inFrontMatter && trimmed != "" && !strings.HasPrefix(trimmed, "%%") {
			fields := strings.Fields(trimmed)
			word := trimmed
			if len(fields) > 0 {
				word = fields[0]
			}
			parsed, ok := kindOf(word)
			if !ok && !judgeEveryType {
				return nil
			}
			if !ok {
				parsed = kindOther
			}
			kind, declaration, found = parsed, word, true
		}
		lines = append(lines, line)
	}
	if !found {
		return nil
	}
	return &diagram{kind: kind, declaration: declaration, lines: lines}
}

func inspectDiagram(diagram *diagram, rules mermaidRules) []Finding {
	// An undeclared type is refused alone: the other checks would be judging
	// a diagram the repository has said may not be there.
	if finding, refused := undeclaredType(diagram, rules); refused {
		return []Finding{finding}
	}
	var findings []Finding
	findings = append(findings, diagramAccessibility(diagram, rules)...)
	findings = append(findings, diagramColors(diagram, rules)...)
	findings = append(findings, defaultClass(diagram, rules)...)
	findings = append(findings, themeOverrides(diagram, rules)...)
	findings = append(findings, diagramLegibility(diagram, rules)...)
	sort.SliceStable(findings, func(i, j int) bool {
		left, right := 0, 0
		if findings[i].Line != nil {
			left = *findings[i].Line
		}
		if findings[j].Line != nil {
			right = *findings[j].Line
		}
		return left < right
	})
	return findings
}

// undeclaredType enforces the type allowlist: a type outside a declared list
// is refused, never skipped.
func undeclaredType(diagram *diagram, rules mermaidRules) (Finding, bool) {
	if rules.allowedTypes == nil {
		return Finding{}, false
	}
	for _, allowed := range *rules.allowedTypes {
		if allowed == diagram.declaration {
			return Finding{}, false
		}
	}
	line := diagramFirstLine(diagram)
	finding := NewFinding(mermaidAccessibilityKind, "",
		fmt.Sprintf("diagram type `%s` is not in the declared allowed types, so this repository does not render it",
			diagram.declaration))
	finding.Line = &line
	return finding, true
}

func diagramFirstLine(diagram *diagram) int {
	if len(diagram.lines) == 0 {
		return 1
	}
	return diagram.lines[0].number
}

// diagramAccessibility enforces the accessible title and description a
// rendered repository requires. Checked only where the repository declared
// the rule: a default here would be this tool choosing an authoring style on
// a repository's behalf.
func diagramAccessibility(diagram *diagram, rules mermaidRules) []Finding {
	if rules.authoring != "rendered" {
		return nil
	}
	line := diagramFirstLine(diagram)
	var findings []Finding
	for _, wanted := range [][2]string{{"accTitle", "title"}, {"accDescr", "description"}} {
		declared := false
		for _, text := range diagram.lines {
			trimmed := strings.TrimSpace(text.text)
			// Either form counts: `accDescr:` carries one line and
			// `accDescr {` opens a block. A rule that accepted only the
			// first would report a diagram whose description is longer than
			// a line as having none.
			if strings.HasPrefix(trimmed, wanted[0]+":") || strings.HasPrefix(trimmed, wanted[0]+" {") {
				declared = true
				break
			}
		}
		if !declared {
			finding := NewFinding(mermaidAccessibilityKind, "",
				fmt.Sprintf("declares no accessible %s; a rendered diagram is opaque to every reader who will never see it, including every automated check and every text search",
					wanted[1]))
			at := line
			finding.Line = &at
			findings = append(findings, finding)
		}
	}
	return findings
}

// -- Colour ---------------------------------------------------------------

// diagramColors refuses colour outside a classDef, where the declared palette
// cannot reach it. A classDef is the one place a diagram may set colour.
func diagramColors(diagram *diagram, rules mermaidRules) []Finding {
	var findings []Finding
	// Declaring the allowed types opts in to reading a colour by what it is
	// assigned to, so a label such as `PR #123` is prose and not a hex colour.
	strict := rules.allowedTypes != nil
	for _, line := range diagram.lines {
		trimmed := strings.TrimSpace(line.text)
		// A plain comment is prose about the palette, never the palette, so
		// a repeated or inaccurate one is not a finding. A `%%{...}%%`
		// directive is not a comment: it configures the renderer.
		if strings.HasPrefix(trimmed, "%%") && !strings.HasPrefix(trimmed, "%%{") {
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "classDef "); ok {
			findings = append(findings, classDefinition(line.number, rest, rules)...)
		} else if containsColor(trimmed, strict) || (strict && colorsABox(trimmed)) {
			finding := NewFinding(mermaidAccessibilityKind, "",
				"color is declared outside a classDef, where the declared palette cannot reach it")
			at := line.number
			finding.Line = &at
			findings = append(findings, finding)
		}
	}
	return findings
}

// defaultClassRoles are what a `classDef default` must set for every node to
// be legible with no class of its own.
var defaultClassRoles = []string{"fill", "stroke", "color"}

// appliesDefaultClass reports whether the renderer applies `classDef
// default` to a diagram of this kind. A state diagram ignores it, so
// demanding one there would demand dead text.
func appliesDefaultClass(kind mermaidKind) bool {
	return kind == kindFlow || kind == kindClass || kind == kindEntity || kind == kindRequirement
}

func defaultClass(diagram *diagram, rules mermaidRules) []Finding {
	if !rules.requireDefaultClass || !appliesDefaultClass(diagram.kind) {
		return nil
	}
	line := diagramFirstLine(diagram)
	declared := ""
	present := false
	for _, text := range diagram.lines {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(text.text), "classDef default "); ok {
			declared, present = rest, true
			break
		}
	}
	at := line
	if !present {
		finding := NewFinding(mermaidAccessibilityKind, "",
			"declares no `classDef default`, so every unclassed node takes a colour the palette never chose")
		finding.Line = &at
		return []Finding{finding}
	}
	var findings []Finding
	for _, role := range defaultClassRoles {
		set := false
		for _, assignment := range strings.Split(declared, ",") {
			name, _, ok := strings.Cut(assignment, ":")
			if ok && strings.TrimSpace(name) == role {
				set = true
				break
			}
		}
		if !set {
			finding := NewFinding(mermaidAccessibilityKind, "",
				fmt.Sprintf("`classDef default` sets no `%s`, so unclassed nodes fall back to an unchosen one", role))
			missing := line
			finding.Line = &missing
			findings = append(findings, finding)
		}
	}
	return findings
}

// themeOverrides refuses an initialization directive or a front-matter theme,
// each on its own line. Either one replaces the renderer's automatic light
// and dark switching with colours the palette never saw.
func themeOverrides(diagram *diagram, rules mermaidRules) []Finding {
	if !rules.forbidThemeOverrides {
		return nil
	}
	var findings []Finding
	inFrontMatter := false
	for _, line := range diagram.lines {
		trimmed := strings.TrimSpace(line.text)
		if trimmed == "---" {
			inFrontMatter = !inFrontMatter
			continue
		}
		directive := strings.HasPrefix(trimmed, "%%{") &&
			(strings.Contains(trimmed, "init") || strings.Contains(trimmed, "initialize"))
		frontMatter := inFrontMatter &&
			(strings.HasPrefix(trimmed, "theme:") ||
				strings.HasPrefix(trimmed, "themeVariables:") ||
				strings.HasPrefix(trimmed, "themeCSS:"))
		if directive || frontMatter {
			finding := NewFinding(mermaidAccessibilityKind, "",
				"a theme override replaces the renderer's light and dark switching with colours the palette never measured")
			at := line.number
			finding.Line = &at
			findings = append(findings, finding)
		}
	}
	return findings
}

func classDefinition(line int, rest string, rules mermaidRules) []Finding {
	var findings []Finding
	var fill, stroke, textColor *string
	// `classDef name k:v,k:v` -- the name is the first token, the rest are
	// the roles.
	assignments := ""
	if _, tail, ok := cutOnSpace(rest); ok {
		assignments = tail
	}
	for _, assignment := range strings.Split(assignments, ",") {
		role, value, ok := strings.Cut(assignment, ":")
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(value)
		switch strings.TrimSpace(role) {
		case "fill":
			fill = &trimmed
		case "stroke":
			stroke = &trimmed
		case "color":
			textColor = &trimmed
		}
	}
	refuse := func(message string) {
		finding := NewFinding(mermaidAccessibilityKind, "", message)
		at := line
		finding.Line = &at
		findings = append(findings, finding)
	}
	for _, role := range []struct {
		name  string
		value *string
	}{{"fill", fill}, {"stroke", stroke}, {"color", textColor}} {
		if role.value != nil && !isSixDigitHex(*role.value) {
			refuse(fmt.Sprintf("`%s:%s` is not a six-digit hex color, so it cannot be compared with the declared palette",
				role.name, *role.value))
		}
	}
	switch {
	case fill != nil:
		if isSixDigitHex(*fill) && !declares(rules.fillColors, *fill) {
			refuse(fmt.Sprintf("fill `%s` is not a declared fill color", *fill))
		}
		switch {
		case stroke == nil:
			refuse("a filled class has no stroke, so its boundary disappears against the background")
		default:
			if isSixDigitHex(*stroke) && !declares(rules.edgeColors, *stroke) {
				refuse(fmt.Sprintf("stroke `%s` is not a declared edge color", *stroke))
			}
			if strings.EqualFold(*stroke, *fill) {
				refuse(fmt.Sprintf("fill and stroke are both `%s`, so the shape has no visible boundary", *fill))
			}
		}
		switch {
		case textColor == nil:
			refuse("a filled class declares no text color, so its label inherits an unknown one")
		default:
			if isSixDigitHex(*textColor) && !declares(rules.textColors, *textColor) {
				refuse(fmt.Sprintf("text color `%s` is not a declared text color", *textColor))
			}
			// Only measure a pairing the repository actually permits.
			// Contrast against a colour it already rejected is a second
			// finding about a value that has to change anyway, and it
			// buries the one a reader can act on.
			if declares(rules.fillColors, *fill) && declares(rules.textColors, *textColor) {
				if background, ok := luminance(*fill); ok {
					if foreground, ok := luminance(*textColor); ok {
						if ratio := contrast(background, foreground); ratio < normalTextContrast {
							finding := NewFinding(mermaidAccessibilityKind, "",
								fmt.Sprintf("text `%s` on fill `%s` is below the normal-text contrast threshold",
									*textColor, *fill))
							at := line
							finding.Line = &at
							finding.Details = append(finding.Details,
								TextDetail("ratio", fmt.Sprintf("%.2f", ratio)),
								TextDetail("threshold", fmt.Sprintf("%.1f", normalTextContrast)))
							findings = append(findings, finding)
						}
					}
				}
			}
		}
	case stroke != nil && textColor == nil:
		// Stroke alone is a legitimate outline-only class.
		if isSixDigitHex(*stroke) && !declares(rules.edgeColors, *stroke) {
			refuse(fmt.Sprintf("stroke `%s` is not a declared edge color", *stroke))
		}
	case fill == nil && stroke == nil && textColor != nil:
		refuse("a class that sets only text color has neither a fill nor a stroke to make it legible")
	case stroke != nil && textColor != nil:
		if isSixDigitHex(*stroke) && !declares(rules.edgeColors, *stroke) {
			refuse(fmt.Sprintf("stroke `%s` is not a declared edge color", *stroke))
		}
	}
	findings = append(findings, canvasFindings(line, fill, stroke, rules)...)
	return findings
}

// cutOnSpace splits at the first whitespace character, the way a class name
// separates from its roles.
func cutOnSpace(text string) (string, string, bool) {
	for index, character := range text {
		if unicode.IsSpace(character) {
			return text[:index], text[index+len(string(character)):], true
		}
	}
	return text, "", false
}

// nonTextContrast is the WCAG 2 threshold for a graphical object against
// what is adjacent to it.
const nonTextContrast = 3.0

// canvasFindings demands that a filled class stay visible on every declared
// canvas: the fill or the outline reaches the non-text threshold, so a pale
// fill with a black outline passes on white and a dark fill with a pale
// outline passes on black.
func canvasFindings(line int, fill, stroke *string, rules mermaidRules) []Finding {
	if fill == nil || !isSixDigitHex(*fill) {
		return nil
	}
	var shapes []float64
	for _, color := range []*string{fill, stroke} {
		if color == nil {
			continue
		}
		if level, ok := luminance(*color); ok {
			shapes = append(shapes, level)
		}
	}
	var findings []Finding
	for _, canvas := range rules.canvasColors {
		background, ok := luminance(canvas)
		if !ok {
			continue
		}
		best := 0.0
		for _, shape := range shapes {
			if ratio := contrast(shape, background); ratio > best {
				best = ratio
			}
		}
		if best < nonTextContrast {
			finding := NewFinding(mermaidAccessibilityKind, "",
				fmt.Sprintf("neither fill `%s` nor its outline reaches the non-text contrast threshold against the canvas `%s`",
					*fill, canvas))
			at := line
			finding.Line = &at
			finding.Details = append(finding.Details,
				TextDetail("ratio", fmt.Sprintf("%.2f", best)),
				TextDetail("threshold", fmt.Sprintf("%.1f", nonTextContrast)),
				TextDetail("canvas", canvas))
			findings = append(findings, finding)
		}
	}
	return findings
}

// normalTextContrast is the WCAG 2 threshold for normal-size text. Not
// configurable: it is a property of human vision, not of a repository.
const normalTextContrast = 4.5

func declares(declared []string, value string) bool {
	for _, candidate := range declared {
		if strings.EqualFold(candidate, value) {
			return true
		}
	}
	return false
}

func isSixDigitHex(value string) bool {
	digits, ok := strings.CutPrefix(value, "#")
	if !ok {
		return false
	}
	if len(digits) != 6 {
		return false
	}
	for i := 0; i < len(digits); i++ {
		character := digits[i]
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f' || character >= 'A' && character <= 'F') {
			return false
		}
	}
	return true
}

// containsColor reports whether a line sets a colour anywhere. Deliberately
// broad: outside a classDef there is no acceptable colour, so the question
// is only whether one is present, never which.
func containsColor(line string, strict bool) bool {
	if strings.Contains(line, "rgb(") || strings.Contains(line, "rgba(") || strings.Contains(line, "hsl(") {
		return true
	}
	for index, character := range line {
		if character != '#' {
			continue
		}
		digits := 0
		for _, following := range line[index+1:] {
			if !isASCIIHexDigit(following) {
				break
			}
			digits++
		}
		if digits < 3 || digits > 8 {
			continue
		}
		if !strict || isAssignedColor(line[:index]) {
			return true
		}
	}
	// A colour named as a word rather than a value, in a role that sets one.
	for _, assignment := range strings.Split(line, ",") {
		role, value, ok := strings.Cut(assignment, ":")
		if !ok {
			continue
		}
		fields := strings.Split(strings.TrimSpace(role), " ")
		if last := fields[len(fields)-1]; (last == "fill" || last == "stroke" || last == "color") &&
			strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func isASCIIHexDigit(character rune) bool {
	return character >= '0' && character <= '9' || character >= 'a' && character <= 'f' || character >= 'A' && character <= 'F'
}

// isAssignedColor reports whether the text before a `#` makes it the value
// of a colour property. `fill:#fff` and `"primaryColor": "#fff"` qualify;
// `fixes PR #123` does not, because nothing there assigns a colour.
func isAssignedColor(before string) bool {
	assigned := strings.TrimRight(before, " \"'")
	property, ok := cutSuffix(assigned, ":")
	if !ok {
		return false
	}
	trimmed := strings.TrimRight(property, " \"'")
	index := len(trimmed)
	for index > 0 && isASCIIAlphanumeric(trimmed[index-1]) {
		index--
	}
	switch word := strings.ToLower(trimmed[index:]); {
	case word == "fill" || word == "stroke" || word == "background" || word == "bkg":
		return true
	case strings.HasSuffix(word, "color") || strings.HasSuffix(word, "bkg") || strings.HasSuffix(word, "border"):
		return true
	}
	return false
}

func cutSuffix(text, suffix string) (string, bool) {
	if !strings.HasSuffix(text, suffix) {
		return text, false
	}
	return text[:len(text)-len(suffix)], true
}

func isASCIIAlphanumeric(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9'
}

// colorsABox reports a sequence-diagram box naming a colour, which that
// syntax cannot express as a hex value and so can never come from the palette.
func colorsABox(line string) bool {
	rest, ok := strings.CutPrefix(line, "box ")
	if !ok {
		return false
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "aqua", "black", "blue", "brown", "cyan", "fuchsia", "gray", "grey",
		"green", "lime", "magenta", "maroon", "navy", "olive", "orange",
		"pink", "purple", "red", "silver", "teal", "white", "yellow",
		"lightblue", "lightgreen":
		return true
	}
	return false
}

func luminance(color string) (float64, bool) {
	digits, ok := strings.CutPrefix(color, "#")
	if !ok || len(digits) != 6 {
		return 0, false
	}
	channel := func(from int) (float64, bool) {
		value, err := strconv.ParseUint(digits[from:from+2], 16, 8)
		if err != nil {
			return 0, false
		}
		scaled := float64(value) / 255.0
		if scaled <= 0.03928 {
			return scaled / 12.92, true
		}
		return math.Pow((scaled+0.055)/1.055, 2.4), true
	}
	red, ok := channel(0)
	if !ok {
		return 0, false
	}
	green, ok := channel(2)
	if !ok {
		return 0, false
	}
	blue, ok := channel(4)
	if !ok {
		return 0, false
	}
	return 0.2126*red + 0.7152*green + 0.0722*blue, true
}

func contrast(first, second float64) float64 {
	lighter, darker := first, second
	if darker > lighter {
		lighter, darker = darker, lighter
	}
	return (lighter + 0.05) / (darker + 0.05)
}

// -- Legibility -------------------------------------------------------------

// labelSegment is where a label was found, which is also what a consumer
// filters on.
type labelSegment int

const (
	segNode labelSegment = iota
	segEdge
)

func (segment labelSegment) name() string {
	if segment == segNode {
		return "node"
	}
	return "edge"
}

func (segment labelSegment) limit(rules mermaidRules) int {
	if segment == segNode {
		return rules.nodeLimit
	}
	return rules.edgeLimit
}

// labeledText is one visible label with what kind of thing it labels.
type labeledText struct {
	segment labelSegment
	label   string
}

func diagramLegibility(diagram *diagram, rules mermaidRules) []Finding {
	var findings []Finding
	inAccessibilityBlock := false
	for _, line := range diagram.lines {
		trimmed := strings.TrimSpace(line.text)
		// The accessible description is prose for a screen reader, not a
		// label a sighted reader sees, so its length and punctuation are
		// not drawn.
		if inAccessibilityBlock {
			if strings.Contains(trimmed, "}") {
				inAccessibilityBlock = false
			}
			continue
		}
		if opensAccessibilityBlock(trimmed) {
			inAccessibilityBlock = true
			continue
		}
		if isNotLabelSource(diagram.kind, trimmed) {
			continue
		}
		// A transition ending in a semicolon renders as part of the label
		// in some viewers, which is a legibility fault rather than a
		// syntax one.
		if diagram.kind == kindState && strings.Contains(trimmed, "-->") && strings.HasSuffix(trimmed, ";") {
			finding := NewFinding(mermaidLegibilityKind, "",
				"a state transition ends in a semicolon, which some renderers draw as part of the label")
			at := line.number
			finding.Line = &at
			findings = append(findings, finding)
		}
		for _, labeled := range lineLabels(diagram.kind, trimmed) {
			limit := labeled.segment.limit(rules)
			for _, visible := range labelSegments(labeled.label) {
				if measured := graphemeCount(visible); measured > limit {
					finding := NewFinding(mermaidLegibilityKind, "",
						fmt.Sprintf("a %s label is longer than the declared limit", labeled.segment.name()))
					at := line.number
					finding.Line = &at
					measuredCopy, limitCopy := measured, limit
					finding.Details = append(finding.Details,
						TextDetail("segment", labeled.segment.name()),
						CountDetail("measured", measuredCopy),
						CountDetail("limit", limitCopy))
					findings = append(findings, finding)
				}
			}
		}
	}
	return findings
}

// opensAccessibilityBlock reports whether a line opens a multi-line
// `accTitle { ... }` or `accDescr { ... }` block, whose lines up to the
// closing brace are prose and not labels.
func opensAccessibilityBlock(line string) bool {
	for _, name := range []string{"accTitle", "accDescr"} {
		rest, ok := strings.CutPrefix(line, name)
		if !ok {
			continue
		}
		if strings.TrimSpace(rest) == "{" {
			return true
		}
		trimmed := strings.TrimLeftFunc(rest, unicode.IsSpace)
		if strings.HasPrefix(trimmed, "{") && !strings.Contains(trimmed, "}") {
			return true
		}
	}
	return false
}

// isNotLabelSource reports lines that declare styling, behaviour, or
// structure rather than visible text. `class` is the interesting case: in a
// class diagram it names a node a reader sees, and in a flowchart it applies
// a classDef to one.
func isNotLabelSource(kind mermaidKind, line string) bool {
	if line == "" || strings.HasPrefix(line, "%%") || line == "---" {
		return true
	}
	first := line
	if fields := strings.Fields(line); len(fields) > 0 {
		first = fields[0]
	}
	switch first {
	case "accTitle:", "accDescr:", "classDef", "style", "linkStyle", "click", "direction", "call", "href":
		return true
	case "class":
		return kind != kindClass
	}
	return false
}

// lineLabels lists every visible label a line declares, with what kind of
// thing it labels.
func lineLabels(kind mermaidKind, line string) []labeledText {
	switch kind {
	case kindFlow, kindBlock:
		return flowLabels(line)
	case kindClass:
		if name, ok := enclosedAfter(line, "class"); ok {
			return []labeledText{{segment: segNode, label: name}}
		}
		return nil
	case kindState:
		if label, ok := quoted(line); ok {
			return []labeledText{{segment: segNode, label: label}}
		}
		return nil
	case kindEntity:
		return entityLabels(line)
	case kindRequirement:
		if name, ok := enclosedAfter(line, "requirement"); ok {
			return []labeledText{{segment: segNode, label: name}}
		}
		return nil
	default:
		// No grammar is known, so no label can be told from other text.
		return nil
	}
}

func flowLabels(line string) []labeledText {
	var found []labeledText
	// Edge labels first, and their text is removed from what the node scan
	// sees, so a pipe-delimited label is never also read as a node.
	var remainder strings.Builder
	rest := line
	for {
		start := strings.Index(rest, "|")
		if start < 0 {
			break
		}
		after := rest[start+1:]
		end := strings.Index(after, "|")
		if end < 0 {
			break
		}
		found = append(found, labeledText{segment: segEdge, label: after[:end]})
		remainder.WriteString(rest[:start])
		rest = after[end+1:]
	}
	remainder.WriteString(rest)
	// A text edge writes its label between the dashes: `A -- label --> B`.
	body := remainder.String()
	if head, tail, ok := strings.Cut(body, "-- "); ok && !strings.Contains(head, "-->") {
		if label, _, ok := strings.Cut(tail, "-->"); ok {
			found = append(found, labeledText{segment: segEdge, label: strings.TrimSpace(label)})
		}
	}
	for _, delimiters := range [][2]rune{{'[', ']'}, {'(', ')'}, {'{', '}'}} {
		for _, label := range between(body, delimiters[0], delimiters[1]) {
			found = append(found, labeledText{segment: segNode, label: label})
		}
	}
	return found
}

func entityLabels(line string) []labeledText {
	// `ALPHA ||--|| BETA : places` -- the entity names are what a reader sees.
	head := line
	if index := strings.Index(line, ":"); index >= 0 {
		head = line[:index]
	}
	var found []labeledText
	for _, token := range strings.Fields(head) {
		plain := true
		for _, character := range token {
			if !(unicode.IsLetter(character) || unicode.IsDigit(character) || character == '_' || character == '-') {
				plain = false
				break
			}
		}
		if plain {
			found = append(found, labeledText{segment: segNode, label: token})
		}
	}
	return found
}

// enclosedAfter returns the identifier following a keyword, stripped of any
// trailing brace.
func enclosedAfter(line, keyword string) (string, bool) {
	rest, ok := strings.CutPrefix(line, keyword)
	if !ok {
		return "", false
	}
	fields := strings.Fields(strings.TrimLeftFunc(rest, unicode.IsSpace))
	if len(fields) == 0 {
		return "", false
	}
	return strings.Trim(strings.TrimRight(fields[0], "{"), "\""), true
}

func quoted(line string) (string, bool) {
	start := strings.IndexByte(line, '"')
	if start < 0 {
		return "", false
	}
	rest := line[start+1:]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

// between returns every run of text between a delimiter pair, innermost first.
func between(line string, open, close rune) []string {
	var found []string
	depth := 0
	start := 0
	for index, character := range line {
		if character == open {
			if depth == 0 {
				start = index + len(string(character))
			}
			depth++
		} else if character == close && depth > 0 {
			depth--
			if depth == 0 {
				found = append(found, strings.Trim(line[start:index], "\""))
			}
		}
	}
	return found
}

// labelSegments splits a label's visible segments, decoded and stripped of
// markup. A break splits a label into lines a reader sees separately, so the
// limit is per segment: two thirty-two grapheme segments are two legible
// lines, not one illegible one.
func labelSegments(label string) []string {
	normalised := label
	for _, separator := range []string{"<br/>", "<br />", "<br>", "\\n"} {
		normalised = strings.ReplaceAll(normalised, separator, "")
	}
	parts := strings.Split(normalised, "")
	visible := make([]string, 0, len(parts))
	for _, part := range parts {
		visible = append(visible, decodeEntities(stripMarkup(part)))
	}
	return visible
}

// stripMarkup removes HTML tags, which are markup a reader never counts.
func stripMarkup(text string) string {
	var out strings.Builder
	depth := 0
	for _, character := range text {
		switch character {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			} else {
				out.WriteRune(character)
			}
		default:
			if depth == 0 {
				out.WriteRune(character)
			}
		}
	}
	return out.String()
}

// decodeEntities decodes the HTML entities Mermaid passes through, so a label
// is measured as it is seen rather than as it is written.
func decodeEntities(text string) string {
	var out strings.Builder
	rest := text
	for {
		start := strings.IndexByte(rest, '&')
		if start < 0 {
			out.WriteString(rest)
			return out.String()
		}
		out.WriteString(rest[:start])
		after := rest[start:]
		end := strings.IndexByte(after, ';')
		if end < 0 {
			out.WriteByte('&')
			rest = after[1:]
			continue
		}
		var decoded rune
		ok := true
		switch entity := after[1:end]; entity {
		case "amp":
			decoded = '&'
		case "lt":
			decoded = '<'
		case "gt":
			decoded = '>'
		case "quot":
			decoded = '"'
		case "apos":
			decoded = '\''
		case "nbsp":
			decoded = ' '
		default:
			decoded, ok = numericEntity(entity)
		}
		if ok {
			out.WriteRune(decoded)
			rest = after[end+1:]
		} else {
			out.WriteByte('&')
			rest = after[1:]
		}
	}
}

func numericEntity(entity string) (rune, bool) {
	digits, ok := strings.CutPrefix(entity, "#")
	if !ok || digits == "" {
		return 0, false
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] == '+' || digits[i] == '-' {
			return 0, false
		}
	}
	var code uint64
	var err error
	if rest, ok := strings.CutPrefix(digits, "x"); ok {
		code, err = strconv.ParseUint(rest, 16, 32)
	} else if rest, ok := strings.CutPrefix(digits, "X"); ok {
		code, err = strconv.ParseUint(rest, 16, 32)
	} else {
		code, err = strconv.ParseUint(digits, 10, 32)
	}
	if err != nil {
		return 0, false
	}
	if code >= 0xD800 && code <= 0xDFFF || code > 0x10FFFF {
		return 0, false
	}
	return rune(code), true
}

// graphemeCount measures a visible segment in extended grapheme clusters,
// the units a reader counts.
func graphemeCount(text string) int {
	count := 0
	segments := uniseg.NewGraphemes(text)
	for segments.Next() {
		count++
	}
	return count
}

func mermaidRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("mermaid", "diagram")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
