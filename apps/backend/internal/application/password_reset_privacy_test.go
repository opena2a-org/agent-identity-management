package application

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A password reset link carries the reset token and nothing else, and a
// failure to send the reset mail is logged without the recipient's address.
// An email address in a URL query lands in browser history, proxy logs,
// referrer headers and analytics; a mail provider's error often echoes the
// recipient, so the failure log is redacted before it is written.

const resetProbeAddress = "reset.probe@example.test"

// privacyUserRepo answers GetByEmail with one fixed user and accepts Update.
type privacyUserRepo struct {
	domain.UserRepository
	user *domain.User
}

func (r *privacyUserRepo) GetByEmail(email string) (*domain.User, error) {
	if r.user != nil && email == r.user.Email {
		return r.user, nil
	}
	return nil, fmt.Errorf("not found")
}

func (r *privacyUserRepo) Update(user *domain.User) error { return nil }

// privacyEmailService records every templated mail and can fail the send.
type privacyEmailService struct {
	domain.EmailService
	sent    []domain.EmailTemplateData
	sendErr error
}

func (e *privacyEmailService) SendTemplatedEmail(_ domain.EmailTemplate, _ string, data interface{}) error {
	if d, ok := data.(domain.EmailTemplateData); ok {
		e.sent = append(e.sent, d)
	}
	return e.sendErr
}

func privacyResetService(t *testing.T, mail *privacyEmailService) (*RegistrationService, *domain.User) {
	t.Helper()
	user := &domain.User{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Email:          resetProbeAddress,
		Name:           "Reset Probe",
		Status:         domain.UserStatusActive,
	}
	return NewRegistrationService(nil, &privacyUserRepo{user: user}, nil, nil, mail), user
}

// C1: no backend source builds a URL query that carries an email address.
func TestNoBackendSourcePutsAnEmailAddressInAURLQuery(t *testing.T) {
	root := repoRoot(t)
	inQuery := regexp.MustCompile(`(?i)[?&]email=`)
	readsQuery := regexp.MustCompile(`(?i)\.Query\(\s*"email"`)
	scanned := 0
	var hits []string
	for _, sub := range []string{"cmd", "internal"} {
		err := filepath.Walk(filepath.Join(root, "apps", "backend", sub), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			scanned++
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(body), "\n") {
				if inQuery.MatchString(line) || readsQuery.MatchString(line) {
					rel, _ := filepath.Rel(root, path)
					hits = append(hits, fmt.Sprintf("%s:%d", rel, i+1))
				}
			}
			return nil
		})
		require.NoError(t, err)
	}
	require.GreaterOrEqual(t, scanned, 50, "the walk must cover the backend sources")
	assert.Empty(t, hits, "backend sources that put an email address in a URL query")
}

// C2: the reset link's query is exactly the token; the address is not in the link.
func TestPasswordResetLinkCarriesOnlyTheToken(t *testing.T) {
	mail := &privacyEmailService{}
	svc, _ := privacyResetService(t, mail)
	require.NoError(t, svc.RequestPasswordReset(context.Background(), resetProbeAddress))
	require.Len(t, mail.sent, 1, "exactly one mail")

	link, ok := mail.sent[0].CustomData["ResetLink"].(string)
	require.True(t, ok, "CustomData.ResetLink is the link the template renders")
	parsed, err := url.Parse(link)
	require.NoError(t, err)
	keys := make([]string, 0, len(parsed.Query()))
	for k := range parsed.Query() {
		keys = append(keys, k)
	}
	assert.Equal(t, []string{"token"}, keys, "the query carries the token and nothing else")
	assert.NotEmpty(t, parsed.Query().Get("token"))
	assert.NotContains(t, link, resetProbeAddress)
	assert.NotContains(t, link, url.QueryEscape(resetProbeAddress))
	assert.NotContains(t, strings.ToLower(link), "email")
}

// C3: when the reset mail cannot be sent, the log names the account id and
// the provider's failure, never the recipient's address.
func TestPasswordResetSendFailureLogCarriesNoAddress(t *testing.T) {
	mail := &privacyEmailService{sendErr: fmt.Errorf("RCPT TO failed: 550 5.1.1 <%s>: recipient rejected", resetProbeAddress)}
	svc, user := privacyResetService(t, mail)

	out := captureAllOutput(t, func() {
		require.NoError(t, svc.RequestPasswordReset(context.Background(), resetProbeAddress))
	})
	assert.NotContains(t, out, resetProbeAddress, "the recipient's address is not logged")
	assert.Contains(t, out, user.ID.String(), "control: the account id is logged")
	assert.Contains(t, out, "RCPT TO failed", "control: the provider's failure is logged")
}

// captureAllOutput routes os.Stdout, os.Stderr and the standard logger through
// one pipe for the duration of fn and returns what was written.
func captureAllOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldOut, oldErr, oldLog := os.Stdout, os.Stderr, log.Writer()
	os.Stdout, os.Stderr = w, w
	log.SetOutput(w)
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	func() {
		defer func() {
			os.Stdout, os.Stderr = oldOut, oldErr
			log.SetOutput(oldLog)
			_ = w.Close()
		}()
		fn()
	}()
	return <-done
}
