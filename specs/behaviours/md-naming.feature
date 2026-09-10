Feature: Markdown file-naming validation

  A repository declares which trees carry which filename style, and what joins an
  encoded directory prefix to a content name. RHINO holds no filename convention
  of its own, and the section is optional: a repository that declares nothing
  gets a refusal rather than somebody else's convention.

  Scenario: A kebab-case surface accepts a kebab-case name
    Given the repository declares the kebab-case naming surface "rules/**/*.md"
    And the repository contains:
      | path                 | content  |
      | rules/file-naming.md | # Naming |
      | rules/001-first.md   | # First  |
    When I run the "naming" validator
    Then the exit code is 0
    And 2 files were inspected

  Scenario Outline: A kebab-case surface refuses a name written any other way
    Given the repository declares the kebab-case naming surface "rules/**/*.md"
    And the repository contains:
      | path   | content |
      | <path> | # Entry |
    When I run the "naming" validator
    Then the only violation starts with "<path>"
    And all violations are "is not named in the declared kebab-case style"

    Examples:
      | path                     |
      | rules/Foo_Bar.md         |
      | rules/Getting Started.md |
      | rules/foo--bar.md        |
      | rules/-leading.md        |

  Scenario: A path-prefixed name encodes the directory it sits in
    Given the repository declares the path-prefixed naming surface "vault/**/*.md" separated by "__"
    And the repository contains:
      | path                                                               | content    |
      | vault/docs/tutorials/do-tu__getting-started.md                     | # Tutorial |
      | vault/people/pe__ada-lovelace.md                                   | # Profile  |
      | vault/jobs/north-branch/events/training/jo-nobr-ev-tr__workshop.md | # Event    |
    When I run the "naming" validator
    Then the exit code is 0
    And 3 files were inspected

  Scenario: A path-prefixed name whose prefix does not encode its directory is refused
    Given the repository declares the path-prefixed naming surface "vault/**/*.md" separated by "__"
    And the repository contains:
      | path                                    | content    |
      | vault/docs/tutorials/getting-started.md | # Tutorial |
    When I run the "naming" validator
    Then the only violation starts with "vault/docs/tutorials/getting-started.md"
    And all violations are "is not named in the declared path-prefixed style"

  Scenario Outline: Each directory segment encodes by the rule its own words follow
    Given the repository declares the path-prefixed naming surface "vault/**/*.md" separated by "__"
    And the repository contains:
      | path   | content |
      | <path> | # Entry |
    When I run the "naming" validator
    Then the exit code is 0
    And 1 files were inspected

    Examples:
      | path                                      |
      | vault/f-sharp/f_sh__records.md            |
      | vault/2025-annual-review/20anre__notes.md |
      | vault/level-1-beginner/le1_be__intro.md   |
      | vault/red-x-of-green/rex_ofgr__agenda.md  |

  Scenario: A file at the surface root has no directory to encode
    Given the repository declares the path-prefixed naming surface "vault/**/*.md" separated by "__"
    And the repository contains:
      | path           | content |
      | vault/index.md | # Index |
    When I run the "naming" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: An exempt file is named by no style at all
    Given the repository declares the kebab-case naming surface "rules/**/*.md"
    And the repository declares the naming exemption "**/README.md"
    And the repository contains:
      | path            | content |
      | rules/README.md | # Rules |
    When I run the "naming" validator
    Then the exit code is 0
    And 0 files were inspected

  Scenario: A later surface overrides an earlier one for the same file
    Given the repository declares the kebab-case naming surface "**/*.md"
    And the repository declares the path-prefixed naming surface "vault/**/*.md" separated by "__"
    And the repository contains:
      | path                             | content   |
      | vault/people/pe__ada-lovelace.md | # Profile |
      | notes/plain-entry.md             | # Notes   |
    When I run the "naming" validator
    Then the exit code is 0
    And 2 files were inspected

  Scenario: Files outside every declared surface carry no style
    Given the repository declares the kebab-case naming surface "rules/**/*.md"
    And the repository contains:
      | path               | content    |
      | guides/Overview.md | # Overview |
    When I run the "naming" validator
    Then the exit code is 0
    And 0 files were inspected

  Scenario: Omitting the section refuses rather than assuming a convention
    Given the repository declares a complete configuration
    When I run the "naming" validator
    Then the exit code is 2
    And stderr contains "md-naming"

  Scenario: An unusable surface glob is a configuration fault
    Given the repository declares the kebab-case naming surface "rules/["
    When I run the "naming" validator
    Then the exit code is 2
    And stderr contains "md-naming.surfaces"

  Scenario: A path-prefixed surface needs the separator that joins its two halves
    Given the configuration sets "md-naming.surfaces" to "[{glob: vault/**/*.md, style: path-prefixed}]"
    And the configuration sets "md-naming.exempt" to "[]"
    When I run the "naming" validator
    Then the exit code is 2
    And stderr contains "separator"

  Scenario: A kebab-case surface has no two halves to separate
    Given the configuration sets "md-naming.surfaces" to "[{glob: rules/**/*.md, style: kebab-case, separator: __}]"
    And the configuration sets "md-naming.exempt" to "[]"
    When I run the "naming" validator
    Then the exit code is 2
    And stderr contains "separator"

  Scenario: An unusable exemption glob is a configuration fault
    Given the repository declares the kebab-case naming surface "rules/**/*.md"
    And the repository declares the naming exemption "rules/["
    When I run the "naming" validator
    Then the exit code is 2
    And stderr contains "md-naming.exempt"
