# Markdown Links

Every repository-authored Markdown file must keep its internal links valid. Resolve a relative link from the file that contains it; its target must remain inside the repository and exist as a file or directory.

Fragments and query strings do not change the target-file check. External and protocol links are outside this local validation — a link to a released document on another host is checked by reading it, not by the gate.

Update every affected link in the same change as a move, rename, or deletion. A link that pointed somewhere real before the change is the change's responsibility.

Sources excluded from validation are declared in [`repo-config.yml`](../../repo-config.yml) under `md-internal-link`. The list is empty by decision: this repository keeps no archived tree whose links are allowed to rot.

## Enforcement

```sh
./hippo run --class ephemeral --disk-path . -- cargo xtask self-validate
```
