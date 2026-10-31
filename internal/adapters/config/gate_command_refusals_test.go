package config_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// TestGateCommandRefusals proves each malformed argv or environment projection
// is refused with the oracle's reason, one case per refusal the static check
// names, so a declaration never reaches dispatch.
func TestGateCommandRefusals(t *testing.T) {
	const head = "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: typed-gate\n      type: check\n      inputs:\n        repository: { kind: repository-state }\n"
	const manual = "      run-on:\n        manual:\n          bind:\n            repository: { source: checkout }\n"
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "environment name must be uppercase",
			text: head + "      command:\n        executable: ./rhino\n        environment:\n          lower: { input: repository.root }\n" + manual,
			want: "is not an uppercase environment projection name",
		},
		{
			name: "typed inputs need a typed command",
			text: head + manual,
			want: "declares typed inputs but no typed executable",
		},
		{
			name: "an argv item holds exactly one of literal or input",
			text: head + "      command:\n        executable: ./rhino\n        args:\n          - { literal: x, input: repository.root, expand: single }\n" + manual,
			want: "must hold exactly one literal or typed input argv item",
		},
		{
			name: "an input argv item omits no expansion mode",
			text: head + "      command:\n        executable: ./rhino\n        args:\n          - { input: repository.root }\n" + manual,
			want: "omits the closed expansion mode from an input argv item",
		},
		{
			name: "an unknown expansion mode is an unknown variant",
			text: head + "      command:\n        executable: ./rhino\n        args:\n          - { input: repository.root, expand: many }\n" + manual,
			want: "unknown variant `many`",
		},
		{
			name: "repeat expands only file-path sequences",
			text: head + "      command:\n        executable: ./rhino\n        args:\n          - { input: repository.root, expand: repeat }\n" + manual,
			want: "uses `expand: repeat` for a field that is not a file-path sequence",
		},
		{
			name: "a reference names exactly one field",
			text: head + "      command:\n        executable: ./rhino\n        args:\n          - { input: repository.root.extra, expand: single }\n" + manual,
			want: "is not an `input.field` reference",
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
