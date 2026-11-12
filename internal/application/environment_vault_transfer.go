// Environment vault transfer: env pull, push, and diff.
//
// Each command runs in two phases. The plan phase reads the manifest, the
// local files, and the vault, and records every write and every provider
// change it would make. The apply phase makes them, in manifest order. A
// refusal in the plan phase therefore changes nothing; a finding only keeps
// the entry it names out of the change set. Names and counts are reported;
// no value, digest, length, item ID, or provider message is ever printed.
package application

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// vaultHeader is the first line of every file pull writes.
const vaultHeader = "# Written by rhino env pull from the vault; change the vault, not this file."

// vaultDefaultMode is the mode of a written file whose entry declares none.
const vaultDefaultMode = 0o600

// vaultValueField is the one value field this build reads: the login password.
const vaultValueField = "password"

// The finding classes are public once released, so they are fixed here.
const (
	classItemMissing     = "item-missing"
	classItemAmbiguous   = "item-ambiguous"
	classLocalMissing    = "local-missing"
	classLocalDiffers    = "local-differs"
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

var (
	// envKeyPattern is a key the env dialect reads: a shell name.
	envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	// envBarePattern is a value the env dialect writes without quotes.
	envBarePattern = regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,-]*$`)
	// envGeneration is the tail of a generation attachment name: g and four digits.
	envGeneration = regexp.MustCompile(`^g([0-9]{4})$`)
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

// localFile is one declared local file as the run reads it.
type localFile struct {
	exists bool
	text   string
}

// environmentTransfer runs env pull, push, or diff. It refuses before any
// write when the manifest is unusable, the selection names no vault entry, or
// the vault session is not usable.
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
// `env validate` would reject. A field override this build does not read also refuses.
func transferUsable(manifest *config.VaultManifest) *vaultFailure {
	rules := &vaultRules{manifest: manifest, found: map[environmentFinding]bool{}, names: map[string]bool{}}
	rules.checkManifest()
	rules.checkItems()
	if len(rules.found) > 0 {
		return &vaultFailure{code: domain.ConfigUnusable,
			message: "environment vault manifest breaks its rules; run `rhino env validate` to see which"}
	}
	for _, entry := range manifest.Entries {
		for _, key := range entry.Keys {
			if key.Field != "" && key.Field != vaultValueField {
				return &vaultFailure{code: domain.ConfigUnusable,
					message: "environment vault field `" + key.Field + "` is not read by this build; only `password` is"}
			}
		}
	}
	return nil
}

// selectVaultEntries returns the vault entries a run covers, in manifest
// order: every one, or only the named ones. A named path that is not a
// declared vault entry refuses the run.
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

// vaultEntryNamed reports whether path names a declared vault entry exactly.
func vaultEntryNamed(manifest *config.VaultManifest, path string) bool {
	for _, entry := range manifest.Entries {
		if entry.Class == "vault" && entry.Path == path {
			return true
		}
	}
	return false
}

// plan records every change and finding for every selected entry, in order.
func (t *transfer) plan(entries []config.VaultEntry) *vaultFailure {
	for _, entry := range entries {
		var findings []transferFinding
		var changed bool
		var failure *vaultFailure
		switch t.operation {
		case vaultOperationPull:
			findings, changed, failure = t.planPull(entry)
		case vaultOperationPush:
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

// planPull plans one entry's file. A file that already matches is identical;
// a file that differs is a finding unless force replaces it.
func (t *transfer) planPull(entry config.VaultEntry) ([]transferFinding, bool, *vaultFailure) {
	desired, findings, failure := t.desired(entry)
	if failure != nil || len(findings) > 0 {
		return findings, false, failure
	}
	local, failure := t.readLocal(entry.Path)
	if failure != nil {
		return nil, false, failure
	}
	switch {
	case local.exists && local.text == desired:
		return nil, false, nil
	case local.exists && !t.force:
		return []transferFinding{{class: classLocalDiffers, name: entry.Path}}, false, nil
	}
	t.writes = append(t.writes, VaultFile{
		Path: entry.Path, Contents: []byte(desired), Mode: entryMode(entry), Replace: local.exists,
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

// desiredEnv renders KEY=value lines in manifest order under the header.
func (t *transfer) desiredEnv(entry config.VaultEntry) (string, []transferFinding, *vaultFailure) {
	var out strings.Builder
	out.WriteString(vaultHeader + "\n")
	var findings []transferFinding
	for _, key := range entry.Keys {
		items, failure := t.matching(t.keyItemName(entry, key), vaultLoginKind)
		if failure != nil {
			return "", nil, failure
		}
		class := itemClass(items)
		if class != "" {
			findings = append(findings, transferFinding{class: class, name: key.Key})
			continue
		}
		value := items[0].Password
		if strings.ContainsAny(value, "\r\n") {
			return "", nil, &vaultFailure{code: domain.VaultRefused,
				message: "the vault holds a value of `" + key.Key + "` that an env file cannot hold on one line"}
		}
		fmt.Fprintf(&out, "%s=%s\n", key.Key, encodeEnvValue(value))
	}
	if len(findings) > 0 {
		return "", findings, nil
	}
	return out.String(), nil, nil
}

// desiredFile returns the newest generation of a file entry's secret.
func (t *transfer) desiredFile(entry config.VaultEntry) (string, []transferFinding, *vaultFailure) {
	items, failure := t.matching(fileItemName(t.manifest, entry), vaultNoteKind)
	if failure != nil {
		return "", nil, failure
	}
	if class := itemClass(items); class != "" {
		return "", []transferFinding{{class: class, name: entry.Path}}, nil
	}
	generations := generationsOf(path.Base(entry.Path), items[0].Attachments)
	newest := highestGeneration(generations)
	if newest == 0 {
		return "", []transferFinding{{class: classAttachmentMiss, name: entry.Path}}, nil
	}
	data, fault := t.vault.ReadAttachment(items[0].ID, generations[newest].ID)
	if fault != nil {
		return "", nil, faultFailure(fault)
	}
	return string(data), nil, nil
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

// pushEnv plans the login items of an env entry, one per declared key.
func (t *transfer) pushEnv(entry config.VaultEntry, text string) ([]transferFinding, bool, *vaultFailure) {
	values, bad := parseEnvDialect(text)
	if bad > 0 {
		return nil, false, &vaultFailure{code: domain.FileUnreadable,
			message: "`" + entry.Path + "` line " + strconv.Itoa(bad) + " is not an env line this build reads"}
	}
	declared := map[string]bool{}
	for _, key := range entry.Keys {
		declared[key.Key] = true
	}
	var findings []transferFinding
	for _, name := range sortedValueNames(values) {
		if !declared[name] {
			findings = append(findings, transferFinding{class: classUndeclaredKey, name: name})
		}
	}
	changed := false
	for _, key := range entry.Keys {
		value, present := values[key.Key]
		if !present {
			findings = append(findings, transferFinding{class: classLocalMissing, name: key.Key})
			continue
		}
		name := t.keyItemName(entry, key)
		items, failure := t.matching(name, vaultLoginKind)
		if failure != nil {
			return nil, false, failure
		}
		switch {
		case len(items) == 0:
			t.mutations = append(t.mutations, createLoginMutation(name, value))
			changed = true
		case len(items) > 1:
			findings = append(findings, transferFinding{class: classItemAmbiguous, name: key.Key})
		case items[0].Password == value:
			continue
		case !t.overwrite:
			findings = append(findings, transferFinding{class: classVaultDiffers, name: key.Key})
		default:
			t.mutations = append(t.mutations, updateLoginMutation(items[0], value))
			changed = true
		}
	}
	return findings, changed, nil
}

// pushFile plans the secure note of a file entry. A new note takes generation
// one. A differing note takes the next generation only with overwrite, and
// then the generation before the current one is removed.
func (t *transfer) pushFile(entry config.VaultEntry, text string) ([]transferFinding, bool, *vaultFailure) {
	base := path.Base(entry.Path)
	name := fileItemName(t.manifest, entry)
	items, failure := t.matching(name, vaultNoteKind)
	if failure != nil {
		return nil, false, failure
	}
	switch len(items) {
	case 0:
		t.mutations = append(t.mutations, createNoteMutation(name, base, []byte(text)))
		return nil, true, nil
	case 1:
	default:
		return []transferFinding{{class: classItemAmbiguous, name: entry.Path}}, false, nil
	}
	note := items[0]
	generations := generationsOf(base, note.Attachments)
	newest := highestGeneration(generations)
	if newest > 0 {
		data, fault := t.vault.ReadAttachment(note.ID, generations[newest].ID)
		if fault != nil {
			return nil, false, faultFailure(fault)
		}
		if string(data) == text {
			return nil, false, nil
		}
		if !t.overwrite {
			return []transferFinding{{class: classVaultDiffers, name: entry.Path}}, false, nil
		}
	}
	var previous *VaultAttachment
	if older, ok := generations[newest-1]; ok && newest >= 2 {
		previous = &older
	}
	t.mutations = append(t.mutations, addGenerationMutation(note, base, newest+1, []byte(text), previous))
	return nil, true, nil
}

// planDiff compares one entry with the vault and reports every difference by name.
func (t *transfer) planDiff(entry config.VaultEntry) ([]transferFinding, bool, *vaultFailure) {
	local, failure := t.readLocal(entry.Path)
	if failure != nil {
		return nil, false, failure
	}
	if !local.exists {
		return []transferFinding{{class: classLocalMissing, name: entry.Path}}, false, nil
	}
	if entry.Form == "file" {
		return t.diffFile(entry, local.text)
	}
	return t.diffEnv(entry, local.text)
}

// diffEnv compares each declared key's local value with its login item.
func (t *transfer) diffEnv(entry config.VaultEntry, text string) ([]transferFinding, bool, *vaultFailure) {
	values, bad := parseEnvDialect(text)
	if bad > 0 {
		return nil, false, &vaultFailure{code: domain.FileUnreadable,
			message: "`" + entry.Path + "` line " + strconv.Itoa(bad) + " is not an env line this build reads"}
	}
	declared := map[string]bool{}
	for _, key := range entry.Keys {
		declared[key.Key] = true
	}
	var findings []transferFinding
	for _, name := range sortedValueNames(values) {
		if !declared[name] {
			findings = append(findings, transferFinding{class: classUndeclaredKey, name: name})
		}
	}
	for _, key := range entry.Keys {
		value, present := values[key.Key]
		if !present {
			findings = append(findings, transferFinding{class: classLocalMissing, name: key.Key})
			continue
		}
		items, failure := t.matching(t.keyItemName(entry, key), vaultLoginKind)
		if failure != nil {
			return nil, false, failure
		}
		if class := itemClass(items); class != "" {
			findings = append(findings, transferFinding{class: class, name: key.Key})
			continue
		}
		if items[0].Password != value {
			findings = append(findings, transferFinding{class: classVaultDiffers, name: key.Key})
		}
	}
	return findings, false, nil
}

// diffFile compares a file entry's local text with its newest generation.
func (t *transfer) diffFile(entry config.VaultEntry, text string) ([]transferFinding, bool, *vaultFailure) {
	items, failure := t.matching(fileItemName(t.manifest, entry), vaultNoteKind)
	if failure != nil {
		return nil, false, failure
	}
	if class := itemClass(items); class != "" {
		return []transferFinding{{class: class, name: entry.Path}}, false, nil
	}
	generations := generationsOf(path.Base(entry.Path), items[0].Attachments)
	newest := highestGeneration(generations)
	if newest == 0 {
		return []transferFinding{{class: classAttachmentMiss, name: entry.Path}}, false, nil
	}
	data, fault := t.vault.ReadAttachment(items[0].ID, generations[newest].ID)
	if fault != nil {
		return nil, false, faultFailure(fault)
	}
	if string(data) != text {
		return []transferFinding{{class: classVaultDiffers, name: entry.Path}}, false, nil
	}
	return nil, false, nil
}

// matching returns every item of one kind whose name equals name exactly.
// The provider's search matches substrings, so the exact filter happens here too.
func (t *transfer) matching(name, kind string) ([]VaultItem, *vaultFailure) {
	found, fault := t.vault.Find(name)
	if fault != nil {
		return nil, faultFailure(fault)
	}
	var items []VaultItem
	for _, item := range found {
		if item.Name == name && item.Kind == kind {
			items = append(items, item)
		}
	}
	return items, nil
}

// itemClass names the finding for a lookup that did not find exactly one item.
// It returns the empty string when exactly one item matched.
func itemClass(items []VaultItem) string {
	switch len(items) {
	case 0:
		return classItemMissing
	case 1:
		return ""
	default:
		return classItemAmbiguous
	}
}

// readLocal reads one declared local file. A missing file is not an error; a
// link on the path, or a file that holds no text, refuses the run.
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
	return localFile{exists: true, text: text}, nil
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

// fileItemName is the secure note a file entry names.
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

func createLoginMutation(name, password string) vaultMutation {
	return func(vault SecretVault) *vaultFailure {
		_, fault := vault.CreateLogin(name, password)
		return faultFailure(fault)
	}
}

func updateLoginMutation(item VaultItem, password string) vaultMutation {
	return func(vault SecretVault) *vaultFailure {
		return faultFailure(vault.UpdateLogin(item, password))
	}
}

func createNoteMutation(name, base string, contents []byte) vaultMutation {
	return func(vault SecretVault) *vaultFailure {
		note, fault := vault.CreateNote(name)
		if fault != nil {
			return faultFailure(fault)
		}
		return storeGeneration(vault, note.ID, base, 1, contents, nil)
	}
}

func addGenerationMutation(note VaultItem, base string, generation int, contents []byte, previous *VaultAttachment) vaultMutation {
	return func(vault SecretVault) *vaultFailure {
		return storeGeneration(vault, note.ID, base, generation, contents, previous)
	}
}

// storeGeneration uploads one generation, reads it back, and compares it with
// the bytes it meant to store. Only a verified generation may retire the
// previous one; an unverified upload is removed and the previous one kept.
func storeGeneration(vault SecretVault, itemID, base string, generation int, contents []byte, previous *VaultAttachment) *vaultFailure {
	attachment, fault := vault.AddAttachment(itemID, generationName(base, generation), contents)
	if fault != nil {
		return faultFailure(fault)
	}
	stored, fault := vault.ReadAttachment(itemID, attachment.ID)
	if fault != nil {
		return faultFailure(fault)
	}
	if !bytes.Equal(stored, contents) {
		_ = vault.DeleteAttachment(itemID, attachment.ID)
		return &vaultFailure{code: domain.VaultRefused,
			message: "the uploaded `" + base + "` did not verify; the previous generation is kept"}
	}
	if previous != nil {
		return faultFailure(vault.DeleteAttachment(itemID, previous.ID))
	}
	return nil
}

// generationName is the attachment name of one generation of base: base.gNNNN.
func generationName(base string, generation int) string {
	return fmt.Sprintf("%s.g%04d", base, generation)
}

// generationsOf maps each generation number of base held by attachments to
// its attachment. A name that is not base.gNNNN, or that names generation zero, is not one.
func generationsOf(base string, attachments []VaultAttachment) map[int]VaultAttachment {
	generations := map[int]VaultAttachment{}
	for _, attachment := range attachments {
		rest, ok := strings.CutPrefix(attachment.FileName, base+".")
		if !ok {
			continue
		}
		match := envGeneration.FindStringSubmatch(rest)
		if match == nil {
			continue
		}
		number, _ := strconv.Atoi(match[1])
		if number == 0 {
			continue
		}
		generations[number] = attachment
	}
	return generations
}

// highestGeneration returns the newest generation number, or zero when none exists.
func highestGeneration(generations map[int]VaultAttachment) int {
	newest := 0
	for number := range generations {
		if number > newest {
			newest = number
		}
	}
	return newest
}

// parseEnvDialect reads KEY=value lines. It returns the values, or the first
// line it cannot read, numbered from one. Blank lines and comments are skipped;
// an optional export prefix is allowed; a key may appear once.
func parseEnvDialect(text string) (map[string]string, int) {
	values := map[string]string{}
	for index, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, " \t\r")
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "export "); ok {
			trimmed = strings.TrimLeft(rest, " \t")
		}
		key, value, found := strings.Cut(trimmed, "=")
		if !found || !envKeyPattern.MatchString(key) {
			return nil, index + 1
		}
		if _, duplicate := values[key]; duplicate {
			return nil, index + 1
		}
		decoded, ok := decodeEnvValue(value)
		if !ok {
			return nil, index + 1
		}
		values[key] = decoded
	}
	return values, 0
}

// decodeEnvValue reads one value: a run of bare bytes, single-quoted text
// with no escapes, double-quoted text with no escapes or substitution, and
// \' outside quotes for a quote, the run the single-quoted form writes for it.
func decodeEnvValue(value string) (string, bool) {
	var out strings.Builder
	for i := 0; i < len(value); {
		switch c := value[i]; {
		case c == '\'':
			end := strings.IndexByte(value[i+1:], '\'')
			if end < 0 {
				return "", false
			}
			out.WriteString(value[i+1 : i+1+end])
			i += end + 2
		case c == '"':
			end := strings.IndexByte(value[i+1:], '"')
			if end < 0 {
				return "", false
			}
			inner := value[i+1 : i+1+end]
			if strings.ContainsAny(inner, "\\$`") {
				return "", false
			}
			out.WriteString(inner)
			i += end + 2
		case c == '\\' && i+1 < len(value) && value[i+1] == '\'':
			out.WriteByte('\'')
			i += 2
		case envBarePattern.MatchString(string(c)):
			out.WriteByte(c)
			i++
		default:
			return "", false
		}
	}
	return out.String(), true
}

// encodeEnvValue writes one value: bare when every byte is plain, otherwise
// single-quoted, each quote written as a close-quote, backslash-quote, open-quote run. The caller refuses a value
// with a line break first, because no single line can hold it.
func encodeEnvValue(value string) string {
	if envBarePattern.MatchString(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// sortedValueNames returns the keys of values in byte order.
func sortedValueNames(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
