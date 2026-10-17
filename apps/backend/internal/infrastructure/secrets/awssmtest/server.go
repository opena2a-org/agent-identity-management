// Package awssmtest is an in-memory stand-in for the AWS Secrets Manager HTTP
// API, for tests. It answers CreateSecret, GetSecretValue, PutSecretValue and
// DeleteSecret in the AWS JSON 1.1 protocol as the Secrets Manager API
// reference describes them, and keeps secrets in memory by name.
//
// A request is POST / with X-Amz-Target: secretsmanager.<Operation>,
// Content-Type: application/x-amz-json-1.1 and an Authorization header that
// starts "AWS4-HMAC-SHA256 Credential=". A request without one of these is
// refused with 400 and the type StubRequestRefused, which is this stub's own
// name and not an AWS error. The signature itself is not checked. Service
// errors answer {"__type": "<Name>", "message": "..."}.
package awssmtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

const (
	contentType  = "application/x-amz-json-1.1"
	targetPrefix = "secretsmanager."
	authPrefix   = "AWS4-HMAC-SHA256 Credential="

	// Region and account the stub puts in the ARNs it returns.
	Region    = "us-east-1"
	accountID = "000000000000"

	// RefusedType is the __type of a request the stub refuses before it
	// reaches an operation.
	RefusedType = "StubRequestRefused"

	maxSecretIDLength = 2048
	maxRequestBytes   = 1 << 20
	defaultRecovery   = 30
)

// Request is one request the stub accepted, as it arrived.
type Request struct {
	Operation string
	Body      []byte
}

type secretVersion struct {
	id     string
	binary []byte
	str    *string
	stages []string
}

type secret struct {
	arn       string
	name      string
	created   time.Time
	versions  []*secretVersion
	deletedAt *time.Time
}

func (s *secret) current() *secretVersion {
	for _, v := range s.versions {
		if slices.Contains(v.stages, "AWSCURRENT") {
			return v
		}
	}
	return nil
}

// Server is the stub. Its URL is the BaseEndpoint a Secrets Manager client
// points at.
type Server struct {
	*httptest.Server

	mu       sync.Mutex
	secrets  map[string]*secret
	requests []Request
}

// NewServer starts a stub on a loopback port and closes it when t ends.
func NewServer(t testing.TB) *Server {
	t.Helper()
	s := &Server{secrets: make(map[string]*secret)}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

// NewClient returns a Secrets Manager client of the AWS SDK pointed at s, with
// the stub's region and static credentials generated for this call. It reads
// no AWS configuration from the environment or from files.
func NewClient(s *Server) *secretsmanager.Client {
	accessKeyID := strings.Join([]string{"awssmtest", "access", randomHex(8)}, "-")
	secretAccessKey := strings.Join([]string{"awssmtest", "secret", randomHex(16)}, "-")
	return secretsmanager.New(secretsmanager.Options{
		Region:       Region,
		BaseEndpoint: aws.String(s.URL),
		HTTPClient:   s.Client(),
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     accessKeyID,
				SecretAccessKey: secretAccessKey,
				Source:          "awssmtest",
			}, nil
		}),
	})
}

// Requests returns the requests the stub accepted, in order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// SecretBinary returns the binary value of the current version of the secret
// called name, as the stub holds it after base64 decoding.
func (s *Server) SecretBinary(name string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sec, ok := s.secrets[name]
	if !ok {
		return nil, false
	}
	v := sec.current()
	if v == nil || v.binary == nil {
		return nil, false
	}
	return append([]byte(nil), v.binary...), true
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/" {
		writeError(w, http.StatusBadRequest, RefusedType, "the Secrets Manager API takes POST /")
		return
	}
	target := r.Header.Get("X-Amz-Target")
	if target == "" {
		writeError(w, http.StatusBadRequest, RefusedType, "X-Amz-Target is missing")
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != contentType {
		writeError(w, http.StatusBadRequest, RefusedType, fmt.Sprintf("Content-Type is %q, want %q", ct, contentType))
		return
	}
	if !strings.HasPrefix(r.Header.Get("Authorization"), authPrefix) {
		writeError(w, http.StatusBadRequest, RefusedType, "Authorization does not start "+authPrefix)
		return
	}
	op, ok := strings.CutPrefix(target, targetPrefix)
	if !ok {
		writeError(w, http.StatusBadRequest, RefusedType, fmt.Sprintf("X-Amz-Target %q is not a secretsmanager operation", target))
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, RefusedType, "request body could not be read: "+err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	switch op {
	case "CreateSecret":
		s.createSecret(w, body)
	case "GetSecretValue":
		s.getSecretValue(w, body)
	case "PutSecretValue":
		s.putSecretValue(w, body)
	case "DeleteSecret":
		s.deleteSecret(w, body)
	default:
		writeError(w, http.StatusBadRequest, RefusedType, fmt.Sprintf("operation %q is not implemented by this stub", op))
		return
	}
	s.requests = append(s.requests, Request{Operation: op, Body: body})
}

// lookup finds a secret by name or ARN. It writes the error answer and returns
// nil when the id is not valid or names no secret.
func (s *Server) lookup(w http.ResponseWriter, secretID string) *secret {
	if len(secretID) < 1 || len(secretID) > maxSecretIDLength {
		writeError(w, http.StatusBadRequest, "InvalidParameterException",
			fmt.Sprintf("SecretId must be 1 to %d characters", maxSecretIDLength))
		return nil
	}
	if sec, ok := s.secrets[secretID]; ok {
		return sec
	}
	for _, sec := range s.secrets {
		if sec.arn == secretID {
			return sec
		}
	}
	writeError(w, http.StatusBadRequest, "ResourceNotFoundException", "Secrets Manager can't find the specified secret.")
	return nil
}

type createSecretInput struct {
	Name               string  `json:"Name"`
	ClientRequestToken string  `json:"ClientRequestToken"`
	SecretBinary       []byte  `json:"SecretBinary"`
	SecretString       *string `json:"SecretString"`
}

func (s *Server) createSecret(w http.ResponseWriter, body []byte) {
	var in createSecretInput
	if !decode(w, body, &in) {
		return
	}
	if in.Name == "" {
		writeError(w, http.StatusBadRequest, "InvalidParameterException", "Name is required")
		return
	}
	if in.SecretBinary != nil && in.SecretString != nil {
		writeError(w, http.StatusBadRequest, "InvalidParameterException", "SecretString and SecretBinary cannot both be set")
		return
	}
	if _, exists := s.secrets[in.Name]; exists {
		writeError(w, http.StatusBadRequest, "ResourceExistsException", "The operation failed because the secret "+in.Name+" already exists.")
		return
	}
	sec := &secret{
		arn:     fmt.Sprintf("arn:aws:secretsmanager:%s:%s:secret:%s-%s", Region, accountID, in.Name, randomHex(3)),
		name:    in.Name,
		created: time.Now(),
	}
	out := map[string]interface{}{"ARN": sec.arn, "Name": sec.name}
	if in.SecretBinary != nil || in.SecretString != nil {
		v := &secretVersion{id: versionID(in.ClientRequestToken), binary: in.SecretBinary, str: in.SecretString, stages: []string{"AWSCURRENT"}}
		sec.versions = append(sec.versions, v)
		out["VersionId"] = v.id
	}
	s.secrets[in.Name] = sec
	writeJSON(w, out)
}

type getSecretValueInput struct {
	SecretID     string `json:"SecretId"`
	VersionID    string `json:"VersionId"`
	VersionStage string `json:"VersionStage"`
}

func (s *Server) getSecretValue(w http.ResponseWriter, body []byte) {
	var in getSecretValueInput
	if !decode(w, body, &in) {
		return
	}
	sec := s.lookup(w, in.SecretID)
	if sec == nil {
		return
	}
	if sec.deletedAt != nil {
		writeError(w, http.StatusBadRequest, "InvalidRequestException", "The secret is marked for deletion.")
		return
	}
	stage := in.VersionStage
	if stage == "" && in.VersionID == "" {
		stage = "AWSCURRENT"
	}
	var found *secretVersion
	for _, v := range sec.versions {
		if in.VersionID != "" && v.id != in.VersionID {
			continue
		}
		if stage != "" && !slices.Contains(v.stages, stage) {
			continue
		}
		found = v
	}
	if found == nil {
		writeError(w, http.StatusBadRequest, "ResourceNotFoundException", "Secrets Manager can't find the specified secret value.")
		return
	}
	out := map[string]interface{}{
		"ARN":           sec.arn,
		"Name":          sec.name,
		"CreatedDate":   epochSeconds(sec.created),
		"VersionId":     found.id,
		"VersionStages": found.stages,
	}
	if found.binary != nil {
		out["SecretBinary"] = found.binary // base64 in the JSON answer
	}
	if found.str != nil {
		out["SecretString"] = *found.str
	}
	writeJSON(w, out)
}

type putSecretValueInput struct {
	SecretID           string   `json:"SecretId"`
	ClientRequestToken string   `json:"ClientRequestToken"`
	SecretBinary       []byte   `json:"SecretBinary"`
	SecretString       *string  `json:"SecretString"`
	VersionStages      []string `json:"VersionStages"`
}

func (s *Server) putSecretValue(w http.ResponseWriter, body []byte) {
	var in putSecretValueInput
	if !decode(w, body, &in) {
		return
	}
	sec := s.lookup(w, in.SecretID)
	if sec == nil {
		return
	}
	if sec.deletedAt != nil {
		writeError(w, http.StatusBadRequest, "InvalidRequestException", "The secret is marked for deletion.")
		return
	}
	if (in.SecretBinary == nil) == (in.SecretString == nil) {
		writeError(w, http.StatusBadRequest, "InvalidParameterException", "exactly one of SecretString and SecretBinary must be set")
		return
	}
	stages := in.VersionStages
	if len(stages) == 0 {
		stages = []string{"AWSCURRENT"}
	}
	if slices.Contains(stages, "AWSCURRENT") {
		// AWSCURRENT moves to the new version and AWSPREVIOUS to the one
		// that held it.
		prev := sec.current()
		for _, v := range sec.versions {
			v.stages = removeStage(v.stages, "AWSPREVIOUS")
		}
		if prev != nil {
			prev.stages = append(removeStage(prev.stages, "AWSCURRENT"), "AWSPREVIOUS")
		}
	}
	v := &secretVersion{id: versionID(in.ClientRequestToken), binary: in.SecretBinary, str: in.SecretString, stages: stages}
	sec.versions = append(sec.versions, v)
	writeJSON(w, map[string]interface{}{
		"ARN":           sec.arn,
		"Name":          sec.name,
		"VersionId":     v.id,
		"VersionStages": v.stages,
	})
}

type deleteSecretInput struct {
	SecretID                   string `json:"SecretId"`
	ForceDeleteWithoutRecovery *bool  `json:"ForceDeleteWithoutRecovery"`
	RecoveryWindowInDays       *int64 `json:"RecoveryWindowInDays"`
}

func (s *Server) deleteSecret(w http.ResponseWriter, body []byte) {
	var in deleteSecretInput
	if !decode(w, body, &in) {
		return
	}
	force := in.ForceDeleteWithoutRecovery != nil && *in.ForceDeleteWithoutRecovery
	if force && in.RecoveryWindowInDays != nil {
		writeError(w, http.StatusBadRequest, "InvalidParameterException", "ForceDeleteWithoutRecovery and RecoveryWindowInDays cannot both be set")
		return
	}
	sec := s.lookup(w, in.SecretID)
	if sec == nil {
		return
	}
	deletionDate := time.Now()
	if force {
		delete(s.secrets, sec.name)
	} else {
		days := int64(defaultRecovery)
		if in.RecoveryWindowInDays != nil {
			days = *in.RecoveryWindowInDays
		}
		deletionDate = deletionDate.Add(time.Duration(days) * 24 * time.Hour)
		sec.deletedAt = &deletionDate
	}
	writeJSON(w, map[string]interface{}{
		"ARN":          sec.arn,
		"Name":         sec.name,
		"DeletionDate": epochSeconds(deletionDate),
	})
}

func decode(w http.ResponseWriter, body []byte, v interface{}) bool {
	if err := json.Unmarshal(body, v); err != nil {
		writeError(w, http.StatusBadRequest, RefusedType, "request body is not the operation's JSON: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"__type": errType, "message": message})
}

// epochSeconds is the JSON 1.1 form of a timestamp.
func epochSeconds(t time.Time) float64 {
	return float64(t.UnixMilli()) / 1000
}

func versionID(token string) string {
	if token != "" {
		return token
	}
	b := randomHex(16)
	return b[0:8] + "-" + b[8:12] + "-" + b[12:16] + "-" + b[16:20] + "-" + b[20:32]
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func removeStage(stages []string, stage string) []string {
	out := make([]string, 0, len(stages))
	for _, s := range stages {
		if s != stage {
			out = append(out, s)
		}
	}
	return out
}
