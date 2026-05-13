// Package envfile parses dotenv-style files for `conure var import`. It is
// intentionally permissive about the common cases (quoted values, comments,
// blank lines) and adds one Conure-specific marker: a comment of the form
// `# @secret` immediately preceding a KEY=VALUE line marks that entry as
// encrypted-at-rest when sent to the API.
package envfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Entry is one variable parsed from a .env file. IsSecret tracks the
// @secret marker so the caller can decide whether to flip IsEncrypted on
// the API request.
type Entry struct {
	// Line is the 1-indexed line number of the KEY=VALUE in the source
	// file. Useful for error messages on conflicts.
	Line     int
	Name     string
	Value    string
	IsSecret bool
}

// secretMarker is the comment that promotes the *next* KEY=VALUE line to a
// secret. Trailing text on the same line is ignored, so `# @secret reason`
// also works. Match is case-insensitive on "@secret" and tolerant of
// whitespace.
const secretMarker = "@secret"

// Parse reads dotenv content from r and returns the parsed entries. Order
// follows the source so the caller's "skip duplicates" logic can be
// deterministic. Returns an error only on I/O failure or malformed lines —
// duplicate keys are NOT collapsed here; the caller decides what to do.
func Parse(r io.Reader) ([]Entry, error) {
	scanner := bufio.NewScanner(r)
	// .env values can legitimately be longer than the default 64KB token
	// limit (think base64-encoded keys), so bump the buffer.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var entries []Entry
	nextIsSecret := false
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" {
			// Blank lines reset the @secret marker so a stray comment
			// far above doesn't bleed into an unrelated variable.
			nextIsSecret = false
			continue
		}
		if strings.HasPrefix(line, "#") {
			body := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			if hasSecretMarker(body) {
				nextIsSecret = true
			}
			// Other comments are ignored but do NOT clear the marker —
			// `# @secret\n# rationale text\nFOO=bar` should still flag FOO.
			continue
		}
		// `export FOO=bar` is a common shell idiom for files that get
		// `source`d. Strip the prefix so the variable name parses cleanly.
		line = strings.TrimPrefix(line, "export ")
		line = strings.TrimSpace(line)

		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE, got %q", lineNo, raw)
		}
		name := strings.TrimSpace(line[:eq])
		value := strings.TrimSpace(line[eq+1:])
		value = stripInlineComment(value)
		value = unquote(value)

		entries = append(entries, Entry{
			Line:     lineNo,
			Name:     name,
			Value:    value,
			IsSecret: nextIsSecret,
		})
		nextIsSecret = false
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// ParseFile is a thin convenience around Parse for the common command-line
// case where the user gave us a path.
func ParseFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

func hasSecretMarker(commentBody string) bool {
	// Case-insensitive prefix match so `@Secret` and `@SECRET` work too.
	lower := strings.ToLower(commentBody)
	if !strings.HasPrefix(lower, secretMarker) {
		return false
	}
	// Either bare `@secret` or `@secret <rest>` — reject `@secrets` etc.
	rest := commentBody[len(secretMarker):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

// unquote strips a single matched pair of leading/trailing quotes. Unlike
// godotenv we keep this simple: no escape sequences, no inline expansion.
// Files that need that complexity belong in a config file, not as input to
// a bulk import.
func unquote(s string) string {
	if len(s) < 2 {
		return s
	}
	first, last := s[0], s[len(s)-1]
	if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

// stripInlineComment removes a `# ...` tail from an unquoted value. We
// only strip when the value isn't a quoted string — otherwise `FOO="a # b"`
// would lose its hash.
func stripInlineComment(s string) string {
	if len(s) > 0 && (s[0] == '"' || s[0] == '\'') {
		return s
	}
	if i := strings.Index(s, " #"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
