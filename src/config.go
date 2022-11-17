// Load and validate enviroment / config settings.
package main

import (
	"fmt"
	"os"
	"strconv"
)

// Config represents all of the possible configuration options for the proxy
type Config struct {
	HTTPPort        int
	HTTPSPort       int
	V1Host          string
	V2Host          string
	APIKey          string
	IgnoreTLSErrors bool
	IsDev           bool
}

// NewConfig validates and initializes our config based on the environment
func NewConfig() (*Config, error) {
	config := &Config{
		HTTPPort:  8080,
		HTTPSPort: 4433,
	}

	// Check that systemd passed us two file descriptors.
	// The first is for http, and the second is for https. Anything else is a
	// fatal error.
	nfds, err := strconv.Atoi(os.Getenv("LISTEN_FDS"))
	if err != nil {
		return nil, fmt.Errorf("invalid LISTEN_FDS enviroment variable. Makes ure you run with systemd: %w", err)
	}
	if nfds != 2 {
		return nil, fmt.Errorf("two ports are required: http and https, but got %d", nfds)
	}

	config.APIKey = os.Getenv("API_KEY")
	if config.APIKey == "" {
		return nil, fmt.Errorf("API_KEY not specified")
	}

	config.V1Host = os.Getenv("V1_HOST")
	if config.V1Host == "" {
		return nil, fmt.Errorf("V1_HOST not specified")
	}

	config.V2Host = os.Getenv("V2_HOST")
	if config.V2Host == "" {
		return nil, fmt.Errorf("V2_HOST not specified")
	}

	ignoreTLS := os.Getenv("IGNORE_TLS")
	if ignoreTLS != "" && ignoreTLS != "false" && ignoreTLS != "true" {
		return nil, fmt.Errorf("invalid IGNORE_TLS value: " + ignoreTLS)
	}
	config.IgnoreTLSErrors = ignoreTLS == "true"

	config.IsDev = os.Getenv("DEV") == "true"

	return config, nil
}
