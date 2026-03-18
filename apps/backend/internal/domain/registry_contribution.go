package domain

// ContributionEvent represents a single event in a registry contribution batch.
// Matches the OpenA2A Registry POST /api/v1/contribute format.
type ContributionEvent struct {
	Type            string           `json:"type"`
	Tool            string           `json:"tool"`
	ToolVersion     string           `json:"toolVersion"`
	Timestamp       string           `json:"timestamp"`
	Package         *PackageRef      `json:"package,omitempty"`
	ScanSummary     *ScanSummary     `json:"scanSummary,omitempty"`
	BehaviorSummary *BehaviorSummary `json:"behaviorSummary,omitempty"`
}

// PackageRef identifies an MCP package in the contribution.
type PackageRef struct {
	Name      string `json:"name"`
	Version   string `json:"version,omitempty"`
	Ecosystem string `json:"ecosystem,omitempty"`
}

// ScanSummary captures aggregate scan results for an MCP.
type ScanSummary struct {
	TotalChecks int    `json:"totalChecks"`
	Passed      int    `json:"passed"`
	Critical    int    `json:"critical"`
	High        int    `json:"high"`
	Medium      int    `json:"medium"`
	Low         int    `json:"low"`
	Score       int    `json:"score"`
	Verdict     string `json:"verdict"`
	DurationMs  int    `json:"durationMs"`
}

// BehaviorSummary captures aggregate behavioral data for an MCP.
type BehaviorSummary struct {
	Interactions int     `json:"interactions"`
	SuccessRate  float64 `json:"successRate"`
	Anomalies    int     `json:"anomalies"`
}

// ContributionBatch is the top-level payload sent to the OpenA2A Registry.
type ContributionBatch struct {
	ContributorToken string              `json:"contributorToken"`
	Events           []ContributionEvent `json:"events"`
	SubmittedAt      string              `json:"submittedAt"`
}
