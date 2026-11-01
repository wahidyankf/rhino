package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

// metadataFrontMatter wraps a front-matter body in the delimiters a document
// carries, so each row changes only the lines it tests.
func metadataFrontMatter(body string) string {
	return "---\n" + body + "---\n"
}

// TestMetadataReadsLayoutAndQuotingInFrontMatter proves a blank line inside the
// front matter, a blank line after a list, and a quoted name each read as the
// same portable agent, so the layout of a valid document never refuses it.
func TestMetadataReadsLayoutAndQuotingInFrontMatter(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "a blank line between keys is ignored",
			body: "name: example\n\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n",
		},
		{
			name: "a blank line after a list ends the list",
			body: "name: example\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n\n",
		},
		{
			name: "a double-quoted name reads as its unquoted text",
			body: "name: \"example\"\ndescription: A complete portable agent description that is long enough.\nwhen_to_use: >-\n  Use this agent when its declared task is the current task.\ntier: execution\ncapabilities:\n  - repository-read\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = metadataAgentConfig
			tree.files["agents/example.md"] = metadataFrontMatter(tc.body)
			outcome := application.Execute(tree, args("metadata", "validate"), nil)
			if outcome.ExitCode != 0 {
				t.Fatalf("valid layout refused: %+v", outcome)
			}
		})
	}
}

// TestMetadataRejectsANameWithNoPathIdentity proves a skill or workflow entry
// at the repository root, where no directory names it, cannot have its declared
// name match its path, so the name is reported rather than silently accepted.
func TestMetadataRejectsANameWithNoPathIdentity(t *testing.T) {
	cases := []struct {
		name   string
		config string
		file   string
	}{
		{
			name:   "a skill file at the root has no directory identity",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"SKILL.md\"\n          schema: skill\n",
			file:   "SKILL.md",
		},
		{
			name:   "a workflow README at the root has no directory identity",
			config: "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"README.md\"\n          schema: workflow\n",
			file:   "README.md",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := newMemoryTree("/root")
			tree.files["repo-config.yml"] = tc.config
			tree.files[tc.file] = metadataFrontMatter("name: root-entry\ndescription: A portable description that is long enough here.\n" +
				"when_to_use: >-\n  Use this entry when its declared task is the current task.\n")
			outcome := application.Execute(tree, args("metadata", "validate"), nil)
			if outcome.ExitCode != 1 || !strings.Contains(outcome.Stdout+outcome.Stderr, "name is not the identity its path already states") {
				t.Fatalf("a name with no path identity was not reported: %+v", outcome)
			}
		})
	}
}

// TestMetadataSkipsASurfaceFileThatVanishedAfterListing proves a surface file the
// walk listed and the read found gone is skipped, neither accused nor refused.
func TestMetadataSkipsASurfaceFileThatVanishedAfterListing(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = metadataAgentConfig
	tree.files["agents/gone.md"] = metadataFrontMatter("name: gone\n")
	tree.vanished["agents/gone.md"] = true
	outcome := application.Execute(tree, args("metadata", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a vanished file refused or was accused: %+v", outcome)
	}
}

// TestMetadataRefusesASurfaceThatReachesAnEscapingLink proves a metadata surface
// whose glob reaches a link leaving the repository refuses the run as an escape.
func TestMetadataRefusesASurfaceThatReachesAnEscapingLink(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    metadata:\n      surfaces:\n        - glob: \"linked/**/*.md\"\n          schema: governance\n"
	tree.treeLinks = []application.TreeLink{{Path: "linked", Kind: application.LinkOutside}}
	outcome := application.Execute(tree, args("metadata", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "linked") {
		t.Fatalf("escaping surface link not refused: %+v", outcome)
	}
}
