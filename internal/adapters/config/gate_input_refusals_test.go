package config_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// TestGateInputDeclarationRefusals proves each input-level rule the oracle
// applies refuses its declaration with the oracle's reason, before any gate can
// dispatch: naming, kind, binding, range, and fallback.
func TestGateInputDeclarationRefusals(t *testing.T) {
	const command = "      command:\n        executable: ./rhino\n        args:\n          - { input: item.paths, expand: repeat }\n"
	tests := []struct {
		name   string
		inputs string
		bind   string
		want   string
	}{
		{
			name:   "an input name must be kebab-case",
			inputs: "        Bad_Name: { kind: files }\n",
			bind:   "            Bad_Name: { source: git-index }\n",
			want:   "is not a portable input name; expected lowercase kebab-case",
		},
		{
			name:   "an unknown kind is an unknown variant",
			inputs: "        item: { kind: nx-graph }\n",
			bind:   "            item: { source: git-index }\n",
			want:   "unknown variant `nx-graph`",
		},
		{
			name:   "a binding for an undeclared input is refused",
			inputs: "        item: { kind: files }\n",
			bind:   "            item: { source: git-index }\n            ghost: { source: checkout }\n",
			want:   "binds an input not declared by",
		},
		{
			name:   "a range on a source other than explicit-range is refused",
			inputs: "        item: { kind: files }\n",
			bind:   "            item: { source: git-index, range: explicit }\n",
			want:   "uses `range` with a source other than `explicit-range`",
		},
		{
			name:   "explicit-range without range: explicit is refused",
			inputs: "        item: { kind: files }\n",
			bind:   "            item: { source: explicit-range }\n",
			want:   "uses `explicit-range` without `range: explicit`",
		},
		{
			name:   "a fallback on a source other than push-updates is refused",
			inputs: "        item: { kind: files }\n",
			bind:   "            item: { source: git-index, fallback: refs/heads/main }\n",
			want:   "uses `fallback` with a source other than `push-updates`",
		},
		{
			name:   "an invalid push-update fallback ref is refused",
			inputs: "        item: { kind: commit-range }\n",
			bind:   "            item: { source: push-updates, fallback: \"-bad\" }\n",
			want:   "declares an invalid push-update fallback ref",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := "schema: rhino/repo-config/v2\ngates:\n  entries:\n    - id: typed-gate\n      type: check\n" +
				"      inputs:\n" + test.inputs + command +
				"      run-on:\n        manual:\n          bind:\n" + test.bind
			_, err := config.Parse(text)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want refusal containing %q", err, test.want)
			}
		})
	}
}
