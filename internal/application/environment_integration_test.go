// Integration level: this file reaches the real host, so the default build omits it (see .golangci.yml).

//go:build integration

package application_test

import (
	"strings"
	"testing"
)

// envTree is a memory tree with link-aware reads and an optional Git index
// snapshot, the two boundaries environment operations depend on.
func TestEnvironmentValidateReadsOnlyDeclaredSources(t *testing.T) {
	config := "environment:\n" +
		"  contracts:\n    - path: .env.example\n      keys: [RUST_KEY, GO_KEY]\n" +
		"  detectors:\n    - language: rust\n      paths: [src/main.rs]\n" +
		"    - language: go\n      paths: [cmd/service/main.go]\n"
	tree := envConfig(config)
	tree.files["src/main.rs"] = "std::env::var(\"RUST_KEY\");\n// SYNTHETIC=value\n"
	tree.files["cmd/service/main.go"] = "os.LookupEnv(\"GO_KEY\")\n"
	tree.files[".env.example"] = "RUST_KEY=\nGO_KEY=\n"

	clean := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate")
	if clean.ExitCode != 0 || clean.Stdout != "[environment-validate] checked 2 declared files, no findings\n" {
		t.Fatalf("clean validate differs: %+v", clean)
	}
	if strings.Contains(clean.Stdout+clean.Stderr, "SYNTHETIC") {
		t.Fatalf("validate reported a value: %+v", clean)
	}

	tree.files["cmd/service/main.go"] = "os.Getenv(\"OUTSIDE\")\n// SYNTHETIC=value\n"
	found := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate")
	if found.ExitCode != 1 {
		t.Fatalf("undeclared read not reported: %+v", found)
	}
	if !strings.Contains(found.Stderr, "[environment-validate] .env.example: declared-but-unread (GO_KEY)") ||
		!strings.Contains(found.Stderr, "[environment-validate] cmd/service/main.go: read-but-undeclared (OUTSIDE)") {
		t.Fatalf("findings differ: %+v", found)
	}
	if strings.Contains(found.Stdout+found.Stderr, "SYNTHETIC") {
		t.Fatalf("findings reported a value: %+v", found)
	}

	json := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate", "--output", "json")
	if json.ExitCode != 1 || json.Stdout != "{\"command\":\"environment-validate\",\"exitCode\":1,\"findings\":[{\"key\":\"GO_KEY\",\"path\":\".env.example\",\"rule\":\"declared-but-unread\"},{\"key\":\"OUTSIDE\",\"path\":\"cmd/service/main.go\",\"rule\":\"read-but-undeclared\"}],\"schemaVersion\":1}\n" {
		t.Fatalf("findings json differs: %+v", json)
	}

	dynamic := envConfig("environment:\n  detectors:\n    - language: rust\n      paths: [src/main.rs]\n")
	dynamic.files["src/main.rs"] = "std::env::var(name);\n// SYNTHETIC=value\n"
	redacted := runEnv(dynamic, &memoryEnvStore{tree: dynamic}, "env", "validate")
	if redacted.ExitCode != 1 || !strings.Contains(redacted.Stderr, "unsupported-dynamic-access (<dynamic>)") {
		t.Fatalf("dynamic access not redacted: %+v", redacted)
	}
	if strings.Contains(redacted.Stdout+redacted.Stderr, "SYNTHETIC") {
		t.Fatalf("dynamic finding reported a value: %+v", redacted)
	}
}
func TestEnvironmentDetectorLanguagesReportOnlyNames(t *testing.T) {
	for _, tc := range []struct {
		name     string
		language string
		source   string
		keys     string
		want     int
		stderr   string
	}{
		{"rust var_os", "rust", "std::env::var_os(\"R2\")\n// SYNTHETIC=value\n", "[R2]", 0, ""},
		{"rust dynamic", "rust", "std::env::var(name);\n// SYNTHETIC=value\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"rust unterminated", "rust", "std::env::var(\"ABC\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"type-script brackets", "type-script", "process.env[\"TS\"]\n", "[TS]", 0, ""},
		{"type-script dotted", "type-script", "process.env.TS2\n", "[TS2]", 0, ""},
		{"type-script object key", "type-script", "export const environment = {\n  SERVICE_TOKEN: text(),\n};\n", "[SERVICE_TOKEN]", 0, ""},
		{"type-script dynamic", "type-script", "process.env[value]; process.env.\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"f-sharp", "f-sharp", "Environment.GetEnvironmentVariable(\"FS\")\n", "[FS]", 0, ""},
		{"f-sharp dynamic", "f-sharp", "Environment.GetEnvironmentVariable(name)\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"go injected lookup", "go", "resolve(os.LookupEnv, \"G2\")\n", "[G2]", 0, ""},
		{"go dynamic", "go", "os.Getenv(name)\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"terraform env", "terraform", "value = env(\"TF2\")\n", "[TF2]", 0, ""},
		{"terraform dynamic", "terraform", "value = get_env(name)\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"ansible env", "ansible", "{{ lookup('env', 'A2') }}\n", "[A2]", 0, ""},
		{"ansible other lookup ignored", "ansible", "{{ lookup('file', 'ignored') }}\n", "[]", 0, ""},
		{"ansible missing comma", "ansible", "{{ lookup('env') }}\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"ansible unquoted key", "ansible", "{{ lookup('env', name) }}\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"ansible unquoted lookup", "ansible", "{{ lookup(name, 'A3') }}\n", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"type-script marker at the end of the source", "type-script", "process.env.", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"ansible lookup at the end of the source", "ansible", "{{ lookup(", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
		{"go call at the end of the source", "go", "os.Getenv(", "[]", 1, "unsupported-dynamic-access (<dynamic>)"},
	} {
		tree := envConfig("environment:\n" +
			"  contracts:\n    - path: cfg\n      keys: " + tc.keys + "\n" +
			"  detectors:\n    - language: " + tc.language + "\n      paths: [src/code]\n")
		tree.files["src/code"] = tc.source
		tree.files["cfg"] = "declared\n"
		outcome := runEnv(tree, &memoryEnvStore{tree: tree}, "env", "validate")
		if outcome.ExitCode != tc.want {
			t.Fatalf("%s: exit %d, want %d: %+v", tc.name, outcome.ExitCode, tc.want, outcome)
		}
		if tc.stderr != "" && !strings.Contains(outcome.Stderr, tc.stderr) {
			t.Fatalf("%s stderr missing %q: %+v", tc.name, tc.stderr, outcome)
		}
		if strings.Contains(outcome.Stdout+outcome.Stderr, "SYNTHETIC") {
			t.Fatalf("%s reported a value: %+v", tc.name, outcome)
		}
	}
}
