package runtime

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// regularFilePaths selects regular-file destinations from Git's raw
// NUL-delimited records. Raw Git metadata decides: destination modes 100644
// and 100755 are included, while symlinks, gitlinks, and deletions are
// excluded, and malformed or unmerged records refuse the whole selection.
func regularFilePaths(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) < 1 || raw[len(raw)-1] != 0 || len(raw) >= 2 && raw[len(raw)-1] == 0 && raw[len(raw)-2] == 0 {
		return nil, fmt.Errorf("Git returned a truncated raw path record")
	}
	fields := strings.Split(string(raw[:len(raw)-1]), "\x00")
	var paths []string
	for i := 0; i < len(fields); i += 2 {
		header := fields[i]
		if header == "" {
			return nil, fmt.Errorf("Git returned an empty raw path header")
		}
		if i+1 >= len(fields) || fields[i+1] == "" {
			return nil, fmt.Errorf("Git returned a raw record without a pathname")
		}
		path := fields[i+1]
		destinationMode, status, err := parseRawHeader(header)
		if err != nil {
			return nil, err
		}
		if !utf8.ValidString(path) {
			return nil, fmt.Errorf("Git returned a non-UTF-8 raw pathname")
		}
		if !gitRepositoryPath(path) {
			return nil, fmt.Errorf("Git returned an unsafe repository path `%s`", path)
		}
		if (destinationMode == "100644" || destinationMode == "100755") && status != 'D' {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	out := paths[:0]
	for _, path := range paths {
		if len(out) == 0 || out[len(out)-1] != path {
			out = append(out, path)
		}
	}
	return out, nil
}

// parseRawHeader reads one `:src dst soid doid status` header and reports
// the destination mode with the change status.
func parseRawHeader(header string) (string, byte, error) {
	if !strings.HasPrefix(header, ":") {
		return "", 0, fmt.Errorf("Git returned a malformed raw path header")
	}
	parts := strings.Split(header[1:], " ")
	if len(parts) != 5 {
		return "", 0, fmt.Errorf("Git returned a malformed raw path header")
	}
	for _, part := range parts {
		if part == "" {
			return "", 0, fmt.Errorf("Git returned a malformed raw path header")
		}
	}
	sourceMode, destinationMode, sourceID, destinationID, status := parts[0], parts[1], parts[2], parts[3], parts[4]
	if !validRawMode(sourceMode) || !validRawMode(destinationMode) {
		return "", 0, fmt.Errorf("Git returned an unsupported raw file mode")
	}
	if !validRawID(sourceID) || !validRawID(destinationID) ||
		(sourceMode == "000000") != allZero(sourceID) ||
		(destinationMode == "000000") != allZero(destinationID) {
		return "", 0, fmt.Errorf("Git returned malformed raw object IDs")
	}
	if len(status) != 1 {
		return "", 0, fmt.Errorf("Git returned an unsupported raw change status")
	}
	code := status[0]
	if code >= '0' && code <= '9' || code != 'A' && code != 'D' && code != 'M' && code != 'T' {
		return "", 0, fmt.Errorf("Git returned an unsupported raw change status")
	}
	sourceMissing := sourceMode == "000000"
	destinationMissing := destinationMode == "000000"
	inconsistent := false
	switch code {
	case 'A':
		inconsistent = !sourceMissing || destinationMissing
	case 'D':
		inconsistent = sourceMissing || !destinationMissing
	case 'M', 'T':
		inconsistent = sourceMissing || destinationMissing
	}
	if inconsistent {
		return "", 0, fmt.Errorf("Git returned inconsistent raw status and file modes")
	}
	return destinationMode, code, nil
}

// validRawMode reports whether mode is a file mode Git records.
func validRawMode(mode string) bool {
	switch mode {
	case "000000", "100644", "100755", "120000", "160000":
		return true
	}
	return false
}

// validRawID reports whether id is a non-empty hexadecimal object ID of at
// most 64 characters: a full SHA-1 or SHA-256, or an abbreviation of one.
func validRawID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		character := id[i]
		if character >= '0' && character <= '9' || character >= 'a' && character <= 'f' || character >= 'A' && character <= 'F' {
			continue
		}
		return false
	}
	return true
}

// allZero reports whether value is an all-zero object ID.
func allZero(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] != '0' {
			return false
		}
	}
	return true
}
