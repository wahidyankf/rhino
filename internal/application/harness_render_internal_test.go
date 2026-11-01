package application

import "testing"

// Config validation refuses these declarations before rendering runs, so the
// render helpers' own refusals are exercised here directly. Each test asserts
// the reason the helper returns and that it leaves the output it was given
// untouched.

func TestHarnessAddFileRefusesADuplicatePath(t *testing.T) {
	desired := map[string]string{}
	if reason, failed := harnessAddFile(desired, "adapters/alpha/a.md", "x"); failed {
		t.Fatalf("first write refused: %s", reason)
	}
	reason, failed := harnessAddFile(desired, "adapters/alpha/a.md", "y")
	if !failed || reason != "generated adapter path `adapters/alpha/a.md` is duplicated" {
		t.Fatalf("duplicate path not refused: %q, %v", reason, failed)
	}
	if desired["adapters/alpha/a.md"] != "x" {
		t.Fatalf("refused write changed the first output: %q", desired["adapters/alpha/a.md"])
	}
}

func TestHarnessRouteNeedsExactlyOnePlaceholder(t *testing.T) {
	route, reason, failed := harnessRoute("Read {path}.", ".agents/a.md")
	if failed || reason != "" || route != "Read .agents/a.md." {
		t.Fatalf("single placeholder route = %q, %q, %v", route, reason, failed)
	}
	_, reason, failed = harnessRoute("Read {path} and {path}.", ".agents/a.md")
	if !failed || reason != "adapter route does not declare exactly one `{path}` placeholder" {
		t.Fatalf("two-placeholder route not refused: %q, %v", reason, failed)
	}
}

func TestHarnessAddTranslationRefusesAnAbsentMemberAdd(t *testing.T) {
	fields := map[string]*harnessRenderField{}
	translation := &harnessTranslation{when: "always", field: "access", absentMembers: []string{"read-files"}}
	reason, failed := harnessAddTranslation(fields, translation)
	if !failed || reason != "adapter output cannot add absent members at `access`" {
		t.Fatalf("absent-member add not refused: %q, %v", reason, failed)
	}
	if len(fields) != 0 {
		t.Fatalf("refused translation wrote a field: %v", fields)
	}
}

func TestHarnessAddDispatchesRefusesAConflictingParent(t *testing.T) {
	scalar := "value"
	cases := []struct {
		name   string
		fields map[string]*harnessRenderField
		field  string
	}{
		{
			name:   "an allow map under a field that holds a scalar refuses",
			fields: map[string]*harnessRenderField{"model": {scalar: &scalar}},
			field:  "model.key",
		},
		{
			name: "an allow map key that is already held refuses",
			fields: map[string]*harnessRenderField{"policy": {
				entries: map[string]harnessEntryValue{"key": {}},
				isEntry: true,
			}},
			field: "policy.key",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dispatches := &harnessDispatches{field: tc.field, format: harnessAllowMap}
			reason, failed := harnessAddDispatches(tc.fields, dispatches, []string{"tidy"})
			want := "adapter output conflicts at `" + tc.field + "`"
			if !failed || reason != want {
				t.Fatalf("dispatch conflict not refused: %q, %v", reason, failed)
			}
		})
	}
}

func TestRenderHarnessAdapterRefusesAFixedOrListFieldThatRepeatsAnIdentity(t *testing.T) {
	profile := &harnessProfile{id: "alpha"}
	source := harnessSource{
		path: ".agents/agents/reviewer.md",
		kind: "agent",
		metadata: &harnessMetadata{
			name:        "reviewer",
			description: "Review changes.",
			lists:       map[string][]string{"skills": {"tidy"}},
		},
	}
	base := harnessAdapter{
		path:       "adapters/alpha/agents/{name}.md",
		format:     "front-matter",
		routeField: "body",
		route:      "Read {path}.",
		identity:   map[string]string{"title": "name"},
	}
	fixed := base
	fixed.fixed = map[string]string{"title": "x"}
	list := base
	list.lists = map[string]string{"title": "skills"}
	for name, adapter := range map[string]harnessAdapter{"fixed": fixed, "list": list} {
		adapter := adapter
		_, reason, failed := renderHarnessAdapter(profile, &adapter, &source, map[string]string{})
		if !failed || reason != "adapter output repeats field `title`" {
			t.Fatalf("%s field repeating an identity not refused: %q, %v", name, reason, failed)
		}
	}
}
