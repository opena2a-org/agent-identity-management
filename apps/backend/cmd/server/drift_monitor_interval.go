package main

import (
	"fmt"
	"strings"
	"time"
)

const (
	driftMonitorIntervalEnv = "DRIFT_MONITOR_INTERVAL"
	// defaultDriftMonitorInterval applies when DRIFT_MONITOR_INTERVAL is unset or unusable.
	defaultDriftMonitorInterval = 15 * time.Minute
	// minDriftMonitorInterval is the floor: each run queries every organization
	// with a registered MCP server, so a shorter interval is raised to this.
	minDriftMonitorInterval = time.Minute
)

// driftMonitorInterval returns how often the capability drift monitor runs,
// read from DRIFT_MONITOR_INTERVAL as a Go duration ("5m", "1h"). Unset uses
// the 15-minute default. A value that does not parse or is not positive also
// uses the default, and a value under one minute runs at one minute; both
// return a warning naming the variable and the interval in effect. The warning
// is empty when the value is used as given.
func driftMonitorInterval(getenv func(string) string) (time.Duration, string) {
	return backgroundJobInterval(getenv, driftMonitorIntervalEnv, defaultDriftMonitorInterval, minDriftMonitorInterval)
}

// backgroundJobInterval reads how often a background job runs from the
// variable named envName, a Go duration ("5m", "1h"), with the spaces around
// it ignored. Unset, empty or only spaces uses fallback. A value that does not
// parse or is not positive also uses fallback, and a value under floor runs at
// floor; both return a warning naming the variable and the interval in effect.
// The warning is empty when the value is used as given.
func backgroundJobInterval(getenv func(string) string, envName string, fallback, floor time.Duration) (time.Duration, string) {
	raw := strings.TrimSpace(getenv(envName))
	if raw == "" {
		return fallback, ""
	}
	interval, err := time.ParseDuration(raw)
	if err != nil || interval <= 0 {
		return fallback, fmt.Sprintf(
			"%s=%q is not a positive duration such as 5m or 1h; using the default %s",
			envName, raw, fallback)
	}
	if interval < floor {
		return floor, fmt.Sprintf(
			"%s=%q is under the %s minimum; using %s",
			envName, raw, floor, floor)
	}
	return interval, ""
}
