package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wahidyankf/rhino/internal/application"
	"github.com/wahidyankf/rhino/internal/domain"
)

// BitwardenVault is the one adapter that starts the Bitwarden CLI, bw. It is
// the only place an explicit vault command reaches a process, and it follows
// the protocol the vault contract documents:
//
//   - every call is argv only, with --nointeraction; a value travels on
//     standard input, never in argv, and item JSON is encoded by `bw encode`;
//   - the child receives only the variables the allowlist names, so the session
//     key, the locale, the proxy and certificate settings, and the platform
//     basics reach it, and no other variable does;
//   - status must answer unlocked, for the declared server, before any call;
//   - an item is found by its exact name, then used by its ID;
//   - an attachment moves through a 0700 temporary directory under its own name;
//   - standard output is parsed strictly, with one trailing newline trimmed;
//   - `bw sync` is never called;
//   - standard error is discarded, and no provider answer reaches the output.
type BitwardenVault struct {
	// Ctx cancels every call the adapter starts; nil means background.
	Ctx context.Context
}

// bitwardenExecutable is the program name the adapter resolves on PATH.
const bitwardenExecutable = "bw"

// The fixed sentences the adapter reports. None names a value or a provider message.
const (
	reasonNoSession    = "no BW_SESSION is set; unlock the vault in your own session first"
	reasonNoExecutable = "the bw executable is not on PATH"
	reasonLocked       = "the vault is not unlocked"
	reasonOtherServer  = "the vault serves another server than the declared one"
	reasonCallFailed   = "the vault provider did not complete the call"
	reasonAnswer       = "the vault provider answered something this build does not understand"
	reasonStaging      = "the attachment could not be staged for the vault provider"
	reasonNoAttachment = "the vault provider did not name the uploaded attachment"
)

// bwItem is the part of one item JSON the adapter reads.
type bwItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  int    `json:"type"`
	Login *struct {
		Password *string `json:"password"`
	} `json:"login"`
	Attachments []struct {
		ID       string `json:"id"`
		FileName string `json:"fileName"`
	} `json:"attachments"`
}

// toVault maps one provider item to the application's view of it. Type 1 is a
// login and type 2 a secure note; any other type is held as "other" and never matches.
func (item bwItem) toVault() application.VaultItem {
	view := application.VaultItem{ID: item.ID, Name: item.Name, Kind: "other"}
	switch item.Type {
	case 1:
		view.Kind = "login"
		if item.Login == nil || item.Login.Password == nil {
			view.NoPassword = true
		} else {
			view.Password = *item.Login.Password
		}
	case 2:
		view.Kind = "note"
	}
	for _, attachment := range item.Attachments {
		view.Attachments = append(view.Attachments, application.VaultAttachment{ID: attachment.ID, FileName: attachment.FileName})
	}
	return view
}

// Status refuses unless a session key is set, bw is on PATH, the vault is
// unlocked, and it serves the declared server. Those checks run before any call
// that could change the vault.
func (v BitwardenVault) Status(server string) *application.VaultFault {
	if os.Getenv("BW_SESSION") == "" {
		return vaultUnavailable(reasonNoSession)
	}
	executable, fault := bitwardenPath()
	if fault != nil {
		return fault
	}
	out, err := v.call(executable, nil, "status")
	if err != nil {
		return vaultUnavailable(reasonCallFailed)
	}
	var answer struct {
		Status    string `json:"status"`
		ServerURL string `json:"serverUrl"`
	}
	if err := decodeStrict(out, &answer); err != nil {
		return vaultRefused(reasonAnswer)
	}
	if answer.Status != "unlocked" {
		return vaultUnavailable(reasonLocked)
	}
	if strings.TrimSuffix(answer.ServerURL, "/") != strings.TrimSuffix(server, "/") {
		return vaultUnavailable(reasonOtherServer)
	}
	return nil
}

// Find lists the items whose name equals name exactly. The provider's search
// is a case-insensitive substring match, so the adapter filters on the full name.
func (v BitwardenVault) Find(name string) ([]application.VaultItem, *application.VaultFault) {
	executable, fault := bitwardenPath()
	if fault != nil {
		return nil, fault
	}
	out, err := v.call(executable, nil, "list", "items", "--search", name)
	if err != nil {
		return nil, vaultUnavailable(reasonCallFailed)
	}
	var listed []bwItem
	if err := decodeStrict(out, &listed); err != nil {
		return nil, vaultRefused(reasonAnswer)
	}
	var found []application.VaultItem
	for _, item := range listed {
		if item.Name == name {
			found = append(found, item.toVault())
		}
	}
	return found, nil
}

// CreateLogin stores a login item whose password is password.
func (v BitwardenVault) CreateLogin(name, password string) (application.VaultItem, *application.VaultFault) {
	return v.create(map[string]any{
		"type": 1, "name": name, "login": map[string]any{"password": password},
	})
}

// CreateNote stores a secure note with no attachment.
func (v BitwardenVault) CreateNote(name string) (application.VaultItem, *application.VaultFault) {
	return v.create(map[string]any{
		"type": 2, "name": name, "secureNote": map[string]any{"type": 0},
	})
}

// create encodes one item on standard input and creates it.
func (v BitwardenVault) create(item map[string]any) (application.VaultItem, *application.VaultFault) {
	executable, fault := bitwardenPath()
	if fault != nil {
		return application.VaultItem{}, fault
	}
	encoded, fault := v.encode(executable, item)
	if fault != nil {
		return application.VaultItem{}, fault
	}
	out, err := v.call(executable, encoded, "create", "item")
	if err != nil {
		return application.VaultItem{}, vaultUnavailable(reasonCallFailed)
	}
	var created bwItem
	if err := decodeStrict(out, &created); err != nil {
		return application.VaultItem{}, vaultRefused(reasonAnswer)
	}
	return created.toVault(), nil
}

// UpdateLogin replaces the password of an existing login item. It reads the
// whole item first, so every field the item holds survives the edit.
func (v BitwardenVault) UpdateLogin(item application.VaultItem, password string) *application.VaultFault {
	executable, fault := bitwardenPath()
	if fault != nil {
		return fault
	}
	raw, err := v.call(executable, nil, "get", "item", item.ID)
	if err != nil {
		return vaultUnavailable(reasonCallFailed)
	}
	var current map[string]any
	if err := decodeStrict(raw, &current); err != nil || current == nil {
		return vaultRefused(reasonAnswer)
	}
	login, _ := current["login"].(map[string]any)
	if login == nil {
		login = map[string]any{}
	}
	login["password"] = password
	current["login"] = login
	encoded, fault := v.encode(executable, current)
	if fault != nil {
		return fault
	}
	if _, err := v.call(executable, encoded, "edit", "item", item.ID); err != nil {
		return vaultUnavailable(reasonCallFailed)
	}
	return nil
}

// AddAttachment uploads contents under fileName, staged in a 0700 temporary
// directory. The file name must be a plain name; a path is never uploaded.
func (v BitwardenVault) AddAttachment(itemID, fileName string, contents []byte) (application.VaultAttachment, *application.VaultFault) {
	executable, fault := bitwardenPath()
	if fault != nil {
		return application.VaultAttachment{}, fault
	}
	if fileName == "" || strings.ContainsAny(fileName, `/\`) || fileName == "." || fileName == ".." {
		return application.VaultAttachment{}, vaultRefused(reasonStaging)
	}
	dir, err := os.MkdirTemp("", "rhino-vault-attachment-")
	if err != nil {
		return application.VaultAttachment{}, vaultUnavailable(reasonStaging)
	}
	defer os.RemoveAll(dir)
	staged := filepath.Join(dir, fileName)
	if err := os.WriteFile(staged, contents, 0o600); err != nil {
		return application.VaultAttachment{}, vaultUnavailable(reasonStaging)
	}
	out, err := v.call(executable, nil, "create", "attachment", "--file", staged, "--itemid", itemID)
	if err != nil {
		return application.VaultAttachment{}, vaultUnavailable(reasonCallFailed)
	}
	var item bwItem
	if err := decodeStrict(out, &item); err != nil {
		return application.VaultAttachment{}, vaultRefused(reasonAnswer)
	}
	var named application.VaultAttachment
	found := false
	for _, attachment := range item.toVault().Attachments {
		if attachment.FileName == fileName {
			named, found = attachment, true
		}
	}
	if !found {
		return application.VaultAttachment{}, vaultRefused(reasonNoAttachment)
	}
	return named, nil
}

// ReadAttachment downloads one attachment into a 0700 temporary directory and
// returns its bytes. The provider names the file, so the directory must hold
// exactly one regular file.
func (v BitwardenVault) ReadAttachment(itemID, attachmentID string) ([]byte, *application.VaultFault) {
	executable, fault := bitwardenPath()
	if fault != nil {
		return nil, fault
	}
	dir, err := os.MkdirTemp("", "rhino-vault-attachment-")
	if err != nil {
		return nil, vaultUnavailable(reasonStaging)
	}
	defer os.RemoveAll(dir)
	if _, err := v.call(executable, nil, "get", "attachment", attachmentID, "--itemid", itemID,
		"--output", dir+string(filepath.Separator)); err != nil {
		return nil, vaultUnavailable(reasonCallFailed)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || !entries[0].Type().IsRegular() {
		return nil, vaultRefused(reasonAnswer)
	}
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		return nil, vaultUnavailable(reasonStaging)
	}
	return data, nil
}

// DeleteAttachment removes one attachment from an item.
func (v BitwardenVault) DeleteAttachment(itemID, attachmentID string) *application.VaultFault {
	executable, fault := bitwardenPath()
	if fault != nil {
		return fault
	}
	if _, err := v.call(executable, nil, "delete", "attachment", attachmentID, "--itemid", itemID); err != nil {
		return vaultUnavailable(reasonCallFailed)
	}
	return nil
}

// encode runs `bw encode` on the item JSON, so a create or edit sends base64 on standard input.
func (v BitwardenVault) encode(executable string, item map[string]any) ([]byte, *application.VaultFault) {
	data, err := json.Marshal(item)
	if err != nil {
		return nil, vaultRefused(reasonAnswer)
	}
	encoded, err := v.call(executable, data, "encode")
	if err != nil {
		return nil, vaultUnavailable(reasonCallFailed)
	}
	return encoded, nil
}

// call starts bw with argv, --nointeraction last, and the given standard input.
// It returns standard output with one trailing newline trimmed. Standard error
// is discarded. The child environment is filtered before the start.
func (v BitwardenVault) call(executable string, stdin []byte, argv ...string) ([]byte, error) {
	ctx := v.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	command := exec.CommandContext(ctx, executable, append(argv, "--nointeraction")...)
	command.Env = childEnvironment(os.Environ())
	command.Stdin = bytes.NewReader(stdin)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	if err := command.Run(); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(stdout.Bytes(), []byte("\n")), nil
}

// bitwardenPath resolves bw on the current PATH, or refuses when it is absent.
func bitwardenPath() (string, *application.VaultFault) {
	executable, err := exec.LookPath(bitwardenExecutable)
	if err != nil {
		return "", vaultUnavailable(reasonNoExecutable)
	}
	return executable, nil
}

// childVariables is the allowlist of variables the provider child receives. A
// variable outside it is dropped, so a token or a cloud credential in the parent
// environment never reaches bw. BW_SESSION is the only BW_* variable kept, and
// BW_CLEANEXIT and BW_RESPONSE, which change the exit code and the output shape,
// are dropped with the rest.
var childVariables = map[string]bool{
	"BW_SESSION": true, "PATH": true, "HOME": true, "TMPDIR": true, "LANG": true,
	"TZ": true, "USER": true, "LOGNAME": true, "BITWARDENCLI_APPDATA_DIR": true,
	"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true,
	"http_proxy": true, "https_proxy": true, "no_proxy": true,
	"SSL_CERT_FILE": true, "SSL_CERT_DIR": true, "NODE_EXTRA_CA_CERTS": true,
	"SystemRoot": true, "APPDATA": true, "LOCALAPPDATA": true, "USERPROFILE": true,
	"ComSpec": true, "PATHEXT": true, "TEMP": true, "TMP": true,
}

// childEnvironment keeps only the allowlisted variables of the parent environment.
func childEnvironment(parent []string) []string {
	var kept []string
	for _, entry := range parent {
		name, _, _ := strings.Cut(entry, "=")
		if keepChildVariable(name) {
			kept = append(kept, entry)
		}
	}
	return kept
}

// keepChildVariable reports whether a variable reaches the provider: an
// allowlisted name, a locale variable, or an XDG base-directory variable.
func keepChildVariable(name string) bool {
	return childVariables[name] || strings.HasPrefix(name, "LC_") || strings.HasPrefix(name, "XDG_")
}

// decodeStrict reads exactly one JSON value and refuses any trailing content.
func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing content after the JSON value")
	}
	return nil
}

func vaultUnavailable(reason string) *application.VaultFault {
	return &application.VaultFault{Code: domain.VaultUnavailable, Reason: reason}
}

func vaultRefused(reason string) *application.VaultFault {
	return &application.VaultFault{Code: domain.VaultRefused, Reason: reason}
}
