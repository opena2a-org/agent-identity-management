package handlers

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// docs/DEPLOYMENT.md tells an operator which refused password sign-ins the
// SIEM receives as aim.auth.signin_failed, the reason each carries and the
// answer the route gave, and which of them AIM also stores. The forwarder
// sends the reason a route reports unchanged. The test below sends the same
// sign-ins to both password routes and holds the guide's three tables to what
// the routes answered, reported and stored.

// signInFailureStore is the auth failure store of one sign-in: it counts the
// failed attempts the auth service records and, when locked, holds an active
// lockout for every email. A call the sign-in path does not make panics.
type signInFailureStore struct {
	domain.AuthFailureRepository
	locked   bool
	recorded int
}

func (s *signInFailureStore) GetActiveLockout(email string) (*domain.AuthLockout, error) {
	if !s.locked {
		return nil, nil
	}
	return &domain.AuthLockout{Email: email, LockedUntil: time.Now().Add(10 * time.Minute)}, nil
}

func (s *signInFailureStore) RecordFailure(*domain.AuthFailure) error {
	s.recorded++
	return nil
}

func (s *signInFailureStore) CountRecentFailures(string, int) (int, error) { return s.recorded, nil }

// signInAttempt is a password sign-in sent to each route: the user the user
// store holds, the registration requests (none when nil), whether the email
// is locked out, and the requirement of provider sign-in (none when nil).
type signInAttempt struct {
	name            string
	user            *domain.User
	registrations   application.RegistrationRepository
	locked          bool
	requirement     application.ProviderSignInRequirement
	email, password string
}

// signInOutcome is what a route did with an attempt.
type signInOutcome struct {
	status   int
	reports  []SignInFailure
	recorded int  // the failed attempts the auth service recorded
	audited  int  // the audit log rows written
	signedIn bool // the user's last sign-in was written
}

// send sends the attempt to route, on stores and routes of its own.
func (a signInAttempt) send(t *testing.T, route string) signInOutcome {
	t.Helper()
	users := &issuanceUserRepo{}
	if a.user != nil {
		held := *a.user
		users.user = &held
	}
	registrations := a.registrations
	if registrations == nil {
		registrations = refusalRegistrationRepo{}
	}
	failures := &signInFailureStore{locked: a.locked}
	authService := application.NewAuthService(users, nil, nil, failures, nil, nil)
	authService.SetProviderSignInRequirement(a.requirement)
	auditRows := &familyAuditRepo{}
	audit := application.NewAuditService(auditRows)
	jwtService := auth.NewJWTService()
	reports := &signInReports{}
	public := NewPublicRegistrationHandler(application.NewRegistrationService(registrations, users, nil, nil, nil),
		authService, jwtService, audit)
	public.SetSignInFailureObserver(reports.observe)
	local := NewAuthHandler(authService, jwtService, nil, audit, nil)
	local.SetSignInFailureObserver(reports.observe)
	app := fiber.New()
	app.Post("/api/v1/public/login", public.Login)
	app.Post("/api/v1/auth/login/local", local.LocalLogin)

	status, _ := postSignIn(t, app, route, signInBody(a.email, a.password))
	return signInOutcome{
		status:   status,
		reports:  reports.got,
		recorded: failures.recorded,
		audited:  len(auditRows.rows),
		signedIn: users.user != nil && users.user.LastLoginAt != nil,
	}
}

// refusedSignInRow is a row of the guide's reason table: the route, its
// answer and the reason the event carries.
type refusedSignInRow struct {
	route  string
	status int
	reason string
}

// refusedSignInRecord is what a refused sign-in left outside the SIEM.
type refusedSignInRecord struct {
	failureRecorded bool // the auth service recorded a failed attempt
	otherwiseStored bool // an audit log row or the user's last sign-in was written
}

// siemForwardingSection returns docs/DEPLOYMENT.md from its "### SIEM
// Forwarding" heading to the next heading.
func siemForwardingSection(t *testing.T) string {
	t.Helper()
	_, section, found := strings.Cut(readBackendFile(t, "../../docs/DEPLOYMENT.md"), "\n### SIEM Forwarding\n")
	require.True(t, found, "docs/DEPLOYMENT.md has no SIEM Forwarding section")
	if next := regexp.MustCompile(`(?m)^#{2,4} `).FindStringIndex(section); next != nil {
		section = section[:next[0]]
	}
	return section
}

// markdownTableRows returns the cells of each row of the table in text whose
// header row is header, without the header and separator rows.
func markdownTableRows(t *testing.T, text, header string) [][]string {
	t.Helper()
	_, table, found := strings.Cut(text, "\n"+header+"\n")
	require.True(t, found, "no table with the header %s", header)
	var rows [][]string
	for i, line := range strings.Split(table, "\n") {
		if !strings.HasPrefix(line, "|") {
			break
		}
		if i == 0 { // the separator row
			continue
		}
		var cells []string
		for _, cell := range strings.Split(strings.Trim(line, "|"), "|") {
			cells = append(cells, strings.TrimSpace(cell))
		}
		rows = append(rows, cells)
	}
	return rows
}

// documentedReasons returns the reasons text names in code spans, such as
// `password_mismatch`.
func documentedReasons(text string) []string {
	var reasons []string
	for _, m := range regexp.MustCompile("`([a-z_]+)`").FindAllStringSubmatch(text, -1) {
		reasons = append(reasons, m[1])
	}
	return reasons
}

func TestDeploymentGuideDescribesTheRefusedSignInsTheSIEMReceives(t *testing.T) {
	requirePublicRepositoryFiles(t)
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")

	member := withPassword(t, domain.User{ID: uuid.New(), OrganizationID: uuid.New(), Email: "member@example.com",
		Name: "Member", Role: domain.RoleMember, Status: domain.UserStatusActive})
	deactivated := *member
	deactivated.Status = domain.UserStatusDeactivated
	noPassword := *member
	noPassword.PasswordHash = nil
	rejectedHash, err := bcrypt.GenerateFromPassword([]byte(issuancePassword), bcrypt.MinCost)
	require.NoError(t, err)
	rejectedHashText := string(rejectedHash)
	rejected := rejectedRegistrationRepo{request: &domain.UserRegistrationRequest{
		ID: uuid.New(), Email: "rejected@example.com", Status: domain.RegistrationStatusRejected, PasswordHash: &rejectedHashText,
	}}

	attempts := []signInAttempt{
		{name: "a wrong password", user: member, email: member.Email, password: "Wrong-Passw0rd!"},
		{name: "an unknown email", user: member, email: "unknown@example.com", password: issuancePassword},
		{name: "a deactivated user", user: &deactivated, email: member.Email, password: issuancePassword},
		{name: "a user without a password", user: &noPassword, email: member.Email, password: issuancePassword},
		{name: "a rejected registration request", user: member, registrations: rejected,
			email: "rejected@example.com", password: issuancePassword},
		{name: "a locked out email", user: member, locked: true, email: member.Email, password: issuancePassword},
		{name: "a member of an organization that requires provider sign-in", user: member,
			requirement: requirementOf{member.OrganizationID: true}, email: member.Email, password: issuancePassword},
		{name: "a member whose organization's requirement cannot be read", user: member,
			requirement: unreadableRequirement{}, email: member.Email, password: issuancePassword},
	}

	// Each refused sign-in the routes reported, by route, answer and reason,
	// with what it left outside the SIEM.
	refused := map[refusedSignInRow]refusedSignInRecord{}
	for _, attempt := range attempts {
		for _, route := range passwordSignInRoutes {
			got := attempt.send(t, route)
			if got.status == fiber.StatusOK {
				assert.Empty(t, got.reports, "%s at %s: a sign-in is not reported", attempt.name, route)
				continue
			}
			require.Len(t, got.reports, 1, "%s at %s: answered %d", attempt.name, route, got.status)
			row := refusedSignInRow{route: got.reports[0].Route, status: got.status, reason: got.reports[0].Reason}
			record := refusedSignInRecord{failureRecorded: got.recorded > 0, otherwiseStored: got.audited > 0 || got.signedIn}
			if earlier, seen := refused[row]; seen {
				assert.Equal(t, earlier, record, "%s at %s: a refusal with the same reason is stored alike", attempt.name, route)
			}
			refused[row] = record
		}
	}
	var answers []int
	var reported []refusedSignInRow
	for row := range refused {
		reported = append(reported, row)
		if !slices.Contains(answers, row.status) {
			answers = append(answers, row.status)
		}
	}

	section := siemForwardingSection(t)

	// The event table names each answer a route reported a refused sign-in with.
	var event []string
	for _, cells := range markdownTableRows(t, section, "| Event | `eventType` | `severity` |") {
		if len(cells) == 3 && cells[1] == "`aim.auth.signin_failed`" {
			event = cells
		}
	}
	require.NotNil(t, event, "the event table has no aim.auth.signin_failed row")
	var named []int
	for _, code := range regexp.MustCompile(`\b\d{3}\b`).FindAllString(event[0], -1) {
		status, err := strconv.Atoi(code)
		require.NoError(t, err)
		if !slices.Contains(named, status) {
			named = append(named, status)
		}
	}
	assert.ElementsMatch(t, answers, named, "the answers the aim.auth.signin_failed row names: %s", event[0])

	// The reason table has a row for each route, answer and reason reported,
	// and no other.
	var rows []refusedSignInRow
	for _, cells := range markdownTableRows(t, section, "| Route | Answer | `reason` | Check |") {
		require.Len(t, cells, 4, "a reason table row has a route, an answer, a reason and a check: %v", cells)
		status, err := strconv.Atoi(cells[1])
		require.NoError(t, err, "the answer of %v", cells)
		assert.NotEmpty(t, cells[3], "the check of %v", cells)
		rows = append(rows, refusedSignInRow{
			route:  strings.TrimPrefix(strings.Trim(cells[0], "`"), "POST "),
			status: status,
			reason: strings.Trim(cells[2], "`"),
		})
	}
	assert.ElementsMatch(t, reported, rows, "the reason table's rows, against the refused sign-ins the routes reported")

	// The table of records outside the SIEM names each reason of POST
	// /api/v1/auth/login/local once: among those that write an auth_failures
	// row, or among those AIM does not store, with every refused POST
	// /api/v1/public/login.
	var loss []string
	for _, cells := range markdownTableRows(t, section, "| Event | Record outside the SIEM |") {
		if len(cells) == 2 && cells[0] == "Failed sign-in (`aim.auth.signin_failed`)" {
			loss = cells
		}
	}
	require.NotNil(t, loss, "the table of records outside the SIEM has no failed sign-in row")
	recordText := loss[1]
	writes := strings.Index(recordText, "also write an `auth_failures` row")
	require.NotEqual(t, -1, writes, "the failed sign-in row names the reasons that write an auth_failures row")
	written := documentedReasons(recordText[:writes])
	notStored := strings.Index(recordText, "are not stored by AIM outside the SIEM stream")
	require.NotEqual(t, -1, notStored, "the failed sign-in row names the reasons AIM does not store")
	unstoredSentence := recordText[strings.LastIndex(recordText[:notStored], ". ")+2 : notStored]
	unstored := documentedReasons(unstoredSentence)
	textHas(t, unstoredSentence, "every refused `POST /api/v1/public/login`", "the reasons AIM does not store")

	var localReasons []string
	for row, record := range refused {
		switch {
		case row.route == "/api/v1/public/login":
			assert.Equal(t, refusedSignInRecord{}, record, "%v is not stored", row)
		case record.failureRecorded:
			localReasons = append(localReasons, row.reason)
			assert.Contains(t, written, row.reason, "%v writes an auth_failures row", row)
		default:
			localReasons = append(localReasons, row.reason)
			assert.Contains(t, unstored, row.reason, "%v is not stored", row)
			assert.False(t, record.otherwiseStored, "%v is not stored", row)
		}
	}
	assert.ElementsMatch(t, localReasons, append(append([]string{}, written...), unstored...),
		"the failed sign-in row names each reason of POST /api/v1/auth/login/local once")
}
