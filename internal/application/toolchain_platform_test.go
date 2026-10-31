package application

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/adapters/config"
)

// The platform names a provision declaration uses are the contract's names:
// the Rust oracle reads std::env::consts::OS, which says macos where Go says
// darwin. These tests run the provision path for a darwin host on any machine,
// because the host's GOOS is an input and not read from the running process.

// scenarioPlatformToolchain is the fixture of the behaviour scenario "Toolchain
// provisioning plans as one JSON object", with one vector per platform.
const scenarioPlatformToolchain = `schema: rhino/repo-config/v2
toolchains:
  entries:
    - id: planned-tool
      executable: planned-tool
      probe: [--version]
      required: false
      provision:
        - {platform: linux, executable: planned-installer, arguments: [install]}
        - {platform: macos, executable: planned-installer, arguments: [install]}
        - {platform: windows, executable: planned-installer, arguments: [install]}
`

const linuxOnlyToolchain = `schema: rhino/repo-config/v2
toolchains:
  entries:
    - id: linux-tool
      executable: linux-tool
      probe: [--version]
      provision: [{platform: linux, executable: linux-installer}]
`

const macosOnlyToolchain = `schema: rhino/repo-config/v2
toolchains:
  entries:
    - id: mac-tool
      executable: mac-tool
      probe: [--version]
      provision: [{platform: macos, executable: mac-installer}]
`

const platformSpecificToolchain = `schema: rhino/repo-config/v2
toolchains:
  entries:
    - id: split-tool
      executable: split-tool
      probe: [--version]
      provision:
        - {platform: linux, executable: linux-installer}
        - {platform: macos, executable: macos-installer}
`

// darwinRecorder answers every launch with a clean status and keeps the
// executable of each one, so a test can see which vector the host selected.
type darwinRecorder struct {
	executables []string
}

func (r *darwinRecorder) Run(launch ToolchainLaunch) (ToolchainResult, error) {
	r.executables = append(r.executables, launch.Executable)
	return ToolchainResult{}, nil
}

func parseProvisionFixture(t *testing.T, text string) *config.Document {
	t.Helper()
	document, err := config.Parse(text)
	if err != nil {
		t.Fatalf("parse = %v", err)
	}
	return document
}

func TestPlatformNameUsesTheContractSpelling(t *testing.T) {
	for goos, want := range map[string]string{
		"darwin":    "macos",
		"linux":     "linux",
		"windows":   "windows",
		"freebsd":   "freebsd",
		"netbsd":    "netbsd",
		"openbsd":   "openbsd",
		"dragonfly": "dragonfly",
		"solaris":   "solaris",
		"illumos":   "illumos",
		"android":   "android",
		"ios":       "ios",
		"aix":       "aix",
		// Go targets the oracle has no platform name for pass through unchanged.
		"js":     "js",
		"plan9":  "plan9",
		"wasip1": "wasip1",
	} {
		if got := platformName(goos); got != want {
			t.Errorf("platformName(%q) = %q, want %q", goos, got, want)
		}
	}
}

func TestProvisionPlansTheMacosVectorOnADarwinHost(t *testing.T) {
	document := parseProvisionFixture(t, scenarioPlatformToolchain)
	outcome := provisionToolchains(document, nil, "darwin", false, JSON)
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, `"count":1`) ||
		!strings.Contains(outcome.Stdout, `"toolchains":["planned-tool"]`) {
		t.Fatalf("darwin plan = %+v", outcome)
	}
}

func TestProvisionAppliesTheMacosVectorOnADarwinHost(t *testing.T) {
	document := parseProvisionFixture(t, platformSpecificToolchain)
	recorder := &darwinRecorder{}
	outcome := provisionToolchains(document, recorder, "darwin", true, Text)
	if outcome.ExitCode != 0 || len(recorder.executables) != 1 || recorder.executables[0] != "macos-installer" {
		t.Fatalf("darwin apply = %+v, launched %v", outcome, recorder.executables)
	}
}

func TestProvisionRefusesALinuxOnlyDeclarationOnADarwinHostNamingMacos(t *testing.T) {
	document := parseProvisionFixture(t, linuxOnlyToolchain)
	outcome := provisionToolchains(document, nil, "darwin", false, Text)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "has no provision declaration for `macos`") ||
		strings.Contains(outcome.Stderr, "darwin") {
		t.Fatalf("darwin refusal = %+v", outcome)
	}
}

func TestProvisionOnALinuxHostKeepsItsVectors(t *testing.T) {
	document := parseProvisionFixture(t, scenarioPlatformToolchain)
	outcome := provisionToolchains(document, nil, "linux", false, Text)
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "planned 1 declared toolchains") {
		t.Fatalf("linux plan = %+v", outcome)
	}
	outcome = provisionToolchains(parseProvisionFixture(t, macosOnlyToolchain), nil, "linux", false, Text)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stderr, "has no provision declaration for `linux`") {
		t.Fatalf("linux refusal of a macos-only declaration = %+v", outcome)
	}
}
