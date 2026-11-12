Feature: Move declared secrets between a vault and the repository

  A repository declares the secrets each of its files holds under environment.vault. env pull writes the declared files
  from an unlocked vault session, env push seeds or overwrites the vault from those files, and env diff compares them
  without writing. Every command reads the vault through one provider boundary, refuses an unusable session before it
  writes anything, and reports names only, never a value.

  Scenario: Restore every declared secret file on a fresh checkout
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: example/fixture
          environments: [local, stag, prod]
          entries:
            - path: config/app.env
              class: vault
              form: env
              app: web
              env: local
              mode: "0600"
              keys: [ALPHA_KEY, BETA_KEY]
            - path: config/service.json
              class: vault
              form: file
              app: api
              env: local
              secret: service-config
              mode: "0600"
      """
    And the vault session is unlocked for the declared server
    And the vault holds a login item "example/fixture/web/local/alpha-key" with the password "marker-alpha-one"
    And the vault holds a login item "example/fixture/web/local/beta-key" with the password "marker-beta-two"
    And the vault holds the secure note "example/fixture/api/local/service-config" with the attachment "service.json.g0001" holding the content "marker-service-one"
    When I run environment pull
    Then the command exits 0
    And the local file "config/app.env" holds this content:
      """
      # Written by rhino env pull from the vault; change the vault, not this file.
      ALPHA_KEY=marker-alpha-one
      BETA_KEY=marker-beta-two
      """
    And the local file "config/app.env" has the mode "0600"
    And the local file "config/service.json" holds the content "marker-service-one"
    And the local file "config/service.json" has the mode "0600"
    And no run output contains "marker-alpha-one"
    And no run output contains "marker-beta-two"
    And no run output contains "marker-service-one"

  Scenario: Refuse to replace a different local file without force
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: example/fixture
          environments: [local, stag, prod]
          entries:
            - path: config/app.env
              class: vault
              form: env
              app: web
              env: local
              mode: "0600"
              keys: [ALPHA_KEY, BETA_KEY]
      """
    And the vault session is unlocked for the declared server
    And the vault holds a login item "example/fixture/web/local/alpha-key" with the password "marker-alpha-one"
    And the vault holds a login item "example/fixture/web/local/beta-key" with the password "marker-beta-two"
    And the repository holds the local file "config/app.env" with this content:
      """
      ALPHA_KEY=marker-local-other
      BETA_KEY=marker-beta-two
      """
    When I run environment pull
    Then the command exits 1
    And the output names "local-differs config/app.env"
    And the local file "config/app.env" holds this content:
      """
      ALPHA_KEY=marker-local-other
      BETA_KEY=marker-beta-two
      """
    And no run output contains "marker-alpha-one"
    And no run output contains "marker-local-other"

  Scenario: Replace a different local file with force
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: example/fixture
          environments: [local, stag, prod]
          entries:
            - path: config/app.env
              class: vault
              form: env
              app: web
              env: local
              mode: "0600"
              keys: [ALPHA_KEY, BETA_KEY]
      """
    And the vault session is unlocked for the declared server
    And the vault holds a login item "example/fixture/web/local/alpha-key" with the password "marker-alpha-one"
    And the vault holds a login item "example/fixture/web/local/beta-key" with the password "marker-beta-two"
    And the repository holds the local file "config/app.env" with the content "ALPHA_KEY=marker-local-other"
    When I run environment pull with force
    Then the command exits 0
    And the local file "config/app.env" holds this content:
      """
      # Written by rhino env pull from the vault; change the vault, not this file.
      ALPHA_KEY=marker-alpha-one
      BETA_KEY=marker-beta-two
      """
    And the local file "config/app.env" has the mode "0600"
    And no run output contains "marker-alpha-one"
    And no run output contains "marker-local-other"

  Scenario: Seed missing items and refuse differences
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: example/fixture
          environments: [local, stag, prod]
          entries:
            - path: config/app.env
              class: vault
              form: env
              app: web
              env: local
              mode: "0600"
              keys: [ALPHA_KEY, BETA_KEY]
            - path: config/service.json
              class: vault
              form: file
              app: api
              env: local
              secret: service-config
              mode: "0600"
      """
    And the vault session is unlocked for the declared server
    And the vault holds a login item "example/fixture/web/local/beta-key" with the password "marker-beta-vault"
    And the repository holds the local file "config/app.env" with this content:
      """
      ALPHA_KEY=marker-alpha-local
      BETA_KEY=marker-beta-local
      """
    And the repository holds the local file "config/service.json" with the content "marker-service-local"
    When I run environment push
    Then the command exits 1
    And the output names "vault-differs BETA_KEY"
    And the vault holds the login item "example/fixture/web/local/alpha-key" with the password "marker-alpha-local"
    And the vault holds the login item "example/fixture/web/local/beta-key" with the password "marker-beta-vault"
    And the secure note "example/fixture/api/local/service-config" holds the attachment "service.json.g0001" with the content "marker-service-local"
    And no run output contains "marker-beta-local"
    And no run output contains "marker-beta-vault"
    And no run output contains "marker-alpha-local"

  Scenario: Overwrite keeps recovery points
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: example/fixture
          environments: [local, stag, prod]
          entries:
            - path: config/app.env
              class: vault
              form: env
              app: web
              env: local
              mode: "0600"
              keys: [ALPHA_KEY, BETA_KEY]
            - path: config/service.json
              class: vault
              form: file
              app: api
              env: local
              secret: service-config
              mode: "0600"
      """
    And the vault session is unlocked for the declared server
    And the vault holds a login item "example/fixture/web/local/alpha-key" with the password "marker-alpha-old"
    And the vault holds a login item "example/fixture/web/local/beta-key" with the password "marker-beta-old"
    And the vault holds the secure note "example/fixture/api/local/service-config" with the attachment "service.json.g0001" holding the content "marker-service-first"
    And the vault holds the secure note "example/fixture/api/local/service-config" with the attachment "service.json.g0002" holding the content "marker-service-second"
    And the repository holds the local file "config/app.env" with this content:
      """
      ALPHA_KEY=marker-alpha-old
      BETA_KEY=marker-beta-new
      """
    And the repository holds the local file "config/service.json" with the content "marker-service-new"
    When I run environment push with overwrite
    Then the command exits 0
    And the vault holds the login item "example/fixture/web/local/beta-key" with the password "marker-beta-new"
    And the login item "example/fixture/web/local/beta-key" keeps the password "marker-beta-old" in its history
    And the secure note "example/fixture/api/local/service-config" holds the attachment "service.json.g0002"
    And the secure note "example/fixture/api/local/service-config" holds the attachment "service.json.g0003"
    And the secure note "example/fixture/api/local/service-config" does not hold the attachment "service.json.g0001"
    And no run output contains "marker-beta-new"
    And no run output contains "marker-beta-old"
    And no run output contains "marker-service-new"

  Scenario: Report differences by name only
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: example/fixture
          environments: [local, stag, prod]
          entries:
            - path: config/app.env
              class: vault
              form: env
              app: web
              env: local
              mode: "0600"
              keys: [ALPHA_KEY, BETA_KEY]
            - path: config/service.json
              class: vault
              form: file
              app: api
              env: local
              secret: service-config
              mode: "0600"
      """
    And the vault session is unlocked for the declared server
    And the vault holds a login item "example/fixture/web/local/alpha-key" with the password "marker-alpha-vault"
    And the vault holds the secure note "example/fixture/api/local/service-config" with the attachment "service.json.g0001" holding the content "marker-service-vault"
    And the repository holds the local file "config/app.env" with this content:
      """
      ALPHA_KEY=marker-alpha-local
      BETA_KEY=marker-beta-local
      """
    And the repository holds the local file "config/service.json" with the content "marker-service-local"
    When I run environment diff
    Then the command exits 1
    And the output names "vault-differs ALPHA_KEY"
    And the output names "item-missing BETA_KEY"
    And the output names "vault-differs config/service.json"
    And the local files are unchanged by the last run
    And the vault is unchanged by the last run
    And no run output contains "marker-alpha-vault"
    And no run output contains "marker-alpha-local"
    And no run output contains "marker-beta-local"
    And no run output contains "marker-service-vault"

  Scenario Outline: Refuse an unusable vault before any write
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: example/fixture
          environments: [local, stag, prod]
          entries:
            - path: config/app.env
              class: vault
              form: env
              app: web
              env: local
              mode: "0600"
              keys: [ALPHA_KEY]
      """
    And the vault session <session>
    And the vault holds a login item "example/fixture/web/local/alpha-key" with the password "marker-alpha-vault"
    And the repository holds the local file "config/app.env" with the content "ALPHA_KEY=marker-alpha-local"
    When I run environment <command> in json format
    Then the command exits 2
    And the output names "rhino.vault.unavailable"
    And the local files are unchanged by the last run
    And the vault is unchanged by the last run
    And no run output contains "marker-alpha-vault"
    And no run output contains "marker-alpha-local"

    Examples:
      | session                  | command |
      | has no key               | pull    |
      | is locked                | pull    |
      | points at another server | pull    |
      | has no key               | push    |
      | is locked                | push    |
      | points at another server | push    |
      | has no key               | diff    |
      | is locked                | diff    |
      | points at another server | diff    |

  Scenario: Refuse an ambiguous exact name
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: example/fixture
          environments: [local, stag, prod]
          entries:
            - path: config/app.env
              class: vault
              form: env
              app: web
              env: local
              mode: "0600"
              keys: [ALPHA_KEY, BETA_KEY]
      """
    And the vault session is unlocked for the declared server
    And the vault holds a login item "example/fixture/web/local/alpha-key" with the password "marker-alpha-first"
    And the vault holds a login item "example/fixture/web/local/alpha-key" with the password "marker-alpha-second"
    And the vault holds a login item "example/fixture/web/local/beta-key" with the password "marker-beta-vault"
    When I run environment pull
    Then the command exits 1
    And the output names "item-ambiguous ALPHA_KEY"
    And the local file "config/app.env" does not exist
    And no run output contains "marker-alpha-first"
    And no run output contains "marker-alpha-second"
    And no run output contains "marker-beta-vault"

  Scenario: Refuse a missing exact name
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: example/fixture
          environments: [local, stag, prod]
          entries:
            - path: config/app.env
              class: vault
              form: env
              app: web
              env: local
              mode: "0600"
              keys: [ALPHA_KEY, BETA_KEY]
      """
    And the vault session is unlocked for the declared server
    And the vault holds a login item "example/fixture/web/local/alpha-key" with the password "marker-alpha-vault"
    When I run environment pull
    Then the command exits 1
    And the output names "item-missing BETA_KEY"
    And the local file "config/app.env" does not exist
    And no run output contains "marker-alpha-vault"

  Scenario Outline: Print no value in any output
    Given the repository declares this environment configuration:
      """
      schema: rhino/repo-config/v2
      environment:
        vault:
          provider: bitwarden
          server: https://vault.example.com
          prefix: example/fixture
          environments: [local, stag, prod]
          entries:
            - path: config/app.env
              class: vault
              form: env
              app: web
              env: local
              mode: "0600"
              keys: [ALPHA_KEY, BETA_KEY]
            - path: config/service.json
              class: vault
              form: file
              app: api
              env: local
              secret: service-config
              mode: "0600"
      """
    And the vault session is unlocked for the declared server
    And the vault holds a login item "example/fixture/web/local/alpha-key" with the password "marker-alpha-vault"
    And the vault holds a login item "example/fixture/web/local/beta-key" with the password "marker-beta-vault"
    And the vault holds the secure note "example/fixture/api/local/service-config" with the attachment "service.json.g0001" holding the content "marker-service-vault"
    And the repository holds the local file "config/app.env" with this content:
      """
      ALPHA_KEY=marker-alpha-local
      BETA_KEY=marker-beta-local
      """
    And the repository holds the local file "config/service.json" with the content "marker-service-local"
    When I run environment <command> in <format> format
    Then the command exits 1
    And no run output contains "marker-alpha-vault"
    And no run output contains "marker-beta-vault"
    And no run output contains "marker-service-vault"
    And no run output contains "marker-alpha-local"
    And no run output contains "marker-beta-local"
    And no run output contains "marker-service-local"

    Examples:
      | command | format |
      | pull    | text   |
      | pull    | json   |
      | push    | text   |
      | push    | json   |
      | diff    | text   |
      | diff    | json   |
