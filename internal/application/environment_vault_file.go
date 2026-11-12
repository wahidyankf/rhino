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

// envGeneration is the tail of a generation attachment name: g and a canonical
// number, four digits at least, zero-padded below ten thousand and never with a
// leading zero beyond that padding. Generation zero is never a generation.
var envGeneration = regexp.MustCompile(`^g(0[0-9]{3}|[1-9][0-9]{3,})$`)

// desiredFile returns the newest generation of a file entry's secret. A newest
// generation that cannot be read refuses the run; an older generation is never
// used in its place.
func (t *transfer) desiredFile(entry config.VaultEntry) (string, []transferFinding, *vaultFailure) {
	note, class, failure := t.lookup(fileItemName(t.manifest, entry), vaultNoteKind)
	if failure != nil {
		return "", nil, failure
	}
	if class != "" {
		return "", []transferFinding{{class: class, name: entry.Path}}, nil
	}
	generations := generationsOf(path.Base(entry.Path), note.Attachments)
	newest := highestGeneration(generations)
	if newest == 0 {
		return "", []transferFinding{{class: classAttachmentMiss, name: entry.Path}}, nil
	}
	data, fault := t.vault.ReadAttachment(note.ID, generations[newest].ID)
	if fault != nil {
		return "", nil, faultFailure(fault)
	}
	return string(data), nil, nil
}

// pushFile plans the secure note of a file entry. A new note takes generation
// one. A differing note takes the next generation only with overwrite; once that
// generation verifies, every generation older than the current one is removed,
// so the current generation stays as the one previous.
func (t *transfer) pushFile(entry config.VaultEntry, text string) ([]transferFinding, bool, *vaultFailure) {
	base := path.Base(entry.Path)
	name := fileItemName(t.manifest, entry)
	note, class, failure := t.lookup(name, vaultNoteKind)
	if failure != nil {
		return nil, false, failure
	}
	switch class {
	case classItemMissing:
		t.mutations = append(t.mutations, createNoteMutation(name, base, []byte(text)))
		return nil, true, nil
	case classItemAmbiguous:
		return []transferFinding{{class: classItemAmbiguous, name: entry.Path}}, false, nil
	}
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
	t.mutations = append(t.mutations, addGenerationMutation(note, base, newest+1, []byte(text), generationsBelow(generations, newest)))
	return nil, true, nil
}

// diffFile compares a file entry's local text with its newest generation.
func (t *transfer) diffFile(entry config.VaultEntry, text string) ([]transferFinding, *vaultFailure) {
	note, class, failure := t.lookup(fileItemName(t.manifest, entry), vaultNoteKind)
	if failure != nil {
		return nil, failure
	}
	if class != "" {
		return []transferFinding{{class: class, name: entry.Path}}, nil
	}
	generations := generationsOf(path.Base(entry.Path), note.Attachments)
	newest := highestGeneration(generations)
	if newest == 0 {
		return []transferFinding{{class: classAttachmentMiss, name: entry.Path}}, nil
	}
	data, fault := t.vault.ReadAttachment(note.ID, generations[newest].ID)
	if fault != nil {
		return nil, faultFailure(fault)
	}
	if string(data) != text {
		return []transferFinding{{class: classVaultDiffers, name: entry.Path}}, nil
	}
	return nil, nil
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

func addGenerationMutation(note VaultItem, base string, generation int, contents []byte, retired []VaultAttachment) vaultMutation {
	return func(vault SecretVault) *vaultFailure {
		return storeGeneration(vault, note.ID, base, generation, contents, retired)
	}
}

// storeGeneration uploads one generation, reads it back, and compares it with
// the bytes it meant to store. Only a verified generation may retire the older
// ones; an unverified upload is removed, and every older generation is kept.
func storeGeneration(vault SecretVault, itemID, base string, generation int, contents []byte, retired []VaultAttachment) *vaultFailure {
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
			message: "the uploaded `" + base + "` did not verify; the previous generations are kept"}
	}
	for _, old := range retired {
		if fault := vault.DeleteAttachment(itemID, old.ID); fault != nil {
			return faultFailure(fault)
		}
	}
	return nil
}

// generationName is the attachment name of one generation of base: base.gNNNN.
func generationName(base string, generation int) string {
	return fmt.Sprintf("%s.g%04d", base, generation)
}

// generationsOf maps each generation number of base held by attachments to its
// attachment. A name that is not base.gNNNN in canonical form, that names
// generation zero, or whose number does not parse, is not a generation.
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
		number, err := strconv.Atoi(match[1])
		if err != nil || number == 0 {
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

// generationsBelow returns every generation older than newest, oldest first.
func generationsBelow(generations map[int]VaultAttachment, newest int) []VaultAttachment {
	var numbers []int
	for number := range generations {
		if number < newest {
			numbers = append(numbers, number)
		}
	}
	sort.Ints(numbers)
	var older []VaultAttachment
	for _, number := range numbers {
		older = append(older, generations[number])
	}
	return older
}
