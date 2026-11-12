// Environment vault transfer: env pull, push, and diff.
//
// Each command runs in two phases. The plan phase reads the manifest, the local
// files, and the vault, and records every file write and provider change the
// run would make; a refusal in this phase changes nothing. The apply phase makes
// the recorded file writes in manifest order, then the recorded provider changes
// in the same order. The vault cannot be changed as one transaction, so an apply
// failure stops the run where it failed and keeps the changes made before it. A
// finding holds back the change it names. A pull writes no file for an entry
// with any finding. A push holds back only the key or file it names, so the
// other keys of an env entry still apply. Every other clean entry still applies.
// Names and counts are reported; no value, digest, length, item ID, or provider
// message is ever printed.
//
// The env form lives in environment_vault_env.go, the file form and its
// generations in environment_vault_file.go, and the env dialect in
// environment_vault_dialect.go.
package application

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// vaultHeader is the first line of every file pull writes.
const vaultHeader = "# Written by rhino env pull from the vault; change the vault, not this file."

// vaultDefaultMode is the mode of a written file whose entry declares none. It
// is a safety invariant of the product, not a per-repository choice: a secret
// file is owner-only unless its entry declares another mode.
const vaultDefaultMode = 0o600

// vaultValueField is the one value field this build reads: the login password.
const vaultValueField = "password"

// The finding classes are public once released, so they are fixed here.
const (
	classItemMissing     = "item-missing"
	classItemAmbiguous   = "item-ambiguous"
	classLocalMissing    = "local-missing"
	classLocalDiffers    = "local-differs"
	classModeDiffers     = "mode-differs"
	classVaultDiffers    = "vault-differs"
	classUndeclaredKey   = "undeclared-key"
	classAttachmentMiss  = "attachment-missing"
	vaultLoginKind       = "login"
	vaultNoteKind        = "note"
	vaultOperationPull   = "pull"
	vaultOperationPush   = "push"
	vaultOperationDiff   = "diff"
	vaultStatusFlagged   = "findings"
	vaultStatusCompleted = "completed"
)

// transferFinding is one class and the entry or key it names.
type transferFinding struct {
	class string
	name  string
}

// entryStatus is how one selected entry ended the plan. Each entry has exactly one.
type entryStatus int

const (
	statusIdentical entryStatus = iota
	statusChanged
	statusFlagged
)

// vaultMutation is one provider change a push applies in the apply phase.
type vaultMutation func(SecretVault) *vaultFailure

// transfer is one run of env pull, push, or diff.
type transfer struct {
	operation string
	format    Format
	manifest  *config.VaultManifest
	tree      Tree
	vault     SecretVault
	files     VaultFileStore
	force     bool
	overwrite bool
	findings  []transferFinding
	tally     map[entryStatus]int
	writes    []VaultFile
	mutations []vaultMutation
}

// localFile is one declared local file as the run reads it: whether it exists,
// its text, and its permission bits.
type localFile struct {
	exists bool
	text   string
	mode   uint32
}

// fileModeTree is the query a tree answers for the permission bits of one
// regular file. A path behind a link refuses, as a read of it does.
type fileModeTree interface {
	FileMode(path string) (uint32, *ReadError)
}

// environmentTransfer runs env pull, push, or diff. It refuses before any write
// when the manifest is unusable, the selection names no vault entry, a selected
// entry reads a field this build does not read, or the vault session is not usable.
func environmentTransfer(document *config.Document, tree Tree, ports VaultPorts, invocation *Invocation) Outcome {
	format := invocation.Format
	operation := strings.TrimPrefix(invocation.Category, "environment-")
	manifest, failure := vaultManifest(document)
	if failure == nil && manifest == nil {
		failure = &vaultFailure{code: domain.ConfigUndeclared,
			message: "environment vault is not declared in repo-config.yml"}
	}
	if failure == nil {
		failure = transferUsable(manifest)
	}
	var entries []config.VaultEntry
	if failure == nil {
		entries, failure = selectVaultEntries(manifest, invocation.Entries)
	}
	if failure == nil && ports.Vault == nil {
		failure = &vaultFailure{code: domain.VaultUnavailable, message: "no vault provider is wired into this run"}
	}
	if failure == nil && operation == vaultOperationPull && ports.Files == nil {
		failure = &vaultFailure{code: domain.FileUnwritable, message: "no local file store is wired into this run"}
	}
	if failure == nil {
		failure = faultFailure(ports.Vault.Status(manifest.Server))
	}
	if failure != nil {
		return RefuseWith(format, failure.code, failure.message)
	}
	run := &transfer{
		operation: operation, format: format, manifest: manifest, tree: tree,
		vault: ports.Vault, files: ports.Files,
		force: invocation.Force, overwrite: invocation.Overwrite,
		tally: map[entryStatus]int{},
	}
	if failure := run.plan(entries); failure != nil {
		return RefuseWith(format, failure.code, failure.message)
	}
	if failure := run.apply(); failure != nil {
		return RefuseWith(format, failure.code, failure.message)
	}
	return run.render()
}

// transferUsable applies the manifest rules that do not read Git. A manifest
// that breaks one refuses the run: a transfer never runs against a manifest
// `env validate` would reject.
func transferUsable(manifest *config.VaultManifest) *vaultFailure {
	rules := &vaultRules{manifest: manifest, found: map[environmentFinding]bool{}, names: map[string]bool{}}
	rules.checkManifest()
	rules.checkItems()
	if len(rules.found) > 0 {
		return &vaultFailure{code: domain.ConfigUnusable,
			message: "environment vault manifest breaks its rules; run `rhino env validate` to see which"}
	}
	return nil
}

// selectVaultEntries returns the vault entries a run covers, in manifest order:
// every one, or only the named ones. A named path that is not a declared vault
// entry with a path refuses the run; a resident entry has no path to name.
func selectVaultEntries(manifest *config.VaultManifest, selected []string) ([]config.VaultEntry, *vaultFailure) {
	var entries []config.VaultEntry
	for _, entry := range manifest.Entries {
		if entry.Class != "vault" {
			continue
		}
		if len(selected) == 0 || contains(selected, entry.Path) {
			entries = append(entries, entry)
		}
	}
	for _, path := range selected {
		if !vaultEntryNamed(manifest, path) {
			return nil, &vaultFailure{code: domain.ConfigUndeclared,
				message: "`--entry " + path + "` names no declared vault entry"}
		}
	}
	return entries, nil
}

// vaultEntryNamed reports whether path names a declared vault entry with a path, exactly.
func vaultEntryNamed(manifest *config.VaultManifest, path string) bool {
	if path == "" {
		return false
	}
	for _, entry := range manifest.Entries {
		if entry.Class == "vault" && entry.Path == path {
			return true
		}
	}
	return false
}

// plan records every change and finding for every selected entry, in order. A
// resident entry has no file, so pull and push skip it; diff checks its item.
func (t *transfer) plan(entries []config.VaultEntry) *vaultFailure {
	for _, entry := range entries {
		var findings []transferFinding
		var changed bool
		var failure *vaultFailure
		switch {
		case entry.Form == "resident" && t.operation != vaultOperationDiff:
			continue
		case entry.Form == "resident":
			findings, changed, failure = t.planResident(entry)
		case t.operation == vaultOperationPull:
			findings, changed, failure = t.planPull(entry)
		case t.operation == vaultOperationPush:
			findings, changed, failure = t.planPush(entry)
		default:
			findings, changed, failure = t.planDiff(entry)
		}
		if failure != nil {
			return failure
		}
		t.findings = append(t.findings, findings...)
		switch {
		case len(findings) > 0:
			t.tally[statusFlagged]++
		case changed:
			t.tally[statusChanged]++
		default:
			t.tally[statusIdentical]++
		}
	}
	return nil
}

// apply makes the planned writes, then the planned provider changes. A failure
// stops the run and names the first change that could not be made.
func (t *transfer) apply() *vaultFailure {
	for _, file := range t.writes {
		if err := t.files.Write(t.tree.Root(), file); err != nil {
			return &vaultFailure{code: domain.FileUnwritable,
				message: "cannot write `" + file.Path + "`: " + err.Error()}
		}
	}
	for _, mutate := range t.mutations {
		if failure := mutate(t.vault); failure != nil {
			return failure
		}
	}
	return nil
}

// planPull plans one entry's file. A file that matches the vault in bytes and
// mode is identical; a file that matches in bytes but not mode is repaired in
// place, without force; a file whose bytes differ is a finding unless force
// replaces it.
func (t *transfer) planPull(entry config.VaultEntry) ([]transferFinding, bool, *vaultFailure) {
	desired, findings, failure := t.desired(entry)
	if failure != nil || len(findings) > 0 {
		return findings, false, failure
	}
	local, failure := t.readLocal(entry.Path)
	if failure != nil {
		return nil, false, failure
	}
	want := entryMode(entry)
	switch {
	case local.exists && local.text == desired && local.mode == want:
		return nil, false, nil
	case local.exists && local.text == desired:
		t.writes = append(t.writes, VaultFile{Path: entry.Path, Contents: []byte(desired), Mode: want, Replace: true})
		return nil, true, nil
	case local.exists && !t.force:
		return []transferFinding{{class: classLocalDiffers, name: entry.Path}}, false, nil
	}
	t.writes = append(t.writes, VaultFile{
		Path: entry.Path, Contents: []byte(desired), Mode: want, Replace: local.exists,
	})
	return nil, true, nil
}

// desired builds the bytes an entry's file must hold, from the vault alone.
// An entry with any finding has no desired bytes, so nothing is written for it.
func (t *transfer) desired(entry config.VaultEntry) (string, []transferFinding, *vaultFailure) {
	if entry.Form == "file" {
		return t.desiredFile(entry)
	}
	return t.desiredEnv(entry)
}

// planPush plans one entry: seed what the vault lacks, and report what differs.
func (t *transfer) planPush(entry config.VaultEntry) ([]transferFinding, bool, *vaultFailure) {
	local, failure := t.readLocal(entry.Path)
	if failure != nil {
		return nil, false, failure
	}
	if !local.exists {
		return []transferFinding{{class: classLocalMissing, name: entry.Path}}, false, nil
	}
	if entry.Form == "file" {
		return t.pushFile(entry, local.text)
	}
	return t.pushEnv(entry, local.text)
}

// planDiff compares one entry with the vault and reports every difference by
// name. A local file whose permission bits differ from the declared or default
// mode is a difference of its own, whether or not its bytes match.
func (t *transfer) planDiff(entry config.VaultEntry) ([]transferFinding, bool, *vaultFailure) {
	local, failure := t.readLocal(entry.Path)
	if failure != nil {
		return nil, false, failure
	}
	if !local.exists {
		return []transferFinding{{class: classLocalMissing, name: entry.Path}}, false, nil
	}
	var findings []transferFinding
	if entry.Form == "file" {
		findings, failure = t.diffFile(entry, local.text)
	} else {
		findings, failure = t.diffEnv(entry, local.text)
	}
	if failure != nil {
		return nil, false, failure
	}
	if local.mode != entryMode(entry) {
		findings = append(findings, transferFinding{class: classModeDiffers, name: entry.Path})
	}
	return findings, false, nil
}

// planResident checks that one resident entry's login item is held exactly once
// under its name. No file is read or written, and no value is compared.
func (t *transfer) planResident(entry config.VaultEntry) ([]transferFinding, bool, *vaultFailure) {
	name := fileItemName(t.manifest, entry)
	_, class, failure := t.lookup(name, vaultLoginKind)
	if failure != nil {
		return nil, false, failure
	}
	if class != "" {
		return []transferFinding{{class: class, name: name}}, false, nil
	}
	return nil, false, nil
}

// lookup returns the one item of kind held under name exactly. Any other answer
// is a class: item-missing when no item holds the name, and item-ambiguous when
// more than one does, or when the one that does is of another kind. The
// provider's search matches substrings, so the exact filter happens here too.
func (t *transfer) lookup(name, kind string) (VaultItem, string, *vaultFailure) {
	found, fault := t.vault.Find(name)
	if fault != nil {
		return VaultItem{}, "", faultFailure(fault)
	}
	var exact []VaultItem
	for _, item := range found {
		if item.Name == name {
			exact = append(exact, item)
		}
	}
	switch {
	case len(exact) == 0:
		return VaultItem{}, classItemMissing, nil
	case len(exact) > 1 || exact[0].Kind != kind:
		return VaultItem{}, classItemAmbiguous, nil
	}
	return exact[0], "", nil
}

// readablePassword refuses a login whose password the vault holds as null. No
// value can be read from it, and an empty value would write a false KEY= line.
func readablePassword(item VaultItem) *vaultFailure {
	if item.NoPassword {
		return &vaultFailure{code: domain.VaultRefused,
			message: "the vault holds a login item with no password under a declared name; no value is read from it"}
	}
	return nil
}

// readLocal reads one declared local file with its permission bits. A missing
// file is not an error; a link on the path, a file that holds no text, or a
// tree that cannot report a mode refuses the run.
func (t *transfer) readLocal(path string) (localFile, *vaultFailure) {
	text, readErr := t.tree.Read(path)
	if readErr != nil {
		switch readErr.Kind {
		case "missing":
			return localFile{}, nil
		case "escapes":
			return localFile{}, &vaultFailure{code: domain.PathEscapesRoot,
				message: "`" + path + "` crosses a symbolic link"}
		case "not-text":
			return localFile{}, &vaultFailure{code: domain.FileUnreadable,
				message: "`" + path + "` holds no text"}
		default:
			return localFile{}, &vaultFailure{code: domain.FileUnreadable,
				message: "`" + path + "` cannot be read: " + readErr.Reason}
		}
	}
	modes, ok := t.tree.(fileModeTree)
	if !ok {
		return localFile{}, &vaultFailure{code: domain.FileUnreadable,
			message: "this tree cannot report the mode of `" + path + "`"}
	}
	mode, modeErr := modes.FileMode(path)
	if modeErr != nil {
		return localFile{}, &vaultFailure{code: domain.FileUnreadable,
			message: "`" + path + "` cannot be read: " + modeErr.Reason}
	}
	return localFile{exists: true, text: text, mode: mode}, nil
}

// keyItemName is the item an env key reads: its own app and secret, or the entry's app.
func (t *transfer) keyItemName(entry config.VaultEntry, key config.VaultKey) string {
	app := key.App
	if app == "" {
		app = entry.App
	}
	segment := key.Secret
	if segment == "" {
		segment = deriveSecretSegment(key.Key)
	}
	return itemName(t.manifest.Prefix, app, entry.Env, segment)
}

// fileItemName is the item a file entry, or a resident entry, names.
func fileItemName(manifest *config.VaultManifest, entry config.VaultEntry) string {
	return itemName(manifest.Prefix, entry.App, entry.Env, entry.Secret)
}

// entryMode is the mode a written file takes: the entry's, or the default.
func entryMode(entry config.VaultEntry) uint32 {
	if entry.Mode == "" {
		return vaultDefaultMode
	}
	mode, _ := strconv.ParseUint(entry.Mode, 8, 32)
	return uint32(mode)
}

// faultFailure converts a provider fault into a refusal; nil stays nil.
func faultFailure(fault *VaultFault) *vaultFailure {
	if fault == nil {
		return nil
	}
	return &vaultFailure{code: fault.Code, message: fault.Reason}
}

// render maps the run to its outcome: 0 clean, 1 findings. The text form
// prints one summary line and the findings on standard error; the JSON form
// carries the findings in its body.
func (t *transfer) render() Outcome {
	findings := append([]transferFinding(nil), t.findings...)
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].class != findings[j].class {
			return findings[i].class < findings[j].class
		}
		return findings[i].name < findings[j].name
	})
	code := 0
	if len(findings) > 0 {
		code = 1
	}
	if t.format == JSON {
		rendered := make([]string, 0, len(findings))
		for _, finding := range findings {
			rendered = append(rendered, fmt.Sprintf("{\"class\":%s,\"name\":%s}",
				domain.Quote(finding.class), domain.Quote(finding.name)))
		}
		status := vaultStatusCompleted
		if code == 1 {
			status = vaultStatusFlagged
		}
		return Outcome{ExitCode: code, Stdout: fmt.Sprintf(
			"{\"command\":%s,\"count\":%d,\"findings\":[%s],\"schemaVersion\":1,\"status\":%s}\n",
			domain.Quote("environment-"+t.operation), len(findings), strings.Join(rendered, ","),
			domain.Quote(status))}
	}
	var stderr strings.Builder
	for _, finding := range findings {
		fmt.Fprintf(&stderr, "[env-%s] %s %s\n", t.operation, finding.class, finding.name)
	}
	stdout := t.summary()
	for _, finding := range findings {
		if finding.class == classItemMissing && t.operation == vaultOperationDiff {
			stdout += "[env-diff] an item is missing: sync the vault in your own session if you expect it, then diff again\n"
			break
		}
	}
	return Outcome{ExitCode: code, Stdout: stdout, Stderr: stderr.String()}
}

// summary is the one line text mode prints first. A flagged count appears only when non-zero.
func (t *transfer) summary() string {
	total := t.tally[statusChanged] + t.tally[statusIdentical] + t.tally[statusFlagged]
	var line string
	switch t.operation {
	case vaultOperationPull:
		line = fmt.Sprintf("[env-pull] %d entries: %d written, %d identical", total, t.tally[statusChanged], t.tally[statusIdentical])
	case vaultOperationPush:
		line = fmt.Sprintf("[env-push] %d entries: %d pushed, %d identical", total, t.tally[statusChanged], t.tally[statusIdentical])
	default:
		line = fmt.Sprintf("[env-diff] %d entries: %d identical", total, t.tally[statusIdentical])
	}
	if flagged := t.tally[statusFlagged]; flagged > 0 {
		line += fmt.Sprintf(", %d flagged", flagged)
	}
	return line + "\n"
}
