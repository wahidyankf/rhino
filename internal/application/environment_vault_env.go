package application

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wahidyankf/rhino/internal/adapters/config"
	"github.com/wahidyankf/rhino/internal/domain"
)

// desiredEnv renders KEY=value lines in manifest order under the header. Each
// key reads its own login item; a key with any finding has no desired bytes.
func (t *transfer) desiredEnv(entry config.VaultEntry) (string, []transferFinding, *vaultFailure) {
	var out strings.Builder
	out.WriteString(vaultHeader + "\n")
	var findings []transferFinding
	for _, key := range entry.Keys {
		item, class, failure := t.lookup(t.keyItemName(entry, key), vaultLoginKind)
		if failure != nil {
			return "", nil, failure
		}
		if class != "" {
			findings = append(findings, transferFinding{class: class, name: key.Key})
			continue
		}
		if failure := readablePassword(item); failure != nil {
			return "", nil, failure
		}
		if strings.ContainsAny(item.Password, "\r\n") {
			return "", nil, &vaultFailure{code: domain.VaultRefused,
				message: "the vault holds a value of `" + key.Key + "` that an env file cannot hold on one line"}
		}
		fmt.Fprintf(&out, "%s=%s\n", key.Key, encodeEnvValue(item.Password))
	}
	if len(findings) > 0 {
		return "", findings, nil
	}
	return out.String(), nil, nil
}

// pushEnv plans the login items of an env entry, one per declared key. A key
// the vault lacks is created; a differing one is a finding unless overwrite is set.
func (t *transfer) pushEnv(entry config.VaultEntry, text string) ([]transferFinding, bool, *vaultFailure) {
	values, failure := envValues(entry, text)
	if failure != nil {
		return nil, false, failure
	}
	findings := undeclaredFindings(entry, values)
	changed := false
	for _, key := range entry.Keys {
		value, present := values[key.Key]
		if !present {
			findings = append(findings, transferFinding{class: classLocalMissing, name: key.Key})
			continue
		}
		name := t.keyItemName(entry, key)
		item, class, failure := t.lookup(name, vaultLoginKind)
		if failure != nil {
			return nil, false, failure
		}
		switch {
		case class == classItemMissing:
			t.mutations = append(t.mutations, createLoginMutation(name, value))
			changed = true
		case class != "":
			findings = append(findings, transferFinding{class: class, name: key.Key})
		default:
			if failure := readablePassword(item); failure != nil {
				return nil, false, failure
			}
			switch {
			case item.Password == value:
			case !t.overwrite:
				findings = append(findings, transferFinding{class: classVaultDiffers, name: key.Key})
			default:
				t.mutations = append(t.mutations, updateLoginMutation(item, value))
				changed = true
			}
		}
	}
	return findings, changed, nil
}

// diffEnv compares each declared key's local value with its login item.
func (t *transfer) diffEnv(entry config.VaultEntry, text string) ([]transferFinding, *vaultFailure) {
	values, failure := envValues(entry, text)
	if failure != nil {
		return nil, failure
	}
	findings := undeclaredFindings(entry, values)
	for _, key := range entry.Keys {
		value, present := values[key.Key]
		if !present {
			findings = append(findings, transferFinding{class: classLocalMissing, name: key.Key})
			continue
		}
		item, class, failure := t.lookup(t.keyItemName(entry, key), vaultLoginKind)
		if failure != nil {
			return nil, failure
		}
		if class != "" {
			findings = append(findings, transferFinding{class: class, name: key.Key})
			continue
		}
		if failure := readablePassword(item); failure != nil {
			return nil, failure
		}
		if item.Password != value {
			findings = append(findings, transferFinding{class: classVaultDiffers, name: key.Key})
		}
	}
	return findings, nil
}

// envValues reads a local env file. A line the dialect cannot read refuses the
// run and names only its line number, never the line.
func envValues(entry config.VaultEntry, text string) (map[string]string, *vaultFailure) {
	values, bad := parseEnvDialect(text)
	if bad > 0 {
		return nil, &vaultFailure{code: domain.FileUnreadable,
			message: "`" + entry.Path + "` line " + strconv.Itoa(bad) + " is not an env line this build reads"}
	}
	return values, nil
}

// undeclaredFindings reports every key a local env file holds that the entry does not declare.
func undeclaredFindings(entry config.VaultEntry, values map[string]string) []transferFinding {
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
	return findings
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
