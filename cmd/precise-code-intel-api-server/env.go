package main

import (
	"log"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/env"
)

var (
	rawJanitorInterval = env.Get("PRECISE_CODE_INTEL_JANITOR_INTERVAL", "1m", "Interval between cleanup runs.")
)

// mustParseInterval returns the interval version of the given raw value fatally logs on failure.
func mustParseInterval(rawValue, name string) time.Duration {
	d, err := time.ParseDuration(rawValue)
	if err != nil {
		log.Fatalf("invalid duration %q for %s: %s", rawValue, name, err)
	}

	return d
}
