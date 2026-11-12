// Environment vault ports: the two boundaries the vault commands hold, one to
// the secret provider and one to the local file system.
//
// The application layer sees only these interfaces. A nil vault or file store
// fails every vault command closed before any write, the way a nil
// EnvironmentStore fails init, backup, and restore. Validators never receive
// either port.
package application

import "github.com/wahidyankf/rhino/internal/domain"

// SecretVault is the provider boundary env pull, push, and diff hold. Every
// method answers names, IDs, and values only to the vault commands; no answer
// reaches standard output. A method that cannot complete returns a VaultFault
// in the fixed vocabulary, never the provider's own message.
type SecretVault interface {
	// Status refuses unless the session is unlocked and the vault serves the
	// declared server.
	Status(server string) *VaultFault
	// Find returns every item whose name equals name exactly, of any kind.
	Find(name string) ([]VaultItem, *VaultFault)
	// CreateLogin stores a new login item whose password is password.
	CreateLogin(name, password string) (VaultItem, *VaultFault)
	// UpdateLogin replaces the password of an existing login item. The
	// provider keeps the previous password in the item's history.
	UpdateLogin(item VaultItem, password string) *VaultFault
	// CreateNote stores a new secure note with no attachment.
	CreateNote(name string) (VaultItem, *VaultFault)
	// AddAttachment stores contents on the item under fileName.
	AddAttachment(itemID, fileName string, contents []byte) (VaultAttachment, *VaultFault)
	// ReadAttachment returns the bytes of one attachment of the item.
	ReadAttachment(itemID, attachmentID string) ([]byte, *VaultFault)
	// DeleteAttachment removes one attachment from the item.
	DeleteAttachment(itemID, attachmentID string) *VaultFault
}

// VaultItem is one item the provider holds. Kind is "login", "note", or
// "other"; a login carries its Password, or NoPassword when the provider holds
// none, and a note its Attachments.
type VaultItem struct {
	ID          string
	Name        string
	Kind        string
	Password    string
	NoPassword  bool
	Attachments []VaultAttachment
}

// VaultAttachment is one file an item holds, by its provider ID and file name.
type VaultAttachment struct {
	ID       string
	FileName string
}

// VaultFault is why a provider call could not complete. Code is one of the
// two vault codes; Reason is a fixed sentence that names no value.
type VaultFault struct {
	Code   domain.ErrorCode
	Reason string
}

// VaultFile is one declared local file a pull writes. Replace authorizes
// replacing an existing regular file; Mode is the permission bits to set.
type VaultFile struct {
	Path     string
	Contents []byte
	Mode     uint32
	Replace  bool
}

// VaultFileStore is the write boundary only env pull holds. It writes one
// file atomically or fails and leaves the target as it was.
type VaultFileStore interface {
	Write(root string, file VaultFile) error
}

// VaultPorts is what ExecuteWithVault hands the vault commands. Either field
// may be nil; the commands then refuse before they touch anything.
type VaultPorts struct {
	Vault SecretVault
	Files VaultFileStore
}
