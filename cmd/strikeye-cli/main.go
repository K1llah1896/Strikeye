// Command strikeye-cli is the stand-alone command-line entry point. The release
// package ships a single `strikeye` binary that reaches the same commands with
// `strikeye cli <command>`; this thin shim keeps `go run ./cmd/strikeye-cli`
// working for development and scripts that predate the merge.
package main

import (
	"os"

	"strikeye/internal/cli"
)

func main() {
	cli.Run(os.Args[1:])
}
