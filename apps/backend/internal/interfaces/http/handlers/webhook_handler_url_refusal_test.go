package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The webhook routes answer a service error without passing the service's
// reason to the caller. A webhook URL the server refuses is answered with 400
// and a fixed reason that names no host or address; a failed host lookup that
// says nothing about the URL is answered as a server fault; a test delivery
// that failed is reported without the delivery error. The reason stays in the
// server log.

// webhookURLCheckAddress is the private address the refused URL and the failed
// delivery below name. It must not reach a response.
const webhookURLCheckAddress = "10.0.0.5"

// webhookURLCheckServerFault is the body of every 5xx answer (respondServerError).
const webhookURLCheckServerFault = `{"error":"An internal error occurred. Please try again later."}`

var webhookURLCheckOrgID = uuid.MustParse("44444444-4444-4444-4444-444444444444")

// webhookURLCheckAudit accepts audit entries. The routes below call no other
// audit method.
type webhookURLCheckAudit struct{ AuditServicer }

func (webhookURLCheckAudit) LogAction(context.Context, uuid.UUID, uuid.UUID, domain.AuditAction, string, uuid.UUID, string, string, map[string]interface{}) error {
	return nil
}

// webhookURLCheckService is the real webhook service with one stored webhook
// in the caller's organization. Create and update run the service's own URL
// check; test returns testResult.
type webhookURLCheckService struct {
	*application.WebhookService
	stored     *domain.Webhook
	testResult *application.WebhookTestResult
}

func (s *webhookURLCheckService) GetWebhook(context.Context, uuid.UUID) (*domain.Webhook, error) {
	return s.stored, nil
}

func (s *webhookURLCheckService) TestWebhook(context.Context, uuid.UUID) (*application.WebhookTestResult, error) {
	return s.testResult, nil
}

func newWebhookURLCheckService() *webhookURLCheckService {
	return &webhookURLCheckService{
		WebhookService: application.NewWebhookService(nil),
		stored: &domain.Webhook{
			ID:             uuid.New(),
			OrganizationID: webhookURLCheckOrgID,
			Name:           "url-check-hook",
			URL:            "https://hook.example/receive",
		},
	}
}

type webhookURLCheckAnswer struct {
	status int
	body   string
	log    string
}

func callWebhookURLCheckRoute(t *testing.T, svc *webhookURLCheckService, method, path, body string) webhookURLCheckAnswer {
	t.Helper()
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	handler := NewWebhookHandlerWithInterfaces(svc, webhookURLCheckAudit{})
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", webhookURLCheckOrgID)
		c.Locals("user_id", uuid.New())
		return c.Next()
	})
	app.Post("/webhooks", handler.CreateWebhook)
	app.Put("/webhooks/:id", handler.UpdateWebhook)
	app.Post("/webhooks/:id/test", handler.TestWebhook)

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	answer, _ := io.ReadAll(resp.Body)
	return webhookURLCheckAnswer{status: resp.StatusCode, body: string(answer), log: logged.String()}
}

// A webhook created or updated with a URL the server refuses is answered with
// 400 and a reason that does not say what the URL resolved to. The test pins
// that a log line was written, not how the service's reason reads.
func TestWebhookHandler_RefusedURLIsAnsweredWithAFixedReason(t *testing.T) {
	body := `{"name":"url-check-hook","url":"http://` + webhookURLCheckAddress + `/receive","events":["agent.created"]}`
	for _, route := range []struct{ name, method, path string }{
		{"create", http.MethodPost, "/webhooks"},
		{"update", http.MethodPut, "/webhooks/"},
	} {
		t.Run(route.name, func(t *testing.T) {
			svc := newWebhookURLCheckService()
			path := route.path
			if route.method == http.MethodPut {
				path += svc.stored.ID.String()
			}
			got := callWebhookURLCheckRoute(t, svc, route.method, path, body)

			if got.status != fiber.StatusBadRequest {
				t.Errorf("status = %d, body = %s; want %d for a URL the server refuses", got.status, got.body, fiber.StatusBadRequest)
			}
			if want := `{"error":"url must be a public http or https URL"}`; got.body != want {
				t.Errorf("body = %s, want %s", got.body, want)
			}
			if strings.TrimSpace(got.log) == "" {
				t.Error("the server log has no line for the refused URL")
			}
		})
	}
}

// A test delivery that could not connect, or that the endpoint answered with
// an error status, is reported as failed without the delivery error (here one
// that names an address).
func TestWebhookHandler_FailedTestDeliveryIsReportedWithoutItsError(t *testing.T) {
	const deliveryError = `Post "https://hook.example/receive": dial tcp ` + webhookURLCheckAddress + `:443: destination address is not allowed (private)`
	for _, tc := range []struct {
		name        string
		result      *application.WebhookTestResult
		wantMessage string
	}{
		{
			name:        "connection refused",
			result:      &application.WebhookTestResult{Success: false, StatusCode: 0, ErrorMessage: deliveryError},
			wantMessage: "Webhook test failed: the endpoint could not be reached",
		},
		{
			name:        "error status",
			result:      &application.WebhookTestResult{Success: false, StatusCode: 503, ErrorMessage: "webhook delivery failed with status 503"},
			wantMessage: "Webhook test failed: the endpoint responded with status 503",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newWebhookURLCheckService()
			svc.testResult = tc.result
			got := callWebhookURLCheckRoute(t, svc, http.MethodPost, "/webhooks/"+svc.stored.ID.String()+"/test", "")

			if got.status != fiber.StatusOK {
				t.Fatalf("status = %d, body = %s; want %d", got.status, got.body, fiber.StatusOK)
			}
			var answer struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal([]byte(got.body), &answer); err != nil {
				t.Fatalf("decode %s: %v", got.body, err)
			}
			if answer.Success || answer.Message != tc.wantMessage {
				t.Errorf("success = %v, message = %q; want false and %q", answer.Success, answer.Message, tc.wantMessage)
			}
			if strings.Contains(got.body, webhookURLCheckAddress) || strings.Contains(got.body, "not allowed") {
				t.Errorf("body = %s; it carries the delivery error", got.body)
			}
			if !strings.Contains(got.log, tc.result.ErrorMessage) {
				t.Errorf("the server log does not carry the delivery error:\n%s", got.log)
			}
		})
	}
}

// Webhook create and update check the URL's host with ValidateExternalURL
// before anything is stored. The test below answers the server's DNS queries
// itself, through a resolver that replaces net.DefaultResolver until the test
// ends, so the host lookup can fail in a chosen way.

const (
	webhookURLCheckServFail = 2
	webhookURLCheckNXDomain = 3
)

// webhookURLCheckDNS answers every DNS query with rcode and no record, and
// counts the queries it answered.
type webhookURLCheckDNS struct {
	rcode   byte
	queries atomic.Int32
}

// useWebhookURLCheckDNS makes dns answer every lookup the server makes until
// the test ends.
func useWebhookURLCheckDNS(t *testing.T, dns *webhookURLCheckDNS) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("dns listener: %v", err)
	}
	go dns.serve(conn)

	previous := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "udp", conn.LocalAddr().String())
		},
	}
	t.Cleanup(func() {
		net.DefaultResolver = previous
		_ = conn.Close()
	})
}

func (d *webhookURLCheckDNS) serve(conn net.PacketConn) {
	buf := make([]byte, 1500)
	for {
		n, from, err := conn.ReadFrom(buf)
		if err != nil {
			return
		}
		if reply := d.reply(buf[:n]); reply != nil {
			d.queries.Add(1)
			_, _ = conn.WriteTo(reply, from)
		}
	}
}

// reply answers query, a DNS message with one question: a 12-byte header, then
// the name as length-prefixed labels ending in a zero byte, a type and a class.
// The answer repeats the question and carries no record.
func (d *webhookURLCheckDNS) reply(query []byte) []byte {
	if len(query) < 12 {
		return nil
	}
	end := 12
	for end < len(query) && query[end] != 0 {
		end += int(query[end]) + 1
	}
	end += 5 // the zero byte, the type and the class
	if end > len(query) {
		return nil
	}
	reply := []byte{
		query[0], query[1], // the query's ID
		0x80 | query[2]&0x01,   // a response, recursion desired as the query asked
		0x80 | d.rcode,         // recursion available, the response code
		0, 1, 0, 0, 0, 0, 0, 0, // one question, nothing else
	}
	return append(reply, query[12:end]...)
}

// A webhook host whose lookup fails for a reason other than the name not
// existing (here the resolver answers SERVFAIL, as a failing or overloaded one
// does) says nothing about the caller's URL: create and update answer it as a
// server fault. A host the resolver reports does not exist is a problem with
// the URL and is answered with 400 and the fixed reason.
func TestWebhookHandler_FailedHostLookupIsAnsweredAsAServerFault(t *testing.T) {
	const refused = `{"error":"url must be a public http or https URL"}`
	body := `{"name":"url-check-hook","url":"https://hook-lookup.aim.test/receive","events":["agent.created"]}`
	for _, tc := range []struct {
		name   string
		rcode  byte
		status int
		want   string
	}{
		{"resolver failure", webhookURLCheckServFail, fiber.StatusInternalServerError, webhookURLCheckServerFault},
		{"no such host", webhookURLCheckNXDomain, fiber.StatusBadRequest, refused},
	} {
		for _, route := range []struct{ name, method string }{
			{"create", http.MethodPost},
			{"update", http.MethodPut},
		} {
			t.Run(tc.name+"/"+route.name, func(t *testing.T) {
				dns := &webhookURLCheckDNS{rcode: tc.rcode}
				useWebhookURLCheckDNS(t, dns)
				svc := newWebhookURLCheckService()
				path := "/webhooks"
				if route.method == http.MethodPut {
					path = "/webhooks/" + svc.stored.ID.String()
				}
				got := callWebhookURLCheckRoute(t, svc, route.method, path, body)

				if got.status != tc.status {
					t.Errorf("status = %d, body = %s; want %d", got.status, got.body, tc.status)
				}
				if got.body != tc.want {
					t.Errorf("body = %s, want %s", got.body, tc.want)
				}
				// Control: the answer came from the lookup this test's resolver
				// failed, not from a check made before it.
				if n := dns.queries.Load(); n == 0 {
					t.Error("the webhook host was not looked up")
				}
				if strings.TrimSpace(got.log) == "" {
					t.Error("the server log has no line for the failed webhook " + route.name)
				}
			})
		}
	}
}
