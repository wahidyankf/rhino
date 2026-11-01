package application_test

import (
	"strings"
	"testing"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
)

// TestExecuteAnswersHelpFromTheCommandTable proves --help is answered with the
// help text and a clean exit, whatever the tree holds.
func TestExecuteAnswersHelpFromTheCommandTable(t *testing.T) {
	outcome := application.Execute(newMemoryTree("/root"), args("--help"), nil)
	if outcome.ExitCode != 0 || !strings.Contains(outcome.Stdout, "Commands:") {
		t.Fatalf("help was not answered cleanly: %+v", outcome)
	}
}

// rootedTree answers RootedAt with its own separate tree, so a run that ignores
// the named root reads the configuration of the tree it was given instead.
type rootedTree struct {
	*memoryTree
	sub application.Tree
}

func (r rootedTree) RootedAt(path string) (application.Tree, error) {
	if path != "sub" {
		return nil, errTest("`--root " + path + "` names nothing usable")
	}
	return r.sub, nil
}

// TestExecuteEntersARootThatItNames proves a `--root` naming a usable directory
// is entered: the configuration is read from that root, so a run that ignores the
// root, and reads the empty tree it was given, is refused for want of a config.
func TestExecuteEntersARootThatItNames(t *testing.T) {
	sub := newMemoryTree("/root/sub")
	sub.files["repo-config.yml"] = metadataAgentConfig
	tree := rootedTree{memoryTree: newMemoryTree("/root"), sub: sub}
	outcome := application.Execute(tree, args("--root", "sub", "metadata", "validate"), nil)
	if outcome.ExitCode != 0 {
		t.Fatalf("a usable root was refused: %+v", outcome)
	}
}

// TestParseRefusesADirectoryFlagWithNoValue proves a `--directory` flag with no
// value after it refuses the invocation, naming the flag, rather than inspecting
// an empty path.
func TestParseRefusesADirectoryFlagWithNoValue(t *testing.T) {
	_, refusal := application.Parse(args("governance", "directory-map", "validate", "--directory"))
	if refusal == nil {
		t.Fatal("a directory flag with no value was accepted")
	}
	if refusal.Code != domain.ArgsIncomplete || refusal.Message != "`--directory` needs a value" {
		t.Fatalf("refused with %v %q, want ArgsIncomplete naming the flag", refusal.Code, refusal.Message)
	}
}

// TestNamingRefusesASurfaceGlobThatDoesNotCompile proves a naming surface whose
// glob the matcher cannot read refuses the run by its key, rather than governing
// no file quietly.
func TestNamingRefusesASurfaceGlobThatDoesNotCompile(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = "schema: rhino/repo-config/v2\npolicies:\n  markdown:\n    naming:\n" +
		"      surfaces:\n        - glob: \"docs/[a\"\n          style: kebab-case\n      exempt: []\n"
	outcome := application.Execute(tree, args("md", "naming", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "md-naming.surfaces: ") {
		t.Fatalf("an uncompilable naming glob was not refused: %+v", outcome)
	}
}

// TestMetadataRefusesAMalformedScanExclusion proves a metadata run that reads the
// shared scan exclusions refuses a malformed section instead of reading it as an
// empty list.
func TestMetadataRefusesAMalformedScanExclusion(t *testing.T) {
	tree := newMemoryTree("/root")
	tree.files["repo-config.yml"] = metadataAgentConfig + "scan:\n  exclude-directories: x\n"
	outcome := application.Execute(tree, args("metadata", "validate"), nil)
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "scan.exclude-directories names no directory") {
		t.Fatalf("malformed scan exclusions not refused: %+v", outcome)
	}
}

// TestToolchainRefusesAnEntryThatIsNotAMapping proves a toolchain entry written
// as a bare scalar refuses the declaration.
func TestToolchainRefusesAnEntryThatIsNotAMapping(t *testing.T) {
	tree := toolchainTree("toolchains:\n  entries:\n    - formatter\n")
	outcome := runToolchain(tree, &recordingToolchainRunner{}, "toolchain", "validate")
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "toolchains.entries names no entries") {
		t.Fatalf("a scalar toolchain entry was not refused: %+v", outcome)
	}
}

// TestToolchainRefusesAnUnknownParser proves a parser name the build does not know
// refuses the entry, naming the parser it was given.
func TestToolchainRefusesAnUnknownParser(t *testing.T) {
	tree := toolchainTree("toolchains:\n  entries:\n    - id: formatter\n      executable: formatter\n      probe: [--version]\n" +
		"      parser: nope\n      version: 1.2.3\n")
	outcome := runToolchain(tree, &recordingToolchainRunner{}, "toolchain", "validate")
	if outcome.ExitCode != 2 || !strings.Contains(outcome.Stdout+outcome.Stderr, "names unknown parser `nope`") {
		t.Fatalf("an unknown parser was not refused: %+v", outcome)
	}
}
