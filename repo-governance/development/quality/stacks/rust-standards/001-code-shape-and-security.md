---
description: >-
  Fixes how Rust code is organised and typed, from modules by domain to newtypes, state enums, and sealed traits, and
  the Rust security rules for secrets, cryptography, transport security, and queries.
when_to_use: >-
  Use when organising a Rust crate's modules, shaping a Rust domain type or conversion, or handling secrets,
  cryptography, transport security, or database queries in Rust.
---

# Code Shape and Security

## Code Shape

- Modules follow the domain; no technical layer spans domains.
- Every type derives `Debug`, so any value can appear in a diagnostic; a type holding a secret redacts it there.
- A conversion between types implements `From`, or `TryFrom` when it can fail, rather than an ad hoc method.
- A parameter that has structure takes a type expressing it, never a string the callee parses.

## Domain Types

Layering follows Hexagonal Architecture and Functional Core, Imperative Shell. Inside it:

| Concept       | Rust shape                                                                               |
| ------------- | ---------------------------------------------------------------------------------------- |
| value object  | a newtype with a private field and a constructor returning `Result`                      |
| identifier    | a newtype per entity over the raw key                                                    |
| state         | an enum whose variants carry only that state's data, not a status beside optional fields |
| aggregate     | private fields, changed only by methods that check invariants first                      |
| domain event  | a variant of an event enum                                                               |
| port          | a trait declared inward and implemented in an adapter                                    |
| closed family | a sealed trait that no outside type can implement                                        |

## Security

- A secret is held in a wrapper type whose debug and display output redacts it, and it is never logged.
- Cryptography, transport security included, uses Rust cryptography libraries: no primitive is written by hand, and
  nothing binds to a system C cryptography library.
- A database query is checked at compile time, and never assembled from strings.

Example: secrecy for the secret wrapper, rustls for transport security, ring for primitives, and SQLx for checked
queries.
