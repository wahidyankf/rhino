# How to move declared secrets through the vault

Use this guide when a repository declares the secrets its files hold under `environment.vault`, and you want to restore
those files from the vault, seed the vault from them, or compare the two. The commands are `env pull`, `env push`, and
`env diff`. Each one is explicit: no validator starts them, and none runs unless you invoke it.

## Before you start

- Declare the entries in `repo-config.yml`. The keys are listed in the
  [vault manifest reference](../reference/v0-4-configuration.md#environment-vault-manifest). Run `rhino env validate`
  first; it checks the manifest without touching the vault.
- Install the Bitwarden CLI, `bw`, and put it on your `PATH`.
- Unlock the vault in your own shell and export the session key it prints. RHINO never unlocks a vault, never logs in,
  and never runs `bw sync`. If an item you expect is missing, sync in your own session, then run the command again.

```text
export BW_SESSION="<the key your own bw unlock printed>"
```

RHINO refuses before it writes anything when the key is absent, when the vault is locked, or when the vault serves a
server other than the one the manifest declares. Each of those refusals exits `2` with the `rhino.vault.unavailable`
code under `--output json`. When `bw` is not on the path, the command exits `127`; when `bw` is on the path but cannot
be executed, it exits `126`. Both use the same `rhino.vault.unavailable` code. Both are checked before the first write,
so neither writes anything when `bw` fails that check. A `bw` that disappears mid-run can exit `127` or `126` after
earlier writes; a vault run is not transactional.

## Compare first

`env diff` reads the vault and the local files and writes nothing. It reports every difference by name:

```text
rhino env diff
```

Exit `0` means every selected entry is identical. Exit `1` lists each finding, such as `vault-differs ALPHA_KEY` or
`item-missing BETA_KEY`, one per line on standard error. No value appears in any output.

`env diff` also reports `mode-differs` when a local file's permission bits are not the mode its entry declares, or
`0600` when it declares none. It names the path only and repairs nothing. A resident entry holds no file, so `diff`
checks only that its login item is held exactly once under its name.

In text mode, when `diff` reports `item-missing`, it also prints
`[env-diff] an item is missing: sync the vault in your own session if you expect it, then diff again` on standard
output. JSON mode does not print that line. RHINO never runs `bw sync`, so sync in your own session first.

Narrow the run to one declared entry with `--entry <path>`, which may repeat. A path that is not a declared vault entry
refuses the run with exit `2` before the vault is read:

```text
rhino env diff --entry config/app.env
```

## Restore the local files

`env pull` writes each declared file from the vault, atomically, with mode `0600` unless its entry declares another
mode. Parent directories are created `0700`.

```text
rhino env pull
```

- A file that does not exist is written.
- A file that already matches the vault in bytes and mode is left alone. A file that matches in bytes but has another
  mode is repaired to its declared mode in place, without `--force`.
- A file that differs from the vault is reported as `local-differs` and is not written. Replace it deliberately with
  `rhino env pull --force`.
- A declared file whose vault items are missing or ambiguous is reported, and nothing is written for that file. Other
  clean entries are still written.
- A symbolic link on the path, or a local file that holds no UTF-8 text, refuses the run before any write. `--force`
  does not replace a file that holds no UTF-8 text; move or remove that file by hand first.
- A resident entry holds no file, so `pull` skips it.

## Seed the vault from the local files

`env push` creates the vault items a declaration needs and reports what differs. Nothing that already matches changes.

```text
rhino env push
```

- A missing login item is created with the local value.
- A missing secure note is created, and its first file generation is uploaded and read back before the run ends.
- A differing value is reported as `vault-differs` and is not changed. Replace it deliberately with
  `rhino env push --overwrite`.
- With `--overwrite`, a login keeps its previous password in the item's history. A file upload is read back and compared
  with the bytes it meant to store. Once the new generation verifies, every older generation is removed, so the newest
  previous generation stays. An upload that does not verify is removed, and every older generation stays.
- A login item whose password the vault holds as null, and a file entry whose newest generation cannot be read, each
  refuse the run with exit `2` and write nothing. RHINO never uses an older generation in place of an unreadable newest
  one.
- A key whose `field` in the manifest is anything other than `password` is reported by `rhino env validate` as
  `vault-manifest-invalid`. Each vault command refuses that manifest with exit `2` before it reads the vault.
- A local env line that cannot be read stops the run with exit `2`, and the message gives the line number only.
- A key in the local file that the manifest does not declare is reported as `undeclared-key`.

## Read the exit codes

| Exit | Meaning for these commands                                                                                 |
| ---- | ---------------------------------------------------------------------------------------------------------- |
| `0`  | Every selected entry is clean, identical, or written.                                                      |
| `1`  | At least one finding. Each names a class and an entry or key. Nothing is written for a flagged pull entry. |
| `2`  | The session, server, manifest, a local file, or a write was unusable. Read the message.                    |

A vault run is not transactional. A session check refuses before any write. A later apply step can fail after earlier
writes have landed, and those writes stay in place. Running the same command again converges on the declared state,
because an entry that already matches is left alone.

Under `--output json`, each run reports one object with `command`, `count`, `findings` (each with `class` and `name`),
and `status`. The full list of finding classes is in the [findings reference](../reference/findings.md#vault-transfer).
