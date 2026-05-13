package ui

import (
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"golang.org/x/term"
)

// outputMode is set from cmd/cli when --output is parsed; non-text modes
// suppress spinners (and other TTY effects) so escape codes don't leak
// into piped or machine-parsed output.
var outputMode = OutputText

// SetOutputMode is called from the cobra layer once the --output flag is
// parsed. Keeping the flag here (rather than reading it back from a global
// every call) avoids ui importing the cobra command tree. Unknown values
// fall back to text mode and return an error so the CLI can surface the
// misuse.
func SetOutputMode(s string) error {
	switch OutputMode(s) {
	case OutputText, OutputJSON, OutputYAML:
		outputMode = OutputMode(s)
		return nil
	default:
		outputMode = OutputText
		return fmt.Errorf("unknown output format %q (expected: text, json, yaml)", s)
	}
}

// Mode returns the current output mode. Exported so non-Render code paths
// (e.g. commands that print plain text only in text mode) can branch on it.
func Mode() OutputMode { return outputMode }

// StartSpinner returns a started spinner, or nil when output is non-TTY or
// a machine-readable mode is active. Callers should always pair this with
// StopSpinner — it tolerates a nil receiver.
func StartSpinner(suffix string) *spinner.Spinner {
	if outputMode != OutputText || !term.IsTerminal(int(os.Stdout.Fd())) {
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
