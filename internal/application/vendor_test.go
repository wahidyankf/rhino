package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func vendorTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const vendorBase = "schema: rhino/repo-config/v2\npolicies:\n  governance:\n    vendor:\n"

func TestExecuteVendorCleanAndFindings(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		exitCode int
		contains []string
		absent   []string
	}{
		{
			name:     "a vendor policy with no roots checks no files and reports clean",
			config:   vendorBase + "      forbidden-terms: [vendor-name]\n",
			files:    map[string]string{"portable/code.md": "vendor-name\n"},
			exitCode: 0,
			contains: []string{"[vendor] checked 0 files, no findings"},
		},
		{
			name:     "an empty vendor policy checks no files and reports clean",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  governance:\n    vendor: {}\n",
			files:    map[string]string{"portable/code.md": "vendor-name\n"},
			exitCode: 0,
			contains: []string{"[vendor] checked 0 files, no findings"},
		},
		{
			name:     "a file holding no declared term reports clean and is scanned",
			config:   vendorBase + "      roots: [portable]\n      forbidden-terms: [zzz-nope]\n",
			files:    map[string]string{"portable/code.md": "portable text\n"},
			exitCode: 0,
			contains: []string{
				"[vendor] checked 1 file, no findings",
				"[vendor] scanned portable/code.md",
			},
		},
		{
			name:     "a file holding a declared term is reported with the term",
			config:   vendorBase + "      roots: [portable]\n      forbidden-terms: [vendor-name]\n",
			files:    map[string]string{"portable/code.md": "holds vendor-name here\n"},
			exitCode: 1,
			contains: []string{
				"[vendor] checked 1 file, 1 finding",
				"[vendor] scanned portable/code.md",
				"portable/code.md: a configured vendor term appears outside its exact exception (term=vendor-name)",
			},
		},
		{
			name: "an exact exception excuses its file but not its neighbours",
			config: vendorBase + "      roots: [portable]\n      forbidden-terms: [vendor-name]\n" +
				"      vocabulary-exceptions:\n        - term: vendor-name\n          paths: [portable/allowed.md]\n",
			files: map[string]string{
				"portable/allowed.md": "vendor-name is fine here\n",
				"portable/code.md":    "vendor-name is not fine here\n",
			},
			exitCode: 1,
			contains: []string{
				"portable/code.md: a configured vendor term appears outside its exact exception",
			},
			absent: []string{"portable/allowed.md: a configured vendor term"},
		},
		{
			name: "an exception for another term does not excuse this one",
			config: vendorBase + "      roots: [portable]\n      forbidden-terms: [vendor-name, other]\n" +
				"      vocabulary-exceptions:\n        - term: other\n          paths: [portable/code.md]\n",
			files:    map[string]string{"portable/code.md": "vendor-name and other\n"},
			exitCode: 1,
			contains: []string{
				"[vendor] checked 1 file, 1 finding",
				"term=vendor-name",
			},
			absent: []string{"term=other"},
		},
		{
			name:     "files outside every root are never read and never accused",
			config:   vendorBase + "      roots: [portable]\n      forbidden-terms: [vendor-name]\n",
			files:    map[string]string{"elsewhere/code.md": "vendor-name\n"},
			exitCode: 0,
			contains: []string{"[vendor] checked 0 files, no findings"},
			absent:   []string{"elsewhere"},
		},
		{
			name:   "files under excluded binding roots are never read and never accused",
			config: vendorBase + "      roots: [portable]\n      excluded-binding-roots: [portable/bindings]\n      forbidden-terms: [vendor-name]\n",
			files: map[string]string{
				"portable/bindings/ignored.md": "vendor-name\n",
				"portable/code.md":             "clean\n",
			},
			exitCode: 0,
			contains: []string{"[vendor] checked 1 file, no findings"},
			absent:   []string{"bindings"},
		},
		{
			name:     "a root naming a file directly still scans it",
			config:   vendorBase + "      roots: [portable/code.md]\n      forbidden-terms: [vendor-name]\n",
			files:    map[string]string{"portable/code.md": "vendor-name\n"},
			exitCode: 1,
			contains: []string{
				"[vendor] checked 1 file, 1 finding",
				"portable/code.md: a configured vendor term appears outside its exact exception",
			},
		},
		{
			name:     "every matching term in one file is reported",
			config:   vendorBase + "      roots: [portable]\n      forbidden-terms: [alpha, beta]\n",
			files:    map[string]string{"portable/code.md": "alpha and beta\n"},
			exitCode: 1,
			contains: []string{
				"[vendor] checked 1 file, 2 findings",
				"term=alpha",
				"term=beta",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := vendorTree(tc.config, tc.files)
			outcome := application.Execute(tree, args("governance", "vendor", "validate"), nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("expected exit %d: %+v", tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("expected %q in %+v", want, outcome)
				}
			}
			for _, want := range tc.absent {
				if strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("expected %q absent from %+v", want, outcome)
				}
			}
		})
	}
}

func TestExecuteVendorJSONCarriesKinds(t *testing.T) {
	tree := vendorTree(
		vendorBase+"      roots: [portable]\n      forbidden-terms: [vendor-name]\n",
		map[string]string{"portable/code.md": "vendor-name\n"},
	)
	outcome := application.Execute(tree, args("governance", "vendor", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, want := range []string{
		`"command":"vendor"`,
		`"subject":"file"`,
		`"exitCode":1`,
		`"inspected":1`,
		`"scanned":["portable/code.md"]`,
		`"kind":"forbidden-vendor-term"`,
		`"path":"portable/code.md"`,
		`"message":"a configured vendor term appears outside its exact exception"`,
		`"term":"vendor-name"`,
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("expected %q in %q", want, outcome.Stdout)
		}
	}
	if outcome.Stderr != "" {
		t.Fatalf("json findings must leave stderr empty: %+v", outcome)
	}
}

func TestExecuteVendorCleanJSON(t *testing.T) {
	tree := vendorTree(
		vendorBase+"      roots: [portable]\n      forbidden-terms: [zzz-nope]\n",
		map[string]string{"portable/code.md": "clean\n"},
	)
	outcome := application.Execute(tree, args("governance", "vendor", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("expected clean: %+v", outcome)
	}
	for _, want := range []string{
		`"command":"vendor"`,
		`"exitCode":0`,
		`"violations":[]`,
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("expected %q in %q", want, outcome.Stdout)
		}
	}
}

func TestExecuteVendorRefusals(t *testing.T) {
	cases := []struct {
		name     string
		config   string
		files    map[string]string
		unread   []string
		binary   []string
		links    []string
		exitCode int
		contains []string
	}{
		{
			name:     "an undeclared vendor section refuses rather than defaulting",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  governance: {}\n",
			exitCode: 2,
			contains: []string{"policies.governance.vendor is not declared"},
		},
		{
			name:     "an absolute vendor root refuses the whole run",
			config:   vendorBase + "      roots: [/portable]\n",
			exitCode: 2,
			contains: []string{"vendor roots and excluded binding roots must be exact repository-relative paths"},
		},
		{
			name:     "a glob vendor root refuses the whole run",
			config:   vendorBase + "      roots: [\"portable/*.md\"]\n",
			exitCode: 2,
			contains: []string{"vendor roots and excluded binding roots must be exact repository-relative paths"},
		},
		{
			name:     "a climbing excluded binding root refuses the whole run",
			config:   vendorBase + "      roots: [portable]\n      excluded-binding-roots: [../bindings]\n",
			exitCode: 2,
			contains: []string{"vendor roots and excluded binding roots must be exact repository-relative paths"},
		},
		{
			name:     "a blank forbidden term refuses the whole run",
			config:   vendorBase + "      roots: [portable]\n      forbidden-terms: [\"  \"]\n",
			exitCode: 2,
			contains: []string{"vendor forbidden terms must not be empty"},
		},
		{
			name: "a climbing exception path refuses the whole run",
			config: vendorBase + "      roots: [portable]\n" +
				"      vocabulary-exceptions:\n        - term: vendor-name\n          paths: [../up.md]\n",
			exitCode: 2,
			contains: []string{"vendor exception terms and paths must be exact repository-relative paths"},
		},
		{
			name: "a blank exception term refuses the whole run",
			config: vendorBase + "      roots: [portable]\n" +
				"      vocabulary-exceptions:\n        - term: \" \"\n          paths: [portable/allowed.md]\n",
			exitCode: 2,
			contains: []string{"vendor exception terms and paths must be exact repository-relative paths"},
		},
		{
			name:     "an unreadable file under a root fails closed with its path",
			config:   vendorBase + "      roots: [portable]\n      forbidden-terms: [vendor-name]\n",
			files:    map[string]string{"portable/code.md": "vendor-name\n"},
			unread:   []string{"portable/code.md"},
			exitCode: 2,
			contains: []string{"portable/code.md"},
		},
		{
			name:     "a binary file under a root fails closed as holding no text",
			config:   vendorBase + "      roots: [portable]\n      forbidden-terms: [vendor-name]\n",
			files:    map[string]string{"portable/code.md": "bytes"},
			binary:   []string{"portable/code.md"},
			exitCode: 2,
			contains: []string{"portable/code.md: holds no text"},
		},
		{
			name:     "a root behind a link is never followed",
			config:   vendorBase + "      roots: [linkroot]\n      forbidden-terms: [vendor-name]\n",
			files:    map[string]string{"linkroot/note.md": "vendor-name\n"},
			links:    []string{"linkroot"},
			exitCode: 2,
			contains: []string{"linkroot: passes through a symbolic link, which RHINO never follows"},
		},
		{
			name:     "an unknown vendor key refuses rather than being ignored",
			config:   vendorBase + "      roots: [portable]\n      bogus: true\n",
			exitCode: 2,
			contains: []string{"policies.governance.vendor names unknown key `bogus`"},
		},
		{
			name:     "scalar roots refuse rather than reading as one root",
			config:   vendorBase + "      roots: portable\n",
			exitCode: 2,
			contains: []string{"vendor.roots names no roots"},
		},
		{
			name:     "a scalar forbidden term refuses rather than reading as one term",
			config:   vendorBase + "      roots: [portable]\n      forbidden-terms: vendor-name\n",
			exitCode: 2,
			contains: []string{"vendor.forbidden-terms names no terms"},
		},
		{
			name:     "a vendor refusal reports as JSON when asked",
			config:   vendorBase + "      roots: [/portable]\n",
			exitCode: 2,
			contains: []string{"rhino.config.unusable"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := vendorTree(tc.config, tc.files)
			for _, path := range tc.unread {
				tree.unreadable[path] = true
			}
			for _, path := range tc.binary {
				tree.binary[path] = true
			}
			for _, path := range tc.links {
				tree.links[path] = true
			}
			format := args("governance", "vendor", "validate")
			if strings.Contains(tc.name, "as JSON") {
				format = args("governance", "vendor", "validate", "--output", "json")
			}
			outcome := application.Execute(tree, format, nil)
			if outcome.ExitCode != tc.exitCode {
				t.Fatalf("expected exit %d: %+v", tc.exitCode, outcome)
			}
			for _, want := range tc.contains {
				if !strings.Contains(outcome.Stdout+outcome.Stderr, want) {
					t.Fatalf("expected %q in %+v", want, outcome)
				}
			}
		})
	}
}
