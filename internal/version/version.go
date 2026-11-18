// Package version defines build and parser metadata shared across Strikeye binaries.
package version

import "fmt"

const (
	AppVersion        = "0.1.0-phase1"
	ParserBaseline    = "demoinfocs-golang/v5@v5.3.0"
	DefaultDataSubdir = "Strikeye"
)

// Summary returns a human-readable build summary string for CLI/flag output.
func Summary(binary string) string {
	return fmt.Sprintf("%s %s (parser=%s)", binary, AppVersion, ParserBaseline)
}
