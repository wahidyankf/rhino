---
description: >-
  Defines the HIPPO resource tiers, the optional last-resort tier and settings, and the bounded cost of a tier that is too
  low or too high.
when_to_use: >-
  Use when choosing the `--resource-tier` of a local compute run guarded by `./hippo`, or when declaring an optional tier
  or setting in a machine file.
---

# Choosing a Resource Tier

Read the [resource rule](../resource-aware-development.md) first. That rule states the obligation; this companion
defines each tier and what a wrong choice costs.

## The Tiers

A resource tier is a reservation, not a limit: HIPPO does not cap the process, so classify by what the work really
needs. `light` is one narrow static check (a linter, a formatter, one validator). `standard` is an ordinary check, one
project's tests, a writer, or one release build. `heavy` is a whole suite or a quality gate that needs more than
`standard`.

## Escalation

Pick the smallest tier that fits, and move up only on measured need. Measure CPU with `hippo history --json`: divide
`childCpuSeconds` by `childWallSeconds` for the average number of CPUs a run kept busy, and compare it with the tier's
maximum. History does not record memory, so measure peak memory with the build tool or the operating system. A
`pressure-shed` outcome shows host pressure, not a need for a larger tier.

## A Wrong Tier

A tier that is too low lets real use exceed the reservation: the host reaches pressure and HIPPO sheds ephemeral work. A
tier that is too high, or `heavy` by habit, holds capacity other runs need and starves the queue. Never choose a tier to
be admitted faster.

## Optional Tier and Settings

`very-heavy` is an optional last resort above `heavy`. Declare it only in the machine file of a machine that can hold
its maximum. A run that names `very-heavy` where the loaded file does not declare it exits `2` with
`hippo.args.invalid`.

`coordination.countSwap` is optional and off by default. It applies only to a schema-3 Linux host, and never to
zram-backed swap.

An older binary refuses a file that declares either setting, with exit `125`. Declare one only after every consumer on
that machine is repinned and reports the version its lock pins. Rollback is deleting the block or key.

## Enforcement

**Enforcement: unenforced by decision**, because the right tier, and whether a machine can hold a tier, depend on what
the work and the host need, which only the person or agent running it knows. The consumer check confirms that every
HIPPO command names a tier and that the tracked example declares only the three required tiers, never which tier is
right.
