# adapters/cli

Standard streams, signals, and the exit code. Standard input is read only when a parsed leaf selects it, so an
ordinary run never blocks on a terminal; a closed reader maps to `141`, never to a finding.
