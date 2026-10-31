// Mutation gates: a declared formatter-style command that may change files,
// run against an isolated snapshot rather than the mutable working tree.
//
// A `type: mutation` gate pairs a local `apply-index` behaviour with a
// pull-request `verify-clean` replay. The gate run resolves the typed inputs,
// projects the command, and classifies the result; the process boundary that
// owns the snapshot and every write is the MutationRunner the entry point
// supplies. Read-only entry points never receive it, so they fail closed.
package application

import "fmt"

// MutationLaunch is one projected mutation: the declared argv and environment,
// the repository it belongs to, the paths the Git index or the reviewed range
// selected, and the candidate revision a disposable replay checks out.
type MutationLaunch struct {
	Argv          []string
	Directory     string
	Environment   map[string]string
	SelectedPaths []string
	// Surface is the lifecycle surface that selected the mutation. The mutator
	// sees it as RHINO_GATE_SURFACE, as a check child does.
	Surface string
	// Revision is the candidate commit a pull-request replay checks out; it
	// is empty on the local surface.
	Revision string
}

// MutationOutcome is what one mutation run produced. Code is the mutator's
// exit status; Changes names the repository-relative paths it changed, and
// Divergences names the paths it changed whose working-tree copy held
// unstaged edits, which were preserved rather than overwritten.
type MutationOutcome struct {
	Code        int
	Changes     []string
	Divergences []string
}

// MutationRunner is the only capability that may turn a declared mutation
// into a write. A caller that is not allowed to write never receives one.
type MutationRunner interface {
	ApplyIndex(launch MutationLaunch) (MutationOutcome, error)
	VerifyClean(launch MutationLaunch) (MutationOutcome, error)
}

// closedMutations fails every mutation closed, the way the oracle's
// NoMutationRunner does: a caller that wired no boundary is told which one it
// lacks, and nothing is written.
type closedMutations struct{}

func (closedMutations) ApplyIndex(MutationLaunch) (MutationOutcome, error) {
	return MutationOutcome{}, fmt.Errorf("this entry point has no index mutation boundary")
}

func (closedMutations) VerifyClean(MutationLaunch) (MutationOutcome, error) {
	return MutationOutcome{}, fmt.Errorf("this entry point has no disposable pull-request replay boundary")
}
