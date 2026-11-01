# application

The invocation contract: the closed command table and parser (`cli.go`), execution and dispatch (`execute.go`), the
shared report shape (`report.go`), path selection (`scan.go`), and validator cores (`validators.go` for emoji,
`naming.go` for filenames, `frontmatter.go` for front matter, `heading.go` for heading hierarchy, `link.go` for internal
links, `mermaid.go` for diagrams, `metadata.go` for path-selected metadata schemas, `readmeindex.go` for README index
completeness, `license.go` for configured license paths, identifiers, and digests, `vendor.go` for declared portable
roots, forbidden terms, and exact exceptions, `layers.go` for declared governance layers and their categories,
`traceability.go` for declared artifacts and their declared local links, `qualitygates.go` for the declared quality-gate
layout, headings, verdicts, and cycle ceiling, `wordbudget.go` for declared surfaces against their declared word
budgets, `directorymap.go` for the declared directory-map trees and their sibling-covering README maps, `environment.go`
for the declared local environment setup: redacted validation plus the plan-then-write init, backup, and restore leaves,
`toolchain.go` for the declared toolchain probes and the plan-then-apply provision vectors).

Validators here stay read-only, network-free, process-free, and path-contained. Declared gate dispatch launches
processes through the consumer-declared `Launcher`; declared toolchain validate and applied provision start only
declared no-shell vectors through the consumer-declared `ToolchainRunner`; declared environment init, backup, and
restore are the one path that writes, through the consumer-declared `EnvironmentStore`. Every leaf in the closed command
table has a routed core, and a leaf without one panics rather than reporting a tree it never checked.
