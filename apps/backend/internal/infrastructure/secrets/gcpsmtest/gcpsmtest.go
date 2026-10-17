// Package gcpsmtest is an in-memory GCP Secret Manager gRPC service for tests
// that drive the cloud.google.com/go/secretmanager/apiv1 client library. It
// listens on 127.0.0.1 only and serves the generated SecretManagerServiceServer
// of google.cloud.secretmanager.v1.
//
// Request rules follow Google's published service definition,
// https://cloud.google.com/secret-manager/docs/reference/rpc/google.cloud.secretmanager.v1
// (now served at docs.cloud.google.com), for the RPCs the AIM backend calls:
//   - CreateSecret: parent is required, projects/* or projects/*/locations/*;
//     secret_id is required, unique within the project, up to 255 letters,
//     numerals, hyphens and underscores; secret is required. A secret holds no
//     versions when created. Replication is optional.
//   - AddSecretVersion: parent is required, projects/*/secrets/* (or the
//     locations form); payload is required and its data must be no larger
//     than 64KiB. The service generates a CRC32C when none is sent.
//   - AccessSecretVersion: name is required, projects/*/secrets/*/versions/*;
//     the version "latest" is an alias to the most recently created version.
//   - DeleteSecret: name is required, projects/*/secrets/*; it deletes the
//     secret and its versions. The optional etag is not modelled.
//
// A missing secret or version is answered with gRPC status NOT_FOUND, an
// existing secret_id with ALREADY_EXISTS and a malformed request with
// INVALID_ARGUMENT. A call without "authorization: Bearer <token>" metadata
// carrying the stub's token is answered UNAUTHENTICATED, the gRPC code for a
// request without valid authentication credentials. The status messages are
// the stub's own and are not verified against the service.
package gcpsmtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// maxPayloadBytes is the SecretPayload.data limit, 64KiB.
const maxPayloadBytes = 64 * 1024

var (
	parentPattern        = regexp.MustCompile(`^projects/[^/]+(/locations/[^/]+)?$`)
	secretNamePattern    = regexp.MustCompile(`^(projects/[^/]+(?:/locations/[^/]+)?)/secrets/([^/]+)$`)
	versionNamePattern   = regexp.MustCompile(`^(projects/[^/]+(?:/locations/[^/]+)?/secrets/[^/]+)/versions/([^/]+)$`)
	secretIDPattern      = regexp.MustCompile(`^[A-Za-z0-9_-]{1,255}$`)
	castagnoli           = crc32.MakeTable(crc32.Castagnoli)
	errMissingCredential = status.Error(codes.Unauthenticated, "Request is missing a valid authentication credential.")
)

var _ secretmanagerpb.SecretManagerServiceServer = (*Server)(nil)

// Request is one RPC the stub received, recorded after it was answered.
type Request struct {
	// Method is the full gRPC method, e.g. /google.cloud.secretmanager.v1.SecretManagerService/CreateSecret.
	Method string
	// Resource is the request's parent or name field.
	Resource string
	// HadAuthorization reports whether authorization metadata was present.
	HadAuthorization bool
	// Authorized reports whether it carried the stub's bearer token.
	Authorized bool
	Code       codes.Code
}

type version struct {
	number int
	data   []byte
	crc    int64
	create *timestamppb.Timestamp
}

type secret struct {
	proto    *secretmanagerpb.Secret
	versions []version
}

// Server is the stub service. Addr is its 127.0.0.1 listen address.
type Server struct {
	secretmanagerpb.UnimplementedSecretManagerServiceServer

	// Addr is host:port for option.WithEndpoint.
	Addr string

	token string

	mu       sync.Mutex
	secrets  map[string]*secret
	requests []Request
}

// New starts a stub service that accepts only the given bearer token. It is
// stopped when the test ends.
func New(t testing.TB, token string) *Server {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("gcpsmtest: listen: %v", err)
	}
	s := &Server{Addr: lis.Addr().String(), token: token, secrets: make(map[string]*secret)}
	g := grpc.NewServer(grpc.UnaryInterceptor(s.intercept))
	secretmanagerpb.RegisterSecretManagerServiceServer(g, s)
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)
	return s
}

// NewToken returns a bearer token built at runtime for one test.
func NewToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("gcpsmtest: crypto/rand: %v", err))
	}
	return "gcpsmtest-" + hex.EncodeToString(b)
}

// Requests returns a copy of the RPCs received so far.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, len(s.requests))
	copy(out, s.requests)
	return out
}

type resourceRequest interface {
	GetName() string
}

type parentRequest interface {
	GetParent() string
}

func (s *Server) intercept(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	rec := Request{Method: info.FullMethod}
	switch r := req.(type) {
	case parentRequest:
		rec.Resource = r.GetParent()
	case resourceRequest:
		rec.Resource = r.GetName()
	}
	md, _ := metadata.FromIncomingContext(ctx)
	auth := md.Get("authorization")
	rec.HadAuthorization = len(auth) > 0
	rec.Authorized = len(auth) == 1 && auth[0] == "Bearer "+s.token

	var resp any
	var err error
	if rec.Authorized {
		resp, err = handler(ctx, req)
	} else {
		err = errMissingCredential
	}
	rec.Code = status.Code(err)
	s.mu.Lock()
	s.requests = append(s.requests, rec)
	s.mu.Unlock()
	return resp, err
}

// CreateSecret creates a secret with no versions.
func (s *Server) CreateSecret(_ context.Context, req *secretmanagerpb.CreateSecretRequest) (*secretmanagerpb.Secret, error) {
	if !parentPattern.MatchString(req.GetParent()) {
		return nil, status.Errorf(codes.InvalidArgument, "parent %q must be projects/* or projects/*/locations/*", req.GetParent())
	}
	if !secretIDPattern.MatchString(req.GetSecretId()) {
		return nil, status.Errorf(codes.InvalidArgument, "secret_id %q must be 1 to 255 letters, numerals, hyphens or underscores", req.GetSecretId())
	}
	if req.GetSecret() == nil {
		return nil, status.Error(codes.InvalidArgument, "secret is required")
	}
	name := req.GetParent() + "/secrets/" + req.GetSecretId()

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.secrets[name]; ok {
		return nil, status.Errorf(codes.AlreadyExists, "Secret [%s] already exists.", name)
	}
	created := &secretmanagerpb.Secret{
		Name:        name,
		Replication: req.GetSecret().GetReplication(),
		CreateTime:  timestamppb.Now(),
		Labels:      req.GetSecret().GetLabels(),
	}
	s.secrets[name] = &secret{proto: created}
	return created, nil
}

// AddSecretVersion attaches a new ENABLED version holding the payload.
func (s *Server) AddSecretVersion(_ context.Context, req *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
	if !secretNamePattern.MatchString(req.GetParent()) {
		return nil, status.Errorf(codes.InvalidArgument, "parent %q must be projects/*/secrets/*", req.GetParent())
	}
	if req.GetPayload() == nil {
		return nil, status.Error(codes.InvalidArgument, "payload is required")
	}
	data := req.GetPayload().GetData()
	if len(data) > maxPayloadBytes {
		return nil, status.Errorf(codes.InvalidArgument, "payload data is %d bytes; the limit is 64KiB", len(data))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	sec, ok := s.secrets[req.GetParent()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "Secret [%s] not found.", req.GetParent())
	}
	v := version{
		number: len(sec.versions) + 1,
		data:   append([]byte(nil), data...),
		crc:    int64(crc32.Checksum(data, castagnoli)),
		create: timestamppb.Now(),
	}
	sec.versions = append(sec.versions, v)
	return &secretmanagerpb.SecretVersion{
		Name:       req.GetParent() + "/versions/" + strconv.Itoa(v.number),
		CreateTime: v.create,
		State:      secretmanagerpb.SecretVersion_ENABLED,
	}, nil
}

// AccessSecretVersion returns a version's payload; "latest" is the most
// recently created version.
func (s *Server) AccessSecretVersion(_ context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	m := versionNamePattern.FindStringSubmatch(req.GetName())
	if m == nil {
		return nil, status.Errorf(codes.InvalidArgument, "name %q must be projects/*/secrets/*/versions/*", req.GetName())
	}
	secretName, versionID := m[1], m[2]

	s.mu.Lock()
	defer s.mu.Unlock()
	sec, ok := s.secrets[secretName]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "Secret [%s] not found or has no versions.", secretName)
	}
	var found *version
	if versionID == "latest" {
		if len(sec.versions) > 0 {
			found = &sec.versions[len(sec.versions)-1]
		}
	} else if n, err := strconv.Atoi(versionID); err == nil && n >= 1 && n <= len(sec.versions) {
		found = &sec.versions[n-1]
	}
	if found == nil {
		return nil, status.Errorf(codes.NotFound, "Secret Version [%s] not found.", req.GetName())
	}
	crc := found.crc
	return &secretmanagerpb.AccessSecretVersionResponse{
		Name: secretName + "/versions/" + strconv.Itoa(found.number),
		Payload: &secretmanagerpb.SecretPayload{
			Data:       append([]byte(nil), found.data...),
			DataCrc32C: &crc,
		},
	}, nil
}

// DeleteSecret deletes a secret and all of its versions.
func (s *Server) DeleteSecret(_ context.Context, req *secretmanagerpb.DeleteSecretRequest) (*emptypb.Empty, error) {
	if !secretNamePattern.MatchString(req.GetName()) {
		return nil, status.Errorf(codes.InvalidArgument, "name %q must be projects/*/secrets/*", req.GetName())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.secrets[req.GetName()]; !ok {
		return nil, status.Errorf(codes.NotFound, "Secret [%s] not found.", req.GetName())
	}
	delete(s.secrets, req.GetName())
	return &emptypb.Empty{}, nil
}

// RPC returns the short RPC name of r, e.g. CreateSecret.
func (r Request) RPC() string {
	return r.Method[strings.LastIndex(r.Method, "/")+1:]
}
