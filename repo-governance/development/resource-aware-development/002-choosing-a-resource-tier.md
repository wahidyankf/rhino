---
description: >-
  Defines the three HIPPO resource tiers and the bounded cost of a tier that is too low or too high.
when_to_use: >-
  Use when choosing the `--resource-tier` of a local compute run guarded by `./hippo`.
---

# Choosing a Resource Tier

Read the [resource rule](../resource-aware-development.md) first. That rule states the obligation; this companion
defines each tier and what a wrong choice costs.

## The Tiers

A resource tier is a reservation, not a limit: HIPPO does not cap the process, so classify by what the work really
needs. `light` is one narrow static check (a linter, a formatter, one validator). `standard` is an ordinary check, one
project's tests, or a writer. `heavy` is a full build, a whole suite, or a quality gate.

## A Wrong Tier

A tier that is too low lets real use exceed the reservation: the host reaches pressure and HIPPO sheds ephemeral work. A
tier that is too high, or `heavy` by habit, holds capacity other runs need and starves the queue. Never choose a tier to
be admitted faster.

## Enforcement

**Enforcement: unenforced by decision**, because the right tier depends on what the work really needs, which only the
person or agent running it knows. The consumer check confirms that every HIPPO command names a tier, never which tier is
right.
