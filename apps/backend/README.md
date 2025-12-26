# AIM Backend

Agent Identity Management (AIM) backend service - a Go-based API server for managing agent identities, trust scores, and organizational policies.

## Test Coverage

| Package | Coverage |
|---------|----------|
| `internal/domain` | 97.5% |
| `internal/infrastructure/cache` | 98.2% |
| `internal/infrastructure/crypto` | 92.3% |
| `internal/crypto` | 90.0% |
| `internal/infrastructure/database` | 88.9% |
| `internal/interfaces/http/middleware` | 75.3% |
| `internal/infrastructure/auth` | 70.8% |
| `internal/infrastructure/email` | 70.0% |
| `internal/interfaces/http/handlers` | 57.8% |
| `internal/application` | 33.5% |

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Generate coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Run tests for a specific package
go test ./internal/interfaces/http/handlers/... -v
```

### Coverage Goals

- **Core packages** (`domain`, `infrastructure/*`): 80%+
- **Handler packages** (`interfaces/http/*`): 60%+
- **Application services**: Work in progress

## Project Structure

```
apps/backend/
├── cmd/                    # Application entrypoints
│   └── server/            # Main server binary
├── internal/
│   ├── application/       # Application services (use cases)
│   ├── domain/            # Domain models and business logic
│   ├── infrastructure/    # External concerns
│   │   ├── auth/         # Authentication (JWT, OAuth)
│   │   ├── cache/        # Caching layer
│   │   ├── crypto/       # Cryptographic operations
│   │   ├── database/     # Database connections
│   │   ├── email/        # Email service
│   │   └── repository/   # Data persistence
│   └── interfaces/
│       └── http/
│           ├── handlers/  # HTTP request handlers
│           └── middleware/ # HTTP middleware
├── scripts/               # Utility scripts
└── tests/
    └── integration/       # Integration tests
```

## Development

### Prerequisites

- Go 1.21+
- PostgreSQL 15+
- Docker (optional, for local development)

### Running Locally

```bash
# Copy environment file
cp .env.example .env

# Run the server
go run cmd/server/main.go

# Or build and run
go build -o aim-backend cmd/server/main.go
./aim-backend
```

### Building

```bash
# Build for current platform
go build -o aim-backend cmd/server/main.go

# Build for Linux (production)
GOOS=linux GOARCH=amd64 go build -o aim-backend cmd/server/main.go
```

## API Documentation

API documentation is available via Swagger UI when running the server:
- Local: http://localhost:8080/swagger/

## License

See the root LICENSE file for details.
