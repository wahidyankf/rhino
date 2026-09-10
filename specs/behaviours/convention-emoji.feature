Feature: Emoji-convention validation

  A repository declares which files may not carry an emoji code point. Only the
  prohibition is checked, never the permission: whether an emoji belongs in a
  particular sentence is a judgement about meaning that no validator can make,
  and a rule that guessed would be worse than no rule.

  Scenario: An emoji in a prohibited file is a finding at the line it sits on
    Given the repository declares the emoji-prohibited surface "**/*.json"
    And the repository contains:
      | path          | content                              |
      | settings.json | {\n  "note": "ship it 🚀"\n}         |
    When I run the "emoji" validator
    Then the only violation starts with "settings.json:2: holds an emoji code point"

  Scenario: The same code point in a file matching no prohibited glob is not a finding
    Given the repository declares the emoji-prohibited surface "**/*.json"
    And the repository contains:
      | path     | content     |
      | notes.md | Ship it 🚀  |
    When I run the "emoji" validator
    Then the exit code is 0
    And 0 files were inspected

  Scenario: Ordinary punctuation is not an emoji
    Given the repository declares the emoji-prohibited surface "**/*.json"
    And the repository contains:
      | path          | content                                                    |
      | settings.json | {"note": "an em dash — an arrow → a copyright © a plus ±"} |
    When I run the "emoji" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: Each line carrying an emoji is reported once
    Given the repository declares the emoji-prohibited surface "**/*.json"
    And the repository contains:
      | path          | content                                        |
      | settings.json | {\n  "a": "🚀 🚀",\n  "b": "✅",\n  "c": "x"\n} |
    When I run the "emoji" validator
    Then there are 2 violations
    And all violations are "holds an emoji code point"

  Scenario: A prohibited file that is clean passes and is still counted
    Given the repository declares the emoji-prohibited surface "**/*.json"
    And the repository contains:
      | path          | content            |
      | settings.json | {"note": "ship it"} |
    When I run the "emoji" validator
    Then the exit code is 0
    And 1 files were inspected

  Scenario: Every declared prohibited surface is walked
    Given the repository declares the emoji-prohibited surface "**/*.json"
    And the repository declares the emoji-prohibited surface "scripts/**/*.sh"
    And the repository contains:
      | path              | content                |
      | settings.json     | {"note": "ship it"}    |
      | scripts/deploy.sh | echo "shipping 🚀"     |
    When I run the "emoji" validator
    Then the only violation starts with "scripts/deploy.sh:1: holds an emoji code point"
    And 2 files were inspected

  Scenario: An excluded directory holds nothing to inspect
    Given the repository declares the emoji-prohibited surface "**/*.json"
    And the repository declares excluded scan directories:
      * build-output
    And the repository contains:
      | path                      | content              |
      | build-output/settings.json | {"note": "ship it 🚀"} |
    When I run the "emoji" validator
    Then the exit code is 0
    And 0 files were inspected

  Scenario: A prohibited file that cannot be read is refused
    Given the repository declares the emoji-prohibited surface "**/*.json"
    And the repository contains:
      | path          | content             |
      | settings.json | {"note": "ship it"} |
    And the governed files are exclusively locked
    When I run the "emoji" validator
    Then the exit code is 2
    And stderr names a file it could not read

  Scenario: Omitting the section refuses rather than assuming a prohibition
    Given the repository declares a complete configuration
    When I run the "emoji" validator
    Then the exit code is 2
    And stderr contains "convention-emoji"

  Scenario: An unusable prohibited glob is a configuration fault
    Given the repository declares the emoji-prohibited surface "scripts/["
    When I run the "emoji" validator
    Then the exit code is 2
    And stderr contains "convention-emoji.prohibited"

  Scenario: A file that vanishes between the walk and the read is not a finding
    Given the repository declares the emoji-prohibited surface "**/*.json"
    And the repository contains:
      | path          | content             |
      | settings.json | {"note": "ship it"} |
    And file "settings.json" vanishes between the walk and the read
    When I run the "emoji" validator
    Then the exit code is 0
    And 0 files were inspected

  Scenario: A prohibited file that holds no text is refused
    Given the repository declares the emoji-prohibited surface "**/*.json"
    And file "settings.json" holds bytes that are not text
    When I run the "emoji" validator
    Then the exit code is 2
