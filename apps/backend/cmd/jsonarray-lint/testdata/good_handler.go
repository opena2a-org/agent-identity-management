// Fixture: handler-shaped functions with correct slice initialization. The
// lint should produce zero findings against this file.
package fixture

type Alert struct{ ID string }

type TrustScoreEntry struct {
	AgentID string
	Score   int
}

func GetTrustScoreHistoryGood() ([]*TrustScoreEntry, error) {
	history := make([]*TrustScoreEntry, 0)
	return history, nil
}

func ListAlertsGood() ([]*Alert, error) {
	alerts := make([]*Alert, 0)
	return alerts, nil
}

func ListNamesGood() []string {
	names := make([]string, 0)
	return names
}

// Returning a literal slice expression is also fine.
func ListLiteral() []string {
	return []string{}
}
