package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// outputFormat is bound to the --output / -o persistent flag in root.go.
var outputFormat string

// isJSON reports whether the user requested JSON output.
func isJSON() bool {
	return strings.EqualFold(strings.TrimSpace(outputFormat), "json")
}

// printJSON serialises v as indented JSON to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("json encode: %w", err)
	}
	return nil
}
