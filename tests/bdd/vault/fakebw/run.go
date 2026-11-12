package fakebw

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Run is the command-line face of the fake. argv is `--state <file> --log
// <file> -- <bw arguments>`. The wrapper that InstallWrapper writes supplies
// the first four words, so a caller of the adapter sees only bw's argv.
//
// Each process appends one record to the log: its bw arguments and the names
// of the environment variables it received, never a value and never standard
// input. Run also enforces the adapter's documented rules, and refuses a call
// that breaks one: every call needs --nointeraction, a value must arrive on
// standard input and never as an argument, and no call may be sync.
//
// Two parent variables change the fake the way the real bw's do, so a test
// proves the adapter strips them. BW_RESPONSE wraps every answer in the
// response envelope the adapter cannot parse. BW_CLEANEXIT turns a failure
// into a silent success.
func Run(argv []string, stdin io.Reader, stdout, stderr io.Writer, environ []string) int {
	statePath, logPath, words, err := splitWrapperArgs(argv)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	env := environmentNames(environ)
	if err := appendRecord(logPath, words, env); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	vault, err := Load(statePath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	outcome := dispatch(vault, words, env, stdin)
	if outcome.changed {
		if err := vault.Save(statePath); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	if outcome.code != 0 {
		if _, cleanExit := env["BW_CLEANEXIT"]; cleanExit {
			return 0
		}
		fmt.Fprintln(stderr, outcome.message)
		return outcome.code
	}
	answer := outcome.stdout
	if _, wrapped := env["BW_RESPONSE"]; wrapped {
		answer = `{"success":true,"data":` + answer + `}`
	}
	_, _ = io.WriteString(stdout, answer)
	return 0
}

// outcome is one command's answer: its exit code, its standard output with no
// trailing newline (the non-terminal shape), and whether it changed the vault.
type outcome struct {
	code    int
	stdout  string
	message string
	changed bool
}

func fail(message string) outcome { return outcome{code: 1, message: message} }

// dispatch answers one bw call against the vault. Every call carries
// --nointeraction. Only status answers without a session key, as the real bw does.
func dispatch(vault *Vault, words []string, env map[string]bool, stdin io.Reader) outcome {
	if len(words) == 0 {
		return fail("missing command")
	}
	if !hasFlag(words, "--nointeraction") {
		return fail("--nointeraction is required")
	}
	args := withoutFlags(words, "--nointeraction")
	if args[0] == "status" {
		return statusOutcome(vault, args)
	}
	if _, session := env["BW_SESSION"]; !session {
		return fail("Vault is locked.")
	}
	switch {
	case len(args) == 4 && args[0] == "list" && args[1] == "items" && args[2] == "--search":
		return jsonOutcome(vault.Search(args[3]))
	case len(args) == 3 && args[0] == "get" && args[1] == "item":
		item, ok := vault.Get(args[2])
		if !ok {
			return fail("Not found.")
		}
		return jsonOutcome(item)
	case len(args) == 7 && args[0] == "get" && args[1] == "attachment" && args[3] == "--itemid" && args[5] == "--output":
		return downloadOutcome(vault, args[2], args[4], args[6])
	case len(args) == 1 && args[0] == "encode":
		return encodeOutcome(stdin)
	case len(args) == 2 && args[0] == "create" && args[1] == "item":
		return createOutcome(vault, stdin)
	case len(args) == 3 && args[0] == "edit" && args[1] == "item":
		return editOutcome(vault, args[2], stdin)
	case len(args) == 6 && args[0] == "create" && args[1] == "attachment" && args[2] == "--file" && args[4] == "--itemid":
		return addAttachmentOutcome(vault, args[3], args[5])
	case len(args) == 5 && args[0] == "delete" && args[1] == "attachment" && args[3] == "--itemid":
		if !vault.DeleteAttachment(args[4], args[2]) {
			return fail("Not found.")
		}
		return outcome{changed: true}
	}
	return fail("unsupported command: " + strings.Join(args, " "))
}

func statusOutcome(vault *Vault, args []string) outcome {
	if len(args) != 1 {
		return fail("unsupported status arguments")
	}
	data, err := json.Marshal(map[string]string{"serverUrl": vault.ServerURL, "status": vault.Status})
	if err != nil {
		return fail(err.Error())
	}
	return outcome{stdout: string(data)}
}

func jsonOutcome(value any) outcome {
	data, err := json.Marshal(value)
	if err != nil {
		return fail(err.Error())
	}
	return outcome{stdout: string(data)}
}

// downloadOutcome writes one attachment into the directory named by --output,
// under the file name the attachment carries, as the real bw does.
func downloadOutcome(vault *Vault, attachmentID, itemID, directory string) outcome {
	item, ok := vault.Get(itemID)
	if !ok {
		return fail("Not found.")
	}
	var fileName string
	for _, attachment := range item.Attachments {
		if attachment.ID == attachmentID {
			fileName = attachment.FileName
		}
	}
	data, ok := vault.ReadAttachment(itemID, attachmentID)
	if !ok || fileName == "" {
		return fail("Not found.")
	}
	if err := os.WriteFile(filepath.Join(directory, fileName), data, 0o600); err != nil {
		return fail(err.Error())
	}
	return outcome{}
}

// encodeOutcome base64-encodes standard input, the way bw encode does.
func encodeOutcome(stdin io.Reader) outcome {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return fail(err.Error())
	}
	return outcome{stdout: base64.StdEncoding.EncodeToString(data)}
}

// decodeItem reads the encoded item JSON that standard input carries.
func decodeItem(stdin io.Reader) (Item, string) {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		return Item{}, err.Error()
	}
	decoded, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		return Item{}, "input is not an encoded item"
	}
	var item Item
	if err := json.Unmarshal(decoded, &item); err != nil {
		return Item{}, "input is not item JSON"
	}
	return item, ""
}

func createOutcome(vault *Vault, stdin io.Reader) outcome {
	item, problem := decodeItem(stdin)
	if problem != "" {
		return fail(problem)
	}
	created := vault.Create(item)
	out := jsonOutcome(created)
	out.changed = true
	return out
}

func editOutcome(vault *Vault, id string, stdin io.Reader) outcome {
	item, problem := decodeItem(stdin)
	if problem != "" {
		return fail(problem)
	}
	item.ID = id
	if !vault.Edit(item) {
		return fail("Not found.")
	}
	updated, _ := vault.Get(id)
	out := jsonOutcome(updated)
	out.changed = true
	return out
}

// addAttachmentOutcome attaches the file at path to the item, under the
// file's base name, which is how the real bw names it.
func addAttachmentOutcome(vault *Vault, path, itemID string) outcome {
	data, err := os.ReadFile(path)
	if err != nil {
		return fail(err.Error())
	}
	item, ok := vault.AddAttachment(itemID, filepath.Base(path), data)
	if !ok {
		return fail("Not found.")
	}
	out := jsonOutcome(item)
	out.changed = true
	return out
}

// splitWrapperArgs separates the wrapper's state and log paths from the bw
// words that follow the `--` marker.
func splitWrapperArgs(argv []string) (string, string, []string, error) {
	var statePath, logPath string
	for index := 0; index < len(argv); index++ {
		switch argv[index] {
		case "--state":
			if index+1 >= len(argv) {
				return "", "", nil, fmt.Errorf("--state needs a path")
			}
			statePath = argv[index+1]
			index++
		case "--log":
			if index+1 >= len(argv) {
				return "", "", nil, fmt.Errorf("--log needs a path")
			}
			logPath = argv[index+1]
			index++
		case "--":
			if statePath == "" || logPath == "" {
				return "", "", nil, fmt.Errorf("the wrapper must name --state and --log")
			}
			return statePath, logPath, argv[index+1:], nil
		default:
			return "", "", nil, fmt.Errorf("unexpected wrapper argument")
		}
	}
	return "", "", nil, fmt.Errorf("the wrapper must end its flags with --")
}

// environmentNames returns the set of variable names in environ, never their
// values, so no record ever holds a value.
func environmentNames(environ []string) map[string]bool {
	names := map[string]bool{}
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		names[name] = true
	}
	return names
}

// record is one line of the call log: the bw arguments and the sorted names of
// the environment variables the process received.
type record struct {
	Argv        []string `json:"argv"`
	Environment []string `json:"environment"`
}

func appendRecord(logPath string, words []string, env map[string]bool) error {
	var names []string
	for name := range env {
		names = append(names, name)
	}
	sort.Strings(names)
	line, err := json.Marshal(record{Argv: words, Environment: names})
	if err != nil {
		return err
	}
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// ReadLog returns the calls the fake received, in order, from a log file.
func ReadLog(logPath string) ([]record, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var calls []record
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var call record
		if err := json.Unmarshal([]byte(line), &call); err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}
	return calls, nil
}

func hasFlag(words []string, flag string) bool {
	for _, word := range words {
		if word == flag {
			return true
		}
	}
	return false
}

func withoutFlags(words []string, flag string) []string {
	var kept []string
	for _, word := range words {
		if word != flag {
			kept = append(kept, word)
		}
	}
	return kept
}
