package vault_test

import (
	"os"
	"strings"
	"testing"
)

// runnerWiring names, for each adapter's runner source, the text that proves
// the runner drives this corpus. The compliance check proves the shared
// registry binds the corpus; this check proves each runner uses the registry.
// Without it a runner could stop calling RunSuite and the corpus would still
// bind, and no test would fail.
var runnerWiring = []struct {
	file     string
	required []string
}{
	{
		file: "../../../internal/application/environment_vault_behaviour_test.go",
		required: []string{
			"vault.RunSuite(t, environmentVaultCorpus,",
			"environment-vault-validation.feature",
		},
	},
	{
		file: "../../../internal/adapters/runtime/environment_vault_behaviour_test.go",
		required: []string{
			"vault.RunSuite(t, environmentVaultCorpus,",
			"environment-vault-validation.feature",
		},
	},
	{
		file:     "../behaviours_test.go",
		required: []string{"vault.WithDriver("},
	},
	{
		file:     "../steps_test.go",
		required: []string{"vault.Bindings()"},
	},
}

// missingText returns every required text the source lacks.
func missingText(source string, required []string) []string {
	var missing []string
	for _, text := range required {
		if !strings.Contains(source, text) {
			missing = append(missing, text)
		}
	}
	return missing
}

// TestEveryRunnerDrivesTheCorpus fails when any adapter's runner stops calling
// the shared registry on this corpus, naming the runner and the missing call.
func TestEveryRunnerDrivesTheCorpus(t *testing.T) {
	for _, runner := range runnerWiring {
		source, err := os.ReadFile(runner.file)
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range missingText(string(source), runner.required) {
			t.Errorf("%s does not contain %q, so it does not run the corpus", runner.file, text)
		}
	}
}

// TestWiringRejectsARunnerThatSkipsTheCorpus proves the check fires on a runner
// with the test function and no call into the registry.
func TestWiringRejectsARunnerThatSkipsTheCorpus(t *testing.T) {
	skipped := "func TestEnvironmentVaultValidationAtUnitAdapter(t *testing.T) {\n\tt.Skip()\n}\n"
	if missing := missingText(skipped, runnerWiring[0].required); len(missing) == 0 {
		t.Fatal("a runner that never calls RunSuite was accepted")
	}
}
