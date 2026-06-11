package domain

import "fmt"

// SignupProfileMetadataKey is the key under user_registration_requests.metadata
// that holds the optional signup questionnaire answers.
const SignupProfileMetadataKey = "signupProfile"

// Signup profile answers use a closed vocabulary: the registration endpoint is
// public, so free text would let arbitrary content into the database and make
// the captured stats unaggregatable.
var (
	SignupProfileRoles = []string{
		"developer",
		"security-engineer",
		"founder-or-exec",
		"student-or-researcher",
		"other",
	}
	SignupProfileUseCases = []string{
		"securing-production-agents",
		"evaluating-for-team",
		"research-or-learning",
		"personal-project",
		"other",
	}
	SignupProfileReferralSources = []string{
		"github",
		"search",
		"social-media",
		"colleague-or-friend",
		"blog-or-article",
		"other",
	}
)

// BuildSignupProfile validates the optional questionnaire answers and returns
// the map to store under SignupProfileMetadataKey. Empty answers are skipped
// (the questions are optional at the API level); a non-empty answer outside
// its vocabulary is an error. Returns nil when no answers were provided.
func BuildSignupProfile(role, primaryUseCase, referralSource string) (map[string]string, error) {
	profile := make(map[string]string)

	// Error messages deliberately omit the submitted value: they are returned
	// verbatim by a public endpoint, so echoing input would reflect
	// arbitrarily long attacker-controlled strings.
	if role != "" {
		if !containsString(SignupProfileRoles, role) {
			return nil, fmt.Errorf("invalid signup profile role")
		}
		profile["role"] = role
	}
	if primaryUseCase != "" {
		if !containsString(SignupProfileUseCases, primaryUseCase) {
			return nil, fmt.Errorf("invalid signup profile primaryUseCase")
		}
		profile["primaryUseCase"] = primaryUseCase
	}
	if referralSource != "" {
		if !containsString(SignupProfileReferralSources, referralSource) {
			return nil, fmt.Errorf("invalid signup profile referralSource")
		}
		profile["referralSource"] = referralSource
	}

	if len(profile) == 0 {
		return nil, nil
	}
	return profile, nil
}

func containsString(values []string, v string) bool {
	for _, candidate := range values {
		if candidate == v {
			return true
		}
	}
	return false
}
