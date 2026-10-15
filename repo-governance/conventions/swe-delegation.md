---
description: >-
  Requires a session doing coding work, which includes every create, modify, update, or delete of code or declarative
  configuration, to dispatch the fitting swe agent rather than doing the work in the main thread. Defines coding work
  and names the two exceptions.
when_to_use: >-
  Use before creating, modifying, updating, or deleting code or declarative configuration in a session, or before
  debugging, reviewing, or testing code.
---

# SWE Delegation

When a session creates, modifies, updates, or deletes code or declarative configuration, or debugs, reviews, or tests
code, it dispatches the fitting `swe-*` agent the repository holds rather than doing the work in the main thread.

## Coding Work

Coding work is two kinds of content:

- **Code:** applications, libraries, scripts, tests, and shell or other script code that carries behaviour.
- **Declarative configuration:** settings held as data, such as JSON or YAML files and package manifests.

Documents, rules, plans, and agent or skill definitions in Markdown are not coding work; their own families handle them.

## Which Agent

| The work                                                      | Agent                  |
| ------------------------------------------------------------- | ---------------------- |
| a goal spanning several coding tasks or agents                | `swe-orchestrator`     |
| a module boundary, a new dependency, or a tradeoff to decide  | `swe-architect`        |
| new or changed behaviour, configuration, or findings to apply | `swe-developer`        |
| a failing type check, lint run, or test                       | `swe-debugger`         |
| a static review of changed code against the adopted standards | `swe-reviewer`         |
| judging a running web interface                               | `swe-web-tester`       |
| judging first use of a web interface or a command-line tool   | `swe-usability-tester` |
| judging a running request-based interface                     | `swe-api-tester`       |
| judging infrastructure as applied                             | `swe-infra-tester`     |
| cutting or deploying a release, or repinning a pinned tool    | `swe-releaser`         |

Each agent's definition, in the repository's agent index, states its modes and hand-offs.

## Family Fixers

A family fixer that finds a repair changing declarative configuration or code does not write that change. It decides the
repair, names `swe-developer` as its repairer in its report, and the fixer or its caller dispatches `swe-developer` to
make the change. The fixer then confirms the change landed.

## Exceptions

The main thread may do coding work itself only:

1. **A harness with no subagent support,** where no agent can be dispatched.
2. **A tool repin** of a pinned upstream tool, such as HIPPO, RHINO, or FERRET, in a repository that holds no
   `swe-releaser`.

A session that uses an exception says so in its report. The harness exception gives its reason. The repin exception
needs no reason, because this convention names it.

## Why

The swe agents carry the adopted standards and test-first discipline. Execution-tier agents run on a cheaper model when
the repository maps one to that tier, and the files they read stay out of the main session's context, which keeps the
reasoning that follows clear, as Agent Workflow Orchestration explains. The root [AGENTS.md](../../AGENTS.md) repeats
the rule in one line, because a contributor must see it before opening any file.

## Enforcement

Unenforced by decision: whether a session delegated is visible only in its transcript, which no repository check reads,
and a check of changed paths cannot tell which agent wrote them. Review and the session's own report carry it.
