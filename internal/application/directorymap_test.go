package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
)

func directoryMapTree(config string, files map[string]string) *memoryTree {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = config
	for path, content := range files {
		tree.files[path] = content
	}
	return tree
}

const directoryMapBase = "schema: rhino/repo-config/v2\npolicies:\n  governance:\n    directory-map:\n"

func directoryMapArgs(directory string) []string {
	if directory == "" {
		return args("governance", "directory-map", "validate")
	}
	return args("governance", "directory-map", "validate", "--directory", directory)
}

func TestExecuteDirectoryMapCleanAndFindings(t *testing.T) {
	cases := []struct {
		name      string
		config    string
		files     map[string]string
		directory string
		exitCode  int
		contains  []string
		absent    []string
	}{
		{
			name:   "a tree whose READMEs list every sibling reports clean",
			config: directoryMapBase + "      trees:\n        - path: docs\n",
			files: map[string]string{
				"docs/README.md":       "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n- [Child](child/README.md)\n",
				"docs/guide.md":        "# Guide\n",
				"docs/child/README.md": "# Child\n\n## Directory Map\n",
			},
			exitCode: 0,
			contains: []string{"[directory-map] checked 2 directories, no findings"},
		},
		{
			name:     "an empty tree list reports clean with zero inspected",
			config:   directoryMapBase + "      trees: []\n",
			files:    map[string]string{"elsewhere/note.md": "note\n"},
			exitCode: 0,
			contains: []string{"[directory-map] checked 0 directories, no findings"},
		},
		{
			name:   "a directory with no README is reported missing",
			config: directoryMapBase + "      trees:\n        - path: docs\n        - path: other\n",
			files: map[string]string{
				"docs/README.md": "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n",
				"docs/guide.md":  "# Guide\n",
				"other/file.md":  "# File\n",
			},
			exitCode: 1,
			contains: []string{
				"[directory-map] checked 2 directories, 1 finding",
				"other: missing README: a mapped directory has to document its own contents",
			},
		},
		{
			name:   "a README with no map section is reported missing",
			config: directoryMapBase + "      trees:\n        - path: docs\n",
			files: map[string]string{
				"docs/README.md": "# Docs\n",
				"docs/guide.md":  "# Guide\n",
			},
			exitCode: 1,
			contains: []string{
				"docs/README.md: missing directory map: a mapped README needs a `## Directory Map` section",
			},
		},
		{
			name:   "an entry addressing outside the directory is reported invalid with its line",
			config: directoryMapBase + "      trees:\n        - path: docs\n",
			files: map[string]string{
				"docs/README.md": "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n- [Bad](../outside.md)\n",
				"docs/guide.md":  "# Guide\n",
			},
			exitCode: 1,
			contains: []string{
				"docs/README.md:5: invalid map entry `../outside.md`: an entry has to name a direct sibling by a relative path that exists",
			},
		},
		{
			name:   "an unlisted sibling is reported missing",
			config: directoryMapBase + "      trees:\n        - path: docs\n",
			files: map[string]string{
				"docs/README.md":   "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n",
				"docs/guide.md":    "# Guide\n",
				"docs/unlisted.md": "# Unlisted\n",
			},
			exitCode: 1,
			contains: []string{
				"missing map entry for `docs/unlisted.md`: every direct sibling appears exactly once",
			},
		},
		{
			name:   "an entry naming nothing in the repository is reported invalid",
			config: directoryMapBase + "      trees:\n        - path: docs\n",
			files: map[string]string{
				"docs/README.md": "# Docs\n\n## Directory Map\n- [Ghost](ghost.md)\n- [Guide](guide.md)\n",
				"docs/guide.md":  "# Guide\n",
			},
			exitCode: 1,
			contains: []string{
				"invalid map entry `ghost.md`",
			},
		},
		{
			name:   "a repeated entry still covers its sibling",
			config: directoryMapBase + "      trees:\n        - path: docs\n",
			files: map[string]string{
				"docs/README.md": "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n- [Guide again](guide.md)\n",
				"docs/guide.md":  "# Guide\n",
			},
			exitCode: 0,
			contains: []string{"[directory-map] checked 1 directory, no findings"},
		},
		{
			name:   "entries with fragments queries star bullets and indentation resolve",
			config: directoryMapBase + "      trees:\n        - path: docs\n",
			files: map[string]string{
				"docs/README.md":       "# Docs\n\n## Directory Map\n   * [Guide](guide.md#frag?q=1)\n- [Child](child)\n## Next\n- [No](no.md)\n",
				"docs/guide.md":        "# Guide\n",
				"docs/child/README.md": "# Child\n\n## Directory Map\n",
			},
			exitCode: 0,
			contains: []string{"[directory-map] checked 2 directories, no findings"},
		},
		{
			name:   "a selected directory replaces the declared trees",
			config: directoryMapBase + "      trees:\n        - path: other\n",
			files: map[string]string{
				"docs/README.md": "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n",
				"docs/guide.md":  "# Guide\n",
				"other/file.md":  "# File\n",
			},
			directory: "docs",
			exitCode:  0,
			contains:  []string{"[directory-map] checked 1 directory, no findings"},
		},
		{
			name:   "a scan exclusion removes a whole tree from the walk",
			config: "schema: rhino/repo-config/v2\nscan:\n  exclude-directories: [other]\npolicies:\n  governance:\n    directory-map:\n      trees:\n        - path: docs\n        - path: other\n",
			files: map[string]string{
				"docs/README.md": "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n",
				"docs/guide.md":  "# Guide\n",
				"other/file.md":  "# File\n",
			},
			exitCode: 0,
			contains: []string{"[directory-map] checked 1 directory, no findings"},
		},
		{
			name:   "an excluded subdirectory is still accused when its map omits it",
			config: "schema: rhino/repo-config/v2\nscan:\n  exclude-directories: [child]\npolicies:\n  governance:\n    directory-map:\n      trees:\n        - path: docs\n",
			files: map[string]string{
				"docs/README.md":       "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n",
				"docs/guide.md":        "# Guide\n",
				"docs/child/README.md": "# Child\n\n## Directory Map\n",
			},
			exitCode: 1,
			contains: []string{
				"missing map entry for `docs/child`: every direct sibling appears exactly once",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := directoryMapTree(tc.config, tc.files)
			outcome := application.Execute(tree, directoryMapArgs(tc.directory), nil)
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

func TestExecuteDirectoryMapJSON(t *testing.T) {
	tree := directoryMapTree(
		directoryMapBase+"      trees:\n        - path: docs\n        - path: other\n",
		map[string]string{
			"docs/README.md":   "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n- [Bad](../outside.md)\n",
			"docs/guide.md":    "# Guide\n",
			"docs/unlisted.md": "# Unlisted\n",
			"other/file.md":    "# File\n",
		},
	)
	outcome := application.Execute(tree, args("governance", "directory-map", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 1 {
		t.Fatalf("expected findings: %+v", outcome)
	}
	for _, want := range []string{
		`"command":"directory-map"`,
		`"subject":"directory"`,
		`"exitCode":1`,
		`"inspected":2`,
		`"kind":"invalid-map-entry"`,
		`"path":"docs/README.md"`,
		`"line":5`,
		`"message":"invalid map entry ` + "`../outside.md`" + `: an entry has to name a direct sibling by a relative path that exists"`,
		`"kind":"missing-map-entry"`,
		`"kind":"missing-readme"`,
		`"path":"other"`,
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("expected %q in %q", want, outcome.Stdout)
		}
	}
	if outcome.Stderr != "" {
		t.Fatalf("json findings must leave stderr empty: %+v", outcome)
	}
}

func TestExecuteDirectoryMapCleanJSON(t *testing.T) {
	tree := directoryMapTree(
		directoryMapBase+"      trees:\n        - path: docs\n",
		map[string]string{
			"docs/README.md": "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n",
			"docs/guide.md":  "# Guide\n",
		},
	)
	outcome := application.Execute(tree, args("governance", "directory-map", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("expected clean: %+v", outcome)
	}
	for _, want := range []string{
		`"command":"directory-map"`,
		`"exitCode":0`,
		`"inspected":1`,
		`"violations":[]`,
	} {
		if !strings.Contains(outcome.Stdout, want) {
			t.Fatalf("expected %q in %q", want, outcome.Stdout)
		}
	}
}

func TestExecuteDirectoryMapRefusals(t *testing.T) {
	cases := []struct {
		name      string
		config    string
		files     map[string]string
		unread    []string
		binary    []string
		links     []string
		directory string
		exitCode  int
		contains  []string
	}{
		{
			name:     "an undeclared directory-map section refuses rather than defaulting",
			config:   "schema: rhino/repo-config/v2\npolicies:\n  governance: {}\n",
			exitCode: 2,
			contains: []string{"policies.governance.directory-map is not declared"},
		},
		{
			name:     "a climbing declared tree refuses the whole run",
			config:   directoryMapBase + "      trees:\n        - path: ../outside\n",
			exitCode: 2,
			contains: []string{"policies.governance.directory-map.trees: `../outside` is not an exact repository-relative path"},
		},
		{
			name:     "an absolute declared tree refuses the whole run",
			config:   directoryMapBase + "      trees:\n        - path: /abs\n",
			exitCode: 2,
			contains: []string{"policies.governance.directory-map.trees: `/abs` is not an exact repository-relative path"},
		},
		{
			name:     "a glob declared tree refuses the whole run",
			config:   directoryMapBase + "      trees:\n        - path: \"docs/*\"\n",
			exitCode: 2,
			contains: []string{"policies.governance.directory-map.trees: `docs/*` is not an exact repository-relative path"},
		},
		{
			name:     "a declared tree behind a symbolic link is never read",
			config:   directoryMapBase + "      trees:\n        - path: linked\n",
			files:    map[string]string{"linked/note.md": "held\n"},
			links:    []string{"linked"},
			exitCode: 2,
			contains: []string{"linked: passes through a symbolic link, which RHINO never follows"},
		},
		{
			name:      "a selected directory that names nothing refuses as missing",
			config:    directoryMapBase + "      trees:\n        - path: docs\n",
			files:     map[string]string{"docs/README.md": "# Docs\n\n## Directory Map\n"},
			directory: "missing",
			exitCode:  2,
			contains:  []string{"missing is not a directory in this repository"},
		},
		{
			name:   "a selected file refuses as not a directory",
			config: directoryMapBase + "      trees:\n        - path: docs\n",
			files: map[string]string{
				"docs/README.md": "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n",
				"docs/guide.md":  "# Guide\n",
			},
			directory: "docs/guide.md",
			exitCode:  2,
			contains:  []string{"docs/guide.md is not a directory in this repository"},
		},
		{
			name:   "a selected directory behind a link refuses rather than being walked",
			config: directoryMapBase + "      trees:\n        - path: docs\n",
			files: map[string]string{
				"docs/README.md": "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n",
				"docs/guide.md":  "# Guide\n",
			},
			links:     []string{"linkdocs"},
			directory: "linkdocs",
			exitCode:  2,
			contains:  []string{"linkdocs passes through a symbolic link, which RHINO never follows"},
		},
		{
			name:   "an unreadable README fails closed with its path",
			config: directoryMapBase + "      trees:\n        - path: docs\n",
			files: map[string]string{
				"docs/README.md": "# Docs\n\n## Directory Map\n- [Guide](guide.md)\n",
				"docs/guide.md":  "# Guide\n",
			},
			unread:   []string{"docs/README.md"},
			exitCode: 2,
			contains: []string{"docs/README.md"},
		},
		{
			name:   "a binary README fails closed as holding no text",
			config: directoryMapBase + "      trees:\n        - path: docs\n",
			files: map[string]string{
				"docs/README.md": "bytes",
				"docs/guide.md":  "# Guide\n",
			},
			binary:   []string{"docs/README.md"},
			exitCode: 2,
			contains: []string{"docs/README.md: holds no text"},
		},
		{
			name:     "an unknown directory-map key refuses rather than being ignored",
			config:   directoryMapBase + "      bogus: true\n",
			exitCode: 2,
			contains: []string{"policies.governance.directory-map names unknown key `bogus`"},
		},
		{
			name:     "scalar trees refuse rather than reading as one tree",
			config:   directoryMapBase + "      trees: docs\n",
			exitCode: 2,
			contains: []string{"directory-map.trees names no trees"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := directoryMapTree(tc.config, tc.files)
			for _, path := range tc.unread {
				tree.unreadable[path] = true
			}
			for _, path := range tc.binary {
				tree.binary[path] = true
			}
			for _, path := range tc.links {
				tree.links[path] = true
			}
			outcome := application.Execute(tree, directoryMapArgs(tc.directory), nil)
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

func TestExecuteDirectoryMapRefusalCodes(t *testing.T) {
	missing := directoryMapTree(
		directoryMapBase+"      trees:\n        - path: docs\n",
		map[string]string{"docs/README.md": "# Docs\n\n## Directory Map\n"},
	)
	outcome := application.Execute(missing, args("governance", "directory-map", "validate", "--directory", "missing", "--output", "json"), nil)
	if outcome.ExitCode != 2 {
		t.Fatalf("expected refusal: %+v", outcome)
	}
	if !strings.Contains(outcome.Stdout+outcome.Stderr, `"code":"rhino.file.missing"`) {
		t.Fatalf("expected file-missing code in %+v", outcome)
	}
	linked := directoryMapTree(
		directoryMapBase+"      trees:\n        - path: docs\n",
		map[string]string{"docs/README.md": "# Docs\n\n## Directory Map\n"},
	)
	linked.links["linkdocs"] = true
	outcome = application.Execute(linked, args("governance", "directory-map", "validate", "--directory", "linkdocs", "--output", "json"), nil)
	if outcome.ExitCode != 2 {
		t.Fatalf("expected refusal: %+v", outcome)
	}
	if !strings.Contains(outcome.Stdout+outcome.Stderr, `"code":"rhino.path.escapes-root"`) {
		t.Fatalf("expected escapes-root code in %+v", outcome)
	}
	undeclared := directoryMapTree("schema: rhino/repo-config/v2\npolicies:\n  governance: {}\n", nil)
	outcome = application.Execute(undeclared, args("governance", "directory-map", "validate", "--output", "json"), nil)
	if outcome.ExitCode != 2 {
		t.Fatalf("expected refusal: %+v", outcome)
	}
	if !strings.Contains(outcome.Stdout+outcome.Stderr, `"code":"rhino.config.undeclared"`) {
		t.Fatalf("expected config-undeclared code in %+v", outcome)
	}
}
