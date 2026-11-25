package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"strikeye/internal/sharecode"
)

func runSharecode(code string, out io.Writer) error {
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("--code is required")
	}
	m, err := sharecode.Decode(code)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}
