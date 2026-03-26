// Package config handles loading and validating do-manager configuration.
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the resolved runtime configuration.
type Config struct {
	Token string
}

// Load resolves the DigitalOcean API token from (in priority order):
//  1. --token CLI flag
//  2. DO_TOKEN environment variable
//  3. ~/.do-manager.yaml config file
func Load() (*Config, error) {
	token := strings.TrimSpace(viper.GetString("token"))
	if token == "" {
		return nil, fmt.Errorf(
			"DigitalOcean API token not set.\n" +
				"  Set via environment : export DO_TOKEN=<your_token>\n" +
				"  Or use the flag     : --token <your_token>\n" +
				"  Or create           : ~/.do-manager.yaml  (token: <your_token>)",
		)
	}
	// Basic sanity check: real tokens are significantly longer than this.
	if len(token) < 20 {
		return nil, fmt.Errorf("token appears invalid (only %d chars) - check DO_TOKEN", len(token))
	}
	return &Config{Token: token}, nil
}
