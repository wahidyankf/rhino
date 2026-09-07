# Specifications

This directory is the canonical statement of RHINO's observable behaviour.
Where the documentation and these specifications disagree, these win, and the
disagreement is a bug worth reporting.

`specs/` sits at the repository root rather than under a project directory
because this repository is the tool: there is no second project to disambiguate
from.

## Directory Map

- [behaviours/](behaviours/README.md) — the Gherkin corpus, one feature file per
  validator plus the shared CLI contract.
