# Why RHINO exists

A repository accumulates rules faster than it accumulates ways to check them.
Instruction files grow past the length anyone reads. Directory maps drift from
the directories they describe. Links rot. Diagrams pick colours that some
readers cannot distinguish. Coding harnesses acquire their own copies of one
canonical instruction, and the copies diverge.

None of these are bugs. All of them are the kind of decay that a reviewer
notices on a good day and misses on a busy one.

## The problem RHINO was extracted from

RHINO's validators started as part of a larger repository, written for that
repository. They worked. Then a second repository wanted them, and the trouble
started — not with the checking logic, which was fine, but with everything the
logic knew without being told.

It knew which directories held governed documents. It knew how long an
instruction file could be. It knew which harness directories existed and what
they were called. Every one of those was a fact about one repository, compiled
into a tool that was about to be pointed at a different one.

The failure mode is worse than a crash. A validator with a wrong assumption
does not fail — it passes. The surface it was told to check does not exist in
the new repository, so it checks nothing, finds nothing, and reports a clean
run. Everyone concludes the repository is in good shape.

So RHINO exists to be the same checks with **nothing known in advance**. Every
value it enforces is declared by the repository being checked. See [why RHINO
holds no defaults](./holding-no-defaults.md) for what that buys and costs.

## What it checks, and why those five

Five validators, chosen because each catches a decay that is invisible until
someone is already lost.

**Word budgets.** An instruction file nobody finishes reading is an instruction
file that is not in force. A declared limit turns "this is getting long" into a
number the repository agreed to.

**Directory maps.** A reader who opens a directory should see what is in it
without listing the filesystem. A map that has drifted from the tree is worse
than no map, because it is believed.

**Internal links.** A link that does not resolve is a reference the reader
cannot follow, and rot is silent — the page still renders.

**Mermaid diagrams.** A label too long to read and a palette that collapses
under colour vision deficiency both produce a diagram that technically exists
and practically does not.

**Harness parity.** This is the one that motivated the rest. When several
coding harnesses each carry their own copy of one instruction, one skill set,
and one agent set, the copies drift — and the drift is invisible, because each
copy is internally consistent. Parity reconciles every harness against one
canon and reports exactly where they disagree.

## What it is not

**Not a linter.** It has no opinion about your prose, your code, or your
formatting. It checks structural claims a repository makes about itself.

**Not a fixer.** It reads and reports. Nothing it finds is repaired
automatically, because every one of these findings has more than one correct
resolution and the choice belongs to a maintainer.

**Not a service.** No daemon, no network, no state between runs. A single
binary, a directory walk, and an exit code. See [why RHINO only
reads](./reading-only.md).

## Related

- [Why RHINO holds no defaults](./holding-no-defaults.md)
- [What a finding means](./what-a-finding-means.md)
- [Why RHINO only reads](./reading-only.md)
