// Package version defines build and parser metadata shared across Strikeye binaries.
package version

import "fmt"

const (
	ParserBaseline    = "demoinfocs-golang/v5@v5.3.0"
	DefaultDataSubdir = "Strikeye"
)

// AppVersion is the application version reported by the UI, the agent, and
// `strikeye -version`. It is a var (not a const) so release builds can inject
// the git tag: -ldflags "-X strikeye/internal/version.AppVersion=0.2.0".
// Keep the default in sync with the most recent tagged release.
var AppVersion = "0.1.0-phase1"

// Summary returns a human-readable build summary string for CLI/flag output.
func Summary(binary string) string {
	return fmt.Sprintf("%s %s (parser=%s)", binary, AppVersion, ParserBaseline)
}
