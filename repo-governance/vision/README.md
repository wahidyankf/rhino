# Vision

RHINO is a repository-hygiene validator that owns no repository's answers.

The future it exists to create is one where a maintainer running several repositories enforces the same documentation contract in all of them without writing the validator again. That is not hypothetical: the same idea had been implemented three times in three languages before this repository existed, each time because the previous implementation had one repository's answers compiled into it and a second repository could not adopt it.

So the vision is a tool that is genuinely adoptable. Every value a repository could reasonably decide differently arrives from that repository's own declaration. A repository that declares nothing gets a configuration error, never a borrowed assumption from whichever repository the author happened to have open. The line is whether a consumer could hold a different opinion and be right: which diagram syntaxes the parser understands is tool behaviour, which colours are acceptable is policy.

Three properties follow from that and are not negotiable.

**It is safe to point at anything.** Read-only, network-free, process-free, path-contained. A validator a maintainer hesitates to run on an unfamiliar tree is a validator that does not get run.

**Its findings are trustworthy.** Exit codes distinguish clean, findings, and an unusable invocation, so a caller can act on the code without knowing which subcommand ran. A check that cannot run reports that it could not, rather than reporting a clean tree it never read.

**Its contract does not move under its consumers.** Commands, flags, exit codes, and the shape of `version --json` are public. Adding is free; moving is a major version. A repository pins a release by version, commit, and checksum, and that pin means the same thing tomorrow.

The measure of success is a second repository unlike this one adopting RHINO by writing configuration, and no code change.

## Directory Map

- This README has no siblings.
