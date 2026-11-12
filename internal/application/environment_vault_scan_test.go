package application

import (
	"reflect"
	"testing"
)

// TestWorkflowReadsResolvesEachContextRead pins how an expression body reads the
// secrets and vars contexts: literal keys, dynamic indexes, whole-context reads,
// and the reads that are not reads at all.
func TestWorkflowReadsResolvesEachContextRead(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []workflowRead
	}{
		{"an empty body reads nothing", "", nil},
		{"a dotted key is a literal read", "secrets.OTHER", []workflowRead{{kind: "secrets", name: "OTHER"}}},
		{"a bracket key is a literal read", `vars["OTHER"]`, []workflowRead{{kind: "vars", name: "OTHER"}}},
		{"a dynamic index is a dynamic read", "secrets[matrix.target]", []workflowRead{{kind: "secrets", dynamic: true}}},
		{"a context with no key is a whole-context read", "toJSON(secrets)", []workflowRead{{kind: "secrets"}}},
		{"a dot with no key after it is a whole-context read", "secrets.", []workflowRead{{kind: "secrets"}}},
		{"a context in another case is the same context", "SECRETS.OTHER", []workflowRead{{kind: "secrets", name: "OTHER"}}},
		{"a word after a dot is a property, not a context", "github.secrets.OTHER", nil},
		{"a longer word is not a context", "secrets-repo.OTHER", nil},
		{"a quoted literal is not read", "'secrets.OTHER'", nil},
		{"a doubled quote stays inside its literal", "'it''s secrets.OTHER'", nil},
		{"an unterminated literal runs to the end", "'secrets.OTHER", nil},
		{"a read after a literal is still read", "'a' && vars.OTHER", []workflowRead{{kind: "vars", name: "OTHER"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := workflowReads(tc.body); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("workflowReads(%q) = %#v, want %#v", tc.body, got, tc.want)
			}
		})
	}
}

// TestWorkflowReadLabelsEachFormOfRead pins the text a finding names each read by.
func TestWorkflowReadLabelsEachFormOfRead(t *testing.T) {
	cases := []struct {
		read workflowRead
		want string
	}{
		{workflowRead{kind: "secrets", name: "OTHER"}, "secrets.OTHER"},
		{workflowRead{kind: "vars", dynamic: true}, "vars[dynamic]"},
		{workflowRead{kind: "secrets"}, "secrets (whole context)"},
	}
	for _, tc := range cases {
		if got := tc.read.label(); got != tc.want {
			t.Fatalf("label(%#v) = %q, want %q", tc.read, got, tc.want)
		}
	}
}
