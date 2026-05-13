package ui

import (
	"os"
	"time"

	"github.com/briandowns/spinner"
	"golang.org/x/term"
)

// jsonMode is set from cmd/cli when --output=json is selected; it suppresses
// spinners (and any other TTY-only effects) so escape codes don't leak into
// piped or machine-parsed output.
var jsonMode bool

// SetJSONMode is called from the cobra layer once the --output flag is
// parsed. Keeping the flag here (rather than reading it back from a global
// every call) avoids ui importing the cobra command tree.
func SetJSONMode(on bool) { jsonMode = on }

// StartSpinner returns a started spinner, or nil when output is non-TTY or
// JSON mode is active. Callers should always pair this with StopSpinner —
// it tolerates a nil receiver.
func StartSpinner(suffix string) *spinner.Spinner {
	if jsonMode || !term.IsTerminal(int(os.Stdout.Fd())) {
		return nil
	}
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = "  " + suffix
	s.Start()
	return s
}

func StopSpinner(s *spinner.Spinner) {
	if s != nil {
		s.Stop()
	}
}
