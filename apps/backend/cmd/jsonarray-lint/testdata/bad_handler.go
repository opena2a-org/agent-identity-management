// Fixture: handler-shaped functions with the nil-slice declaration the lint
// is meant to catch. This file is parsed but never compiled; it lives under
// testdata so the surrounding package build ignores it.
package fixture

import "time"

type Alert struct{ ID string }

type TrustScoreEntry struct {
	AgentID    string
	Score      int
	RecordedAt time.Time
}

func GetTrustScoreHistory() ([]*TrustScoreEntry, error) {
	var history []*TrustScoreEntry
	return history, nil
}

func ListAlerts() ([]*Alert, error) {
	var alerts []*Alert
	return alerts, nil
}

func ListNames() []string {
	var names []string
	return names
}
