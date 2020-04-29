package main

import (
	"log"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/env"
)

var (
	rawPollInterval = env.Get("PRECISE_CODE_INTEL_POLL_INTERVAL", "1s", "Interval between queries to the work queue.")
)

// mustParseInterval returns the interval version of the given raw value fatally logs on failure.
func mustParseInterval(rawValue, name string) time.Duration {
	d, err := time.ParseDuration(rawValue)
	if err != nil {
		log.Fatalf("invalid duration %q for %s: %s", rawValue, name, err)
	}

	return d
}
