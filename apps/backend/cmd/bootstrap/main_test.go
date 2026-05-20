package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// TestApplyDefaultBootstrapValues_FillsEmptyFields confirms that --default mode
// fills every empty canonical field and that operator-supplied flag values are
// preserved.
func TestApplyDefaultBootstrapValues_FillsEmptyFields(t *testing.T) {
	t.Parallel()

	cfg := &BootstrapConfig{
		AdminName: "System Administrator", // flag default
		OrgDomain: "localhost",            // flag default
		MaxUsers:  100,                    // flag default
		MaxAgents: 1000,                   // flag default
	}

	if err := applyDefaultBootstrapValues(cfg); err != nil {
		t.Fatalf("applyDefaultBootstrapValues: %v", err)
	}

	if cfg.AdminEmail != defaultAdminEmail {
		t.Errorf("AdminEmail = %q, want %q", cfg.AdminEmail, defaultAdminEmail)
	}
	if cfg.OrgName != defaultOrgName {
		t.Errorf("OrgName = %q, want %q", cfg.OrgName, defaultOrgName)
	}
	if cfg.OrgDomain != defaultOrgDomain {
		t.Errorf("OrgDomain = %q, want %q", cfg.OrgDomain, defaultOrgDomain)
	}
	if cfg.MaxUsers != defaultMaxUsers {
		t.Errorf("MaxUsers = %d, want %d", cfg.MaxUsers, defaultMaxUsers)
	}
	if cfg.MaxAgents != defaultMaxAgents {
		t.Errorf("MaxAgents = %d, want %d", cfg.MaxAgents, defaultMaxAgents)
	}
	if !cfg.SkipPrompts {
		t.Error("SkipPrompts = false, want true (--default implies --yes)")
	}
	if cfg.AdminPassword == "" {
		t.Error("AdminPassword is empty after applyDefaultBootstrapValues; should be auto-generated")
	}
	if !cfg.passwordWasGenerated {
		t.Error("passwordWasGenerated = false; expected true when password was auto-filled")
	}
}

// TestApplyDefaultBootstrapValues_PreservesOperatorOverrides confirms that an
// operator running `aim-bootstrap --default --org-name=X --admin-password=Y`
// keeps the explicit X and Y instead of overwriting them with canonicals.
func TestApplyDefaultBootstrapValues_PreservesOperatorOverrides(t *testing.T) {
	t.Parallel()

	cfg := &BootstrapConfig{
		AdminEmail:    "custom@example.com",
		AdminPassword: "OperatorChosenP@ssword1",
		OrgName:       "CustomOrg",
		OrgDomain:     "custom.example.com",
		MaxUsers:      500,
		MaxAgents:     5000,
	}

	if err := applyDefaultBootstrapValues(cfg); err != nil {
		t.Fatalf("applyDefaultBootstrapValues: %v", err)
	}

	if cfg.AdminEmail != "custom@example.com" {
		t.Errorf("AdminEmail clobbered: got %q", cfg.AdminEmail)
	}
	if cfg.AdminPassword != "OperatorChosenP@ssword1" {
		t.Errorf("AdminPassword clobbered: got %q", cfg.AdminPassword)
	}
	if cfg.passwordWasGenerated {
		t.Error("passwordWasGenerated = true; expected false when password was operator-supplied")
	}
	if cfg.OrgName != "CustomOrg" {
		t.Errorf("OrgName clobbered: got %q", cfg.OrgName)
	}
	if cfg.OrgDomain != "custom.example.com" {
		t.Errorf("OrgDomain clobbered: got %q", cfg.OrgDomain)
	}
	if cfg.MaxUsers != 500 {
		t.Errorf("MaxUsers clobbered: got %d", cfg.MaxUsers)
	}
	if cfg.MaxAgents != 5000 {
		t.Errorf("MaxAgents clobbered: got %d", cfg.MaxAgents)
	}
}

// TestGenerateRandomPassword_PassesValidator runs the random-password generator
// many times and asserts every output passes PasswordHasher.ValidatePassword.
// The 4-char-class guarantee in generateRandomPassword should make this
// deterministic, not probabilistic — a single failure is a real defect.
func TestGenerateRandomPassword_PassesValidator(t *testing.T) {
	t.Parallel()
	hasher := auth.NewPasswordHasher()

	const iterations = 1000
	for i := 0; i < iterations; i++ {
		pw, err := generateRandomPassword()
		if err != nil {
			t.Fatalf("iteration %d: generateRandomPassword: %v", i, err)
		}
		if err := hasher.ValidatePassword(pw); err != nil {
			t.Fatalf("iteration %d: validator rejected %q: %v", i, pw, err)
		}
	}
}

// TestGenerateRandomPassword_Uniqueness asserts that the generator does not
// return the same password twice in a tight loop. A handful of collisions
// across 1000 iterations would indicate the rand source is unseeded or the
// length is too small.
func TestGenerateRandomPassword_Uniqueness(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		pw, err := generateRandomPassword()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if _, dup := seen[pw]; dup {
			t.Fatalf("collision at iteration %d: password %q already seen", i, pw)
		}
		seen[pw] = struct{}{}
	}
}

// TestGenerateRandomPassword_LengthAndCharset asserts the contract documented
// in the generateRandomPassword comment: exactly 32 chars, drawn from the
// look-alike-free union charset, with at least one char from each class.
func TestGenerateRandomPassword_LengthAndCharset(t *testing.T) {
	t.Parallel()
	pw, err := generateRandomPassword()
	if err != nil {
		t.Fatalf("generateRandomPassword: %v", err)
	}
	if got, want := len(pw), 32; got != want {
		t.Errorf("len = %d, want %d", got, want)
	}

	// No look-alike chars permitted.
	for _, ch := range "IOl01" {
		if strings.ContainsRune(pw, ch) {
			t.Errorf("password %q contains look-alike char %q", pw, ch)
		}
	}

	// Each class present at least once.
	classes := map[string]*regexp.Regexp{
		"upper":   regexp.MustCompile(`[A-Z]`),
		"lower":   regexp.MustCompile(`[a-z]`),
		"digit":   regexp.MustCompile(`[0-9]`),
		"special": regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>/?]`),
	}
	for name, re := range classes {
		if !re.MatchString(pw) {
			t.Errorf("password %q missing %s class", pw, name)
		}
	}
}
