package runtime

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestChildEnvironmentAllowsOnlyListedVariables proves the child receives the
// allowlisted variables, the locale and XDG variables, and nothing else. A forge
// token, a cloud credential, a Terraform or AWS variable, an unlisted credential,
// and every BW_* variable except the session key never reach it.
func TestChildEnvironmentAllowsOnlyListedVariables(t *testing.T) {
	parent := []string{
		"PATH=/bin", "HOME=/opt/fixture", "BW_SESSION=key", "BW_CLEANEXIT=1", "BW_RESPONSE=1",
		"BW_QUIET=1", "BW_RAW=1", "GITHUB_TOKEN=t", "GH_TOKEN=t", "CLOUDFLARE_API_TOKEN=t",
		"TF_VAR_x=t", "AWS_SECRET_ACCESS_KEY=t", "EXAMPLE_API_KEY=t", "LANG=en_US.UTF-8",
		"LC_ALL=en_US.UTF-8", "TMPDIR=/tmp", "XDG_CONFIG_HOME=/opt/fixture/.config", "TZ=UTC",
		"HTTPS_PROXY=http://proxy.example.invalid:3128", "https_proxy=http://proxy.example.invalid:3128",
		"SSL_CERT_FILE=/certs/ca.pem", "SystemRoot=C:\\Windows",
	}
	want := []string{
		"PATH=/bin", "HOME=/opt/fixture", "BW_SESSION=key", "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8",
		"TMPDIR=/tmp", "XDG_CONFIG_HOME=/opt/fixture/.config", "TZ=UTC",
		"HTTPS_PROXY=http://proxy.example.invalid:3128", "https_proxy=http://proxy.example.invalid:3128",
		"SSL_CERT_FILE=/certs/ca.pem", "SystemRoot=C:\\Windows",
	}
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

// TestKeepChildVariableNamesEachRule pins each rule of the child allowlist by name.
func TestKeepChildVariableNamesEachRule(t *testing.T) {
	cases := map[string]bool{
		"BW_SESSION": true, "PATH": true, "HOME": true, "TMPDIR": true, "LANG": true, "LC_CTYPE": true,
		"XDG_STATE_HOME": true, "TZ": true, "USER": true, "LOGNAME": true, "BITWARDENCLI_APPDATA_DIR": true,
		"NO_PROXY": true, "no_proxy": true, "NODE_EXTRA_CA_CERTS": true, "SSL_CERT_DIR": true,
		"ComSpec": true, "LOCALAPPDATA": true, "BW_QUIET": false, "BW_CLEANEXIT": false, "GH_TOKEN": false,
		"GITHUB_TOKEN": false, "CLOUDFLARE_X": false, "TF_VAR_x": false, "AWS_REGION": false,
		"EXAMPLE_API_KEY": false, "LCX": false, "XDGX": false,
	}
	for name, want := range cases {
		if got := keepChildVariable(name); got != want {
			t.Fatalf("keepChildVariable(%q) = %t, want %t", name, got, want)
		}
	}
}

// TestToVaultMarksALoginWithNoPasswordAsSo proves a login whose password the
// provider holds as null is marked, and a login holding an empty password is not.
func TestToVaultMarksALoginWithNoPasswordAsSo(t *testing.T) {
	var missing, empty bwItem
	if err := json.Unmarshal([]byte(`{"id":"1","name":"x","type":1,"login":{}}`), &missing); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"id":"2","name":"y","type":1,"login":{"password":""}}`), &empty); err != nil {
		t.Fatal(err)
	}
	if view := missing.toVault(); !view.NoPassword || view.Password != "" {
		t.Fatalf("a null password view = %+v", view)
	}
	if view := empty.toVault(); view.NoPassword || view.Password != "" || view.Kind != "login" {
		t.Fatalf("an empty password view = %+v", view)
	}
}
