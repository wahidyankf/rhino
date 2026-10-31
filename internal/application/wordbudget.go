// Word-budget validation: declared surfaces against their declared limits.
//
// A repository declares which files are governed and how long each may be.
// RHINO holds no opinion about either: it enforces what the configuration
// says and reports what it counted, which is why a clean run still lists
// every path it read. Only Markdown files are counted, and where two globs
// match one file the last matching entry wins.
package application

import (
	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

func wordBudgetValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("governance", "word-budget")
	if !ok {
		report := NewReport("word-budget", "file")
		report.Refusal = strptr("policies.governance.word-budget is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	if raw, present := mapping["surfaces"]; !present || raw == nil {
		return wordBudgetRefused(format, domain.ConfigUnusable,
			"word-budget.surfaces names no budget surfaces")
	}
	budget, err := decodeWordBudget(mapping)
	if err != nil {
		return wordBudgetRefused(format, domain.ConfigUnusable, err.Error())
	}
	skipped, err := document.ScanExcludeDirectories()
	if err != nil {
		return wordBudgetRefused(format, domain.ConfigUnusable, err.Error())
	}
	report := NewReport("word-budget", "file")
	for _, path := range tree.Files() {
		if !isMarkdown(path) || scanExcluded(path, skipped) {
			continue
		}
		limit := budget.limit(path)
		if limit == nil {
			continue
		}
		text, readErr := tree.Read(path)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				continue
			case "not-text":
				return wordBudgetRefused(format, domain.FileUnreadable, path+": holds no text")
			default:
				return wordBudgetRefused(format, domain.FileUnreadable, path+": "+readErr.Reason)
			}
		}
		report.Scanned = append(report.Scanned, path)
		report.Inspected++
		if words := budget.countWords(text); words > *limit {
			finding := NewFinding("word-limit-exceeded", path,
				"is longer than its declared word budget")
			finding.Details = append(finding.Details,
				CountDetail("words", words), CountDetail("limit", *limit))
			report.Findings = append(report.Findings, finding)
		}
	}
	return report.Render(format)
}

func wordBudgetRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("word-budget", "file")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
