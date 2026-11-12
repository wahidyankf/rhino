package application

import (
	"regexp"
	"sort"
	"strings"
)

var (
	// envKeyPattern is a key the env dialect reads: a shell name.
	envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	// envBarePattern is a value the env dialect writes without quotes.
	envBarePattern = regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,-]*$`)
)

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
// single-quoted, each quote written as a close-quote, backslash-quote, open-quote
// run. The caller refuses a value with a line break first, because no single
// line can hold it.
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
