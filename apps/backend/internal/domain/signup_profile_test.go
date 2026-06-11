package domain

import (
	"testing"
)

func TestBuildSignupProfile_AllAnswersValid(t *testing.T) {
	profile, err := BuildSignupProfile("developer", "securing-production-agents", "github")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile["role"] != "developer" ||
		profile["primaryUseCase"] != "securing-production-agents" ||
		profile["referralSource"] != "github" {
		t.Fatalf("unexpected profile: %v", profile)
	}
}

func TestBuildSignupProfile_PartialAnswers(t *testing.T) {
	profile, err := BuildSignupProfile("", "research-or-learning", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profile) != 1 || profile["primaryUseCase"] != "research-or-learning" {
		t.Fatalf("unexpected profile: %v", profile)
	}
}

func TestBuildSignupProfile_NoAnswersReturnsNil(t *testing.T) {
	profile, err := BuildSignupProfile("", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile != nil {
		t.Fatalf("expected nil profile, got: %v", profile)
	}
}

func TestBuildSignupProfile_RejectsUnknownValues(t *testing.T) {
	cases := []struct {
		name                          string
		role, useCase, referralSource string
	}{
		{"unknown role", "ceo<script>", "", ""},
		{"unknown use case", "", "world-domination", ""},
		{"unknown referral source", "", "", "carrier-pigeon"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := BuildSignupProfile(tc.role, tc.useCase, tc.referralSource); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}
