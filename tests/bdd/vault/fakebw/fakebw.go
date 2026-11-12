// Package fakebw is a stateful test double of the Bitwarden CLI, bw. It
// implements exactly the subset RHINO's vault adapter documents: status,
// list items --search, get item, get attachment, encode, create item, edit
// item, create attachment, and delete attachment. Every other command, sync
// included, is refused. No test starts the real bw.
//
// The same Vault type serves two callers. The command-line adapter reaches it
// through Run, one argv per process, with its state kept in a file. The unit
// adapter holds a Vault in memory and calls its methods directly. This package
// is test support. No product package imports it.
package fakebw

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/wahidyankf/rhino/tests/bdd/vault"
)

// Vault is the state of one fake vault session: its status and server, its
// items, and the bytes of each attachment, keyed by attachment ID. Corrupt
// makes every attachment download return altered bytes, which is how a
// scenario proves that a push verifies what it uploaded.
type Vault struct {
	Status    string            `json:"status"`
	ServerURL string            `json:"serverUrl"`
	Corrupt   bool              `json:"corruptDownloads,omitempty"`
	Next      int               `json:"next"`
	Items     []Item            `json:"items"`
	Blobs     map[string][]byte `json:"blobs"`
}

// Item is one vault item. Type 1 is a login, whose value is in Login.Password
// or a hidden field; type 2 is a secure note, which carries attachments.
type Item struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Type            int            `json:"type"`
	Login           *Login         `json:"login"`
	Fields          []Field        `json:"fields"`
	Attachments     []Attachment   `json:"attachments"`
	PasswordHistory []HistoryEntry `json:"passwordHistory"`
}

// Login holds the password of a login item. A nil password is JSON null.
type Login struct {
	Password *string `json:"password"`
}

// Field is a custom field. Type 1 is hidden.
type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  int    `json:"type"`
}

// Attachment names one file held by an item.
type Attachment struct {
	ID       string `json:"id"`
	FileName string `json:"fileName"`
}

// HistoryEntry is one retired password the vault keeps for a login item.
type HistoryEntry struct {
	Password string `json:"password"`
}

// Load reads the state file a Run call or a driver keeps.
func Load(path string) (*Vault, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fake vault state: %w", err)
	}
	vault := &Vault{}
	if err := json.Unmarshal(data, vault); err != nil {
		return nil, fmt.Errorf("decode fake vault state: %w", err)
	}
	if vault.Blobs == nil {
		vault.Blobs = map[string][]byte{}
	}
	return vault, nil
}

// Save writes the state file. A test that seeds a vault saves it before a run.
func (v *Vault) Save(path string) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode fake vault state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write fake vault state: %w", err)
	}
	return nil
}

// Digest fingerprints the whole state. Two equal digests mean no byte of the
// vault changed, so a scenario can prove that a refused command wrote nothing.
func (v *Vault) Digest() string {
	data, err := json.Marshal(v)
	if err != nil {
		return "unencodable"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Search returns the items whose name contains term, ignoring case, in the
// order they were created. The real bw does the same substring match, so the
// adapter must filter on the exact name.
func (v *Vault) Search(term string) []Item {
	var found []Item
	for _, item := range v.Items {
		if containsFold(item.Name, term) {
			found = append(found, clone(item))
		}
	}
	return found
}

// Get returns one item by ID.
func (v *Vault) Get(id string) (Item, bool) {
	for _, item := range v.Items {
		if item.ID == id {
			return clone(item), true
		}
	}
	return Item{}, false
}

// Create stores a new item, assigning it the next item ID.
func (v *Vault) Create(item Item) Item {
	v.Next++
	item.ID = "item-" + strconv.Itoa(v.Next)
	item.Attachments = nil
	item.PasswordHistory = nil
	v.Items = append(v.Items, item)
	return clone(item)
}

// Edit replaces the item with the same ID. A password that changes leaves the
// previous one in the item's history, as the real vault does.
func (v *Vault) Edit(item Item) bool {
	for index, current := range v.Items {
		if current.ID != item.ID {
			continue
		}
		item.Attachments = current.Attachments
		item.PasswordHistory = current.PasswordHistory
		if old := password(current); old != nil && *old != passwordText(item) {
			item.PasswordHistory = append(item.PasswordHistory, HistoryEntry{Password: *old})
		}
		v.Items[index] = clone(item)
		return true
	}
	return false
}

// AddAttachment stores bytes under a new attachment ID on the item.
func (v *Vault) AddAttachment(itemID, fileName string, data []byte) (Item, bool) {
	for index := range v.Items {
		if v.Items[index].ID != itemID {
			continue
		}
		v.Next++
		attachmentID := "att-" + strconv.Itoa(v.Next)
		v.Items[index].Attachments = append(v.Items[index].Attachments, Attachment{ID: attachmentID, FileName: fileName})
		v.Blobs[attachmentID] = append([]byte(nil), data...)
		return clone(v.Items[index]), true
	}
	return Item{}, false
}

// ReadAttachment returns the bytes of one attachment on the item. With Corrupt
// set, the bytes come back altered.
func (v *Vault) ReadAttachment(itemID, attachmentID string) ([]byte, bool) {
	item, ok := v.Get(itemID)
	if !ok || !hasAttachment(item, attachmentID) {
		return nil, false
	}
	data := append([]byte(nil), v.Blobs[attachmentID]...)
	if v.Corrupt {
		if len(data) == 0 {
			return []byte("x"), true
		}
		data[len(data)-1] ^= 0x01
	}
	return data, true
}

// DeleteAttachment removes one attachment from the item and its bytes.
func (v *Vault) DeleteAttachment(itemID, attachmentID string) bool {
	for index := range v.Items {
		if v.Items[index].ID != itemID || !hasAttachment(v.Items[index], attachmentID) {
			continue
		}
		var kept []Attachment
		for _, attachment := range v.Items[index].Attachments {
			if attachment.ID != attachmentID {
				kept = append(kept, attachment)
			}
		}
		v.Items[index].Attachments = kept
		delete(v.Blobs, attachmentID)
		return true
	}
	return false
}

// Password returns the login password of an item, or nil when it has none.
func password(item Item) *string {
	if item.Login == nil {
		return nil
	}
	return item.Login.Password
}

// passwordText returns the login password of an item as text, empty when absent.
func passwordText(item Item) string {
	if p := password(item); p != nil {
		return *p
	}
	return ""
}

func hasAttachment(item Item, attachmentID string) bool {
	for _, attachment := range item.Attachments {
		if attachment.ID == attachmentID {
			return true
		}
	}
	return false
}

// clone copies an item so a caller cannot change the vault through a result.
func clone(item Item) Item {
	data, err := json.Marshal(item)
	if err != nil {
		panic("fake vault item cannot be encoded: " + err.Error())
	}
	var copied Item
	if err := json.Unmarshal(data, &copied); err != nil {
		panic("fake vault item cannot be decoded: " + err.Error())
	}
	return copied
}

func containsFold(text, term string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(term))
}

// Views returns every item held under name as the scenario reads it: its
// kind, its login password, its retired passwords, and each attachment's
// content by file name. Scenarios read the vault through this view, never
// through a command's output.
func (v *Vault) Views(name string) []vault.ItemView {
	var views []vault.ItemView
	for _, item := range v.Items {
		if item.Name != name {
			continue
		}
		view := vault.ItemView{Kind: "login", Attachments: map[string]string{}}
		if item.Type == 2 {
			view.Kind = "note"
		}
		if item.Login != nil && item.Login.Password != nil {
			view.Password = *item.Login.Password
		}
		for _, retired := range item.PasswordHistory {
			view.History = append(view.History, retired.Password)
		}
		for _, attachment := range item.Attachments {
			view.Attachments[attachment.FileName] = string(v.Blobs[attachment.ID])
		}
		views = append(views, view)
	}
	return views
}

// NoteNamed returns the secure note held under the exact name, the first one
// created when several share it.
func (v *Vault) NoteNamed(name string) (Item, bool) {
	for _, item := range v.Items {
		if item.Name == name && item.Type == 2 {
			return clone(item), true
		}
	}
	return Item{}, false
}
