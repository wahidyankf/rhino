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

RHINO refuses before it writes anything when the key is absent, when `bw` is not on the path, when the vault is locked,
or when the vault serves a server other than the one the manifest declares. Each refusal exits `2` with the
`rhino.vault.unavailable` code under `--output json`.

## Compare first

`env diff` reads the vault and the local files and writes nothing. It reports every difference by name:

```text
rhino env diff
```

Exit `0` means every selected entry is identical. Exit `1` lists each finding, such as `vault-differs ALPHA_KEY` or
`item-missing BETA_KEY`, one per line on standard error. No value appears in any output.

Narrow the run to one declared entry with `--entry <path>`, which may repeat:

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
- A file that already matches the vault is left alone.
- A file that differs from the vault is reported as `local-differs` and is not written. Replace it deliberately with
  `rhino env pull --force`.
- A declared file whose vault items are missing or ambiguous is reported, and nothing is written for that file. Other
  clean entries are still written.
- A symbolic link on the path, or a local file that holds no text, refuses the run before any write.

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
  with the bytes it meant to store. Only a verified generation retires the one before it. An upload that does not verify
  is removed, and the previous generation stays.
- A local env line that cannot be read stops the run with exit `2`, and the message gives the line number only.
- A key in the local file that the manifest does not declare is reported as `undeclared-key`.

## Read the exit codes

| Exit | Meaning for these commands                                                                                 |
| ---- | ---------------------------------------------------------------------------------------------------------- |
| `0`  | Every selected entry is clean, identical, or written.                                                      |
| `1`  | At least one finding. Each names a class and an entry or key. Nothing is written for a flagged pull entry. |
| `2`  | The session, server, manifest, a local file, or a write was unusable. Read the message.                    |

Under `--output json`, each run reports one object with `command`, `count`, `findings` (each with `class` and `name`),
and `status`. The full list of finding classes is in the [findings reference](../reference/findings.md#vault-transfer).
