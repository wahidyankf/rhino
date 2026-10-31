// Validator cores: read-only, network-free, process-free, path-contained.
package application

import (
	"fmt"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// emojiBlock is one code-point range the emoji prohibition covers. The
// boundaries are the rule: neighbours such as em dashes, arrows, and
// copyright signs live just outside them and must not be accused.
type emojiBlock struct {
	first rune
	last  rune
}

var emojiBlocks = []emojiBlock{
	{0x1F000, 0x1FAFF},
	{0x2600, 0x27BF},
	{0x2B00, 0x2BFF},
	{0xFE00, 0xFE0F},
	{0x3297, 0x3299},
}

func isEmoji(character rune) bool {
	for _, block := range emojiBlocks {
		if character >= block.first && character <= block.last {
			return true
		}
	}
	return false
}

func emojiValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("conventions", "emoji")
	if !ok {
		report := NewReport("emoji", "file")
		report.Refusal = strptr("policies.conventions.emoji is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	raw, ok := mapping["prohibited"].([]any)
	if !ok {
		return emojiRefused(format, domain.ConfigUnusable,
			"policies.conventions.emoji.prohibited names no glob")
	}
	var globs []string
	for _, item := range raw {
		member, ok := item.(map[string]any)
		if !ok {
			return emojiRefused(format, domain.ConfigUnusable,
				"policies.conventions.emoji.prohibited names no glob")
		}
		glob, _ := member["glob"].(string)
		if strings.TrimSpace(glob) == "" {
			return emojiRefused(format, domain.ConfigUnusable,
				"policies.conventions.emoji.prohibited names no glob")
		}
		globs = append(globs, glob)
	}
	// The prohibited surfaces select from the shared corpus, the way the
	// oracle's surface walk drops shared-excluded directories first.
	shared, scanErr := document.ScanExcludeDirectories()
	if scanErr != nil {
		return emojiRefused(format, domain.ConfigUnusable, scanErr.Error())
	}
	selected, err := Select(scanFiles(tree.Files(), shared), "policies.conventions.emoji.prohibited", globs)
	if err != nil {
		return emojiRefused(format, domain.ConfigUnusable, err.Error())
	}
	report := NewReport("emoji", "file")
	for _, path := range selected {
		text, readErr := tree.Read(path)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				continue
			case "not-text":
				return emojiRefused(format, domain.FileUnreadable, path+": holds no text")
			default:
				return emojiRefused(format, domain.FileUnreadable, path+": "+readErr.Reason)
			}
		}
		report.Scanned = append(report.Scanned, path)
		report.Inspected++
		for number, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
			// One finding per line: three emoji on one line are one edit.
			for _, character := range line {
				if isEmoji(character) {
					finding := NewFinding("emoji-in-prohibited-file", path, "holds an emoji code point")
					at := number + 1
					finding.Line = &at
					finding.Details = append(finding.Details, TextDetail("codePoint", fmt.Sprintf("U+%04X", character)))
					report.Findings = append(report.Findings, finding)
					break
				}
			}
		}
	}
	return report.Render(format)
}

func emojiRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("emoji", "file")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
