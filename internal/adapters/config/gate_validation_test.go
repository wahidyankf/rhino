package config_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// gateTemplate builds one check gate. The inputs, argv, and run-on text are
// spliced in so each test states only the declaration it exercises.
func gateTemplate(inputs, args, runOn string) string {
	return "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: typed-gate\n      type: check\n" +
		"      inputs:\n" + inputs +
		"      command:\n        executable: ./rhino\n        args:\n" + args +
		"      run-on:\n" + runOn
}

// TestGateDeclarationRefusals proves the static typed-input rules refuse each
// unsound declaration with the reason the oracle names, before any gate runs.
func TestGateDeclarationRefusals(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "unknown source variant is refused as an unknown variant",
			text: gateTemplate("        files: { kind: files }\n",
				"          - { input: files.paths, expand: repeat }\n",
				"        manual:\n          bind:\n            files: { source: nx-affected }\n"),
			want: "unknown variant `nx-affected`",
		},
		{
			name: "source that does not accept the input kind is refused",
			text: gateTemplate("        message: { kind: commit-message }\n",
				"          - { input: message.text, expand: single }\n",
				"        manual:\n          bind:\n            message: { source: git-index }\n"),
			want: "not a compatible source",
		},
		{
			name: "argv reference to an undeclared input is refused",
			text: gateTemplate("        repository: { kind: repository-state }\n",
				"          - { input: missing.root, expand: single }\n",
				"        manual:\n          bind:\n            repository: { source: checkout }\n"),
			want: "names an input this gate does not declare",
		},
		{
			name: "reference to a field the kind does not offer is refused",
			text: gateTemplate("        message: { kind: commit-message }\n",
				"          - { input: message.paths, expand: single }\n",
				"        manual:\n          bind:\n            message: { source: hook-message-file }\n"),
			want: "does not name a field available on a commit-message input",
		},
		{
			name: "declared input that is never projected is refused",
			text: gateTemplate("        repository: { kind: repository-state }\n",
				"          - { literal: check }\n",
				"        manual:\n          bind:\n            repository: { source: checkout }\n"),
			want: "is declared but never projected",
		},
		{
			name: "run-on membership cannot carry a command",
			text: gateTemplate("",
				"          - { literal: check }\n",
				"        manual:\n          command:\n            executable: not-allowed-here\n"),
			want: "unknown field `command`",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := config.Parse(test.text)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want refusal containing %q", err, test.want)
			}
		})
	}
}

// TestGateDeclarationAcceptsSoundTypedInputs proves the same rules accept a
// declaration that binds, projects, and sources each input correctly.
func TestGateDeclarationAcceptsSoundTypedInputs(t *testing.T) {
	text := gateTemplate("        repository: { kind: repository-state }\n",
		"          - { input: repository.root, expand: single }\n",
		"        manual:\n          bind:\n            repository: { source: checkout }\n")
	if _, err := config.Parse(text); err != nil {
		t.Fatalf("sound typed declaration refused: %v", err)
	}
}
