// Traceability validation: declared artifacts exist and link as declared.
//
// Each artifact pairs a simple identifier with an exact repository-relative
// path, and each relationship names two declared artifacts the source must
// reach by a local link. A name the configuration never declared, or a shape
// the configuration never described, fails the run closed: a check that
// silently ignored what it could not read would be a check that never ran.
package application

import (
	"fmt"
	"sort"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// traceabilityArtifact pairs a simple identifier with the exact path it claims.
type traceabilityArtifact struct {
	id   string
	path string
}

// traceabilityRelationship names two declared artifacts the source must link.
type traceabilityRelationship struct {
	from string
	to   string
}

func traceabilityValidate(document *config.Document, tree Tree, format Format) Outcome {
	mapping, ok := document.PolicyGroup("governance", "traceability")
	if !ok {
		report := NewReport("traceability", "artifact")
		report.Refusal = strptr("policies.governance.traceability is not declared, and RHINO holds no default for it")
		report.RefusalCode = domain.ConfigUndeclared
		return report.Render(format)
	}
	artifacts, relationships, refusal := decodeTraceability(mapping)
	if refusal != nil {
		return traceabilityRefused(format, domain.ConfigUnusable, refusal.Error())
	}
	byID := make(map[string]string, len(artifacts))
	for _, artifact := range artifacts {
		// The identifier rule is the layer simple-name rule: lowercase
		// ASCII letters, digits, and hyphens, shared so an editor and this
		// check agree on what a name is.
		if !isLayerSimpleName(artifact.id) || !isExactPath(artifact.path) {
			return traceabilityRefused(format, domain.ConfigUnusable,
				"traceability artifacts require unique simple identifiers and exact repository-relative paths")
		}
		if _, repeated := byID[artifact.id]; repeated {
			return traceabilityRefused(format, domain.ConfigUnusable,
				"traceability artifact identifiers must not repeat")
		}
		byID[artifact.id] = artifact.path
	}
	for _, relationship := range relationships {
		if _, ok := byID[relationship.from]; !ok {
			return traceabilityRefused(format, domain.ConfigUnusable,
				"every traceability relationship must name declared artifacts")
		}
		if _, ok := byID[relationship.to]; !ok {
			return traceabilityRefused(format, domain.ConfigUnusable,
				"every traceability relationship must name declared artifacts")
		}
	}
	report := NewReport("traceability", "artifact")
	contents := make(map[string]string, len(byID))
	// Sorted, so two runs produce byte-identical output.
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		path := byID[id]
		text, readErr := tree.Read(path)
		if readErr != nil {
			switch readErr.Kind {
			case "missing":
				report.Findings = append(report.Findings, NewFinding("missing-traceability-artifact", path,
					"the declared traceability artifact is absent"))
				continue
			case "not-text":
				return traceabilityRefused(format, domain.FileUnreadable, path+": holds no text")
			default:
				return traceabilityRefused(format, domain.FileUnreadable, path+": "+readErr.Reason)
			}
		}
		report.Scanned = append(report.Scanned, path)
		report.Inspected++
		contents[id] = text
	}
	for _, relationship := range relationships {
		source := byID[relationship.from]
		target := byID[relationship.to]
		text, present := contents[relationship.from]
		if !present {
			continue
		}
		// A missing source is already reported as itself: with no text to
		// read, there is no link to accuse, only an artifact to restore.
		if !readmeTargets(source, text)[target] {
			finding := NewFinding("missing-traceability-relationship", source,
				"the declared artifact relationship has no matching local link")
			finding.Details = append(finding.Details, TextDetail("target", target))
			report.Findings = append(report.Findings, finding)
		}
	}
	return report.Render(format)
}

// decodeTraceability projects the declared artifact vocabulary. Unknown keys
// refuse: a policy the tool silently ignored would be a check that never ran.
// An absent list holds nothing; a present one must hold mappings.
func decodeTraceability(mapping map[string]any) ([]traceabilityArtifact, []traceabilityRelationship, error) {
	for key := range mapping {
		if key != "artifacts" && key != "relationships" {
			return nil, nil, fmt.Errorf("policies.governance.traceability names unknown key `%s`", key)
		}
	}
	artifacts, err := decodeTraceabilityArtifacts(mapping)
	if err != nil {
		return nil, nil, err
	}
	relationships, err := decodeTraceabilityRelationships(mapping)
	if err != nil {
		return nil, nil, err
	}
	return artifacts, relationships, nil
}

// decodeTraceabilityArtifacts projects the declared artifacts. An entry that
// is not a mapping, or that names no identifier or path, reads as empty and
// fails the semantic check below: the run still refuses, with the one
// message that describes what an artifact has to be.
func decodeTraceabilityArtifacts(mapping map[string]any) ([]traceabilityArtifact, error) {
	raw, present := mapping["artifacts"]
	if !present || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("traceability.artifacts names no artifacts")
	}
	artifacts := make([]traceabilityArtifact, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			artifacts = append(artifacts, traceabilityArtifact{})
			continue
		}
		for key := range entry {
			if key != "id" && key != "path" {
				return nil, fmt.Errorf("traceability.artifacts name unknown key `%s`", key)
			}
		}
		id, _ := entry["id"].(string)
		path, _ := entry["path"].(string)
		artifacts = append(artifacts, traceabilityArtifact{id: id, path: path})
	}
	return artifacts, nil
}

// decodeTraceabilityRelationships projects the declared relationships. A
// relationship naming nothing declared reads as empty and fails the declared
// check below, the same way an unshaped artifact does.
func decodeTraceabilityRelationships(mapping map[string]any) ([]traceabilityRelationship, error) {
	raw, present := mapping["relationships"]
	if !present || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("traceability.relationships names no relationships")
	}
	relationships := make([]traceabilityRelationship, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			relationships = append(relationships, traceabilityRelationship{})
			continue
		}
		for key := range entry {
			if key != "from" && key != "to" {
				return nil, fmt.Errorf("traceability.relationships name unknown key `%s`", key)
			}
		}
		from, _ := entry["from"].(string)
		to, _ := entry["to"].(string)
		relationships = append(relationships, traceabilityRelationship{from: from, to: to})
	}
	return relationships, nil
}

func traceabilityRefused(format Format, code domain.ErrorCode, reason string) Outcome {
	report := NewReport("traceability", "artifact")
	report.Refusal = &reason
	report.RefusalCode = code
	return report.Render(format)
}
