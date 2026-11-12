package runtime

import (
	"reflect"
	"testing"
)

// TestChildEnvironmentKeepsOnlyWhatTheProviderNeeds proves the filter keeps the
// session key, PATH, HOME, and ordinary variables, and drops every other BW_*
// variable, the forge and cloud tokens, and the Terraform and AWS variables.
func TestChildEnvironmentKeepsOnlyWhatTheProviderNeeds(t *testing.T) {
	parent := []string{
		"PATH=/bin", "HOME=/home/user", "BW_SESSION=key", "BW_CLEANEXIT=1", "BW_RESPONSE=1",
		"BW_QUIET=1", "BW_RAW=1", "GITHUB_TOKEN=t", "GH_TOKEN=t", "CLOUDFLARE_API_TOKEN=t",
		"TF_VAR_x=t", "AWS_SECRET_ACCESS_KEY=t", "LANG=en_US.UTF-8", "TMPDIR=/tmp",
	}
	want := []string{"PATH=/bin", "HOME=/home/user", "BW_SESSION=key", "LANG=en_US.UTF-8", "TMPDIR=/tmp"}
	if got := childEnvironment(parent); !reflect.DeepEqual(got, want) {
		t.Fatalf("childEnvironment = %v, want %v", got, want)
	}
}

// TestDecodeStrictReadsOneValueOnly proves an answer must be exactly one JSON
// value, with no trailing content and no empty output.
func TestDecodeStrictReadsOneValueOnly(t *testing.T) {
	var target map[string]string
	if err := decodeStrict([]byte(`{"status":"unlocked"}`), &target); err != nil {
		t.Fatalf("one value refused: %v", err)
	}
	for _, data := range []string{`{"status":"unlocked"} {}`, `{"status":"unlocked"} x`, ``, `{`} {
		var again map[string]string
		if err := decodeStrict([]byte(data), &again); err == nil {
			t.Fatalf("decodeStrict(%q) accepted an answer that is not one value", data)
		}
	}
}

// TestKeepChildVariableNamesEachRule pins each rule of the child filter by name.
func TestKeepChildVariableNamesEachRule(t *testing.T) {
	cases := map[string]bool{
		"BW_SESSION": true, "PATH": true, "HOME": true, "BW_QUIET": false, "GH_TOKEN": false,
		"GITHUB_TOKEN": false, "CLOUDFLARE_X": false, "TF_VAR_x": false, "AWS_REGION": false, "LANG": true,
	}
	for name, want := range cases {
		if got := keepChildVariable(name); got != want {
			t.Fatalf("keepChildVariable(%q) = %t, want %t", name, got, want)
		}
	}
}
