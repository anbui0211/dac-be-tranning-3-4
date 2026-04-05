# Agent Guidelines for be-training-3-4

## Project Overview

This is a Go-based pub-sub data processing system with two microservices:
- **pub-service**: Publishes messages to SQS from CSV files stored in S3
- **sub-service**: Consumes messages from SQS with Redis deduplication

Both services use worker pools with channels for concurrent processing.

## Build Commands

### Build Services
```bash
# Build pub-service
cd pub-service && go build -o pub-service .

# Build sub-service
cd sub-service && go build -o sub-service .
```

### Run Services Locally
```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f pub-service
docker-compose logs -f sub-service
```

### Lint and Format
```bash
# Format code (standard Go formatting)
cd pub-service && go fmt ./...
cd sub-service && go fmt ./...

# Run vet (static analysis)
cd pub-service && go vet ./...
cd sub-service && go vet ./...
```

### Testing
No test framework is currently configured. When adding tests:
- Use standard `go test` commands
- Follow Go testing conventions in `*_test.go` files

## Code Style Guidelines

### Imports
- Group imports into three blocks: stdlib, third-party, local modules
- Sort imports alphabetically within each block
- Do not use blank imports (`import _`)

### Formatting
- Use `gofmt` (standard Go formatter)
- Indent with tabs, not spaces
- No trailing whitespace
- Max line length: ~120 characters

### Types and Structs
- Structs should have a clear constructor function: `NewX() (*X, error)`
- Use exported struct fields only when necessary
- Prefer pointer receivers for methods that modify struct
- Exported types must have documentation comments

### Naming Conventions
- **Files**: lowercase with underscores (e.g., `batch.go`, `sqs_provider.go`)
- **Packages**: lowercase, single word when possible (e.g., `handlers`, `providers`)
- **Functions/Methods**: PascalCase for exported, camelCase for private
- **Variables/Constants**: camelCase
- **Interfaces**: PascalCase, typically named with -er suffix (e.g., `Reader`, `Writer`)
- **Constants**: PascalCase for exported

### Error Handling
- Always wrap errors with context using `fmt.Errorf("operation: %w", err)`
- Return errors immediately, don't store them
- Validate required environment variables in constructors
- Log errors with context (e.g., user ID, message ID)
- Use `defer` for cleanup (closing files, connections)

### Concurrency Patterns
- Use worker pools with channels for parallel processing
- Always use `sync.WaitGroup` to track goroutines
- Close channels when done sending
- Use `select` for graceful shutdown
- Prefer buffered channels to avoid blocking

### Provider Pattern
- External service clients (S3, SQS, Redis) go in `providers/` package
- Each provider has:
  - Constructor: `NewProvider(ctx) (*Provider, error)`
  - Methods with `ctx` as first parameter
  - Getter methods: `GetClient()`, `GetQueueURL()`, etc.
  - Close/cleanup method if needed

### Logging
- Use standard `log` package with `log.LstdFlags | log.Lshortfile`
- Include context in logs (user ID, file name, worker ID)
- Log at appropriate levels:
  - Info: Normal operations, startup, shutdown
  - Error: Failures, retries
  - Fatal: Unrecoverable errors (use `log.Fatalf`)

### Environment Variables
- Read in constructors with `os.Getenv()`
- Validate required variables and return errors
- Provide sensible defaults where appropriate
- Document required variables in README

### JSON Encoding
- Use `json` struct tags with snake_case field names
- Always validate JSON input before use
- Use `ShouldBindJSON` for API requests (Gin framework)

### API Handlers
- Use Gin framework for HTTP routing
- Handle errors with appropriate status codes
- Return JSON responses
- Keep handlers thin - business logic in handlers package
- Route setup in `main.go`

### Constants
- Batch sizes: 10 messages for SQS
- Worker counts: 20 (pub), 50 (sub) - configurable via env vars
- Timeouts: 30s for shutdown, 20s for SQS long polling
- Channel buffer sizes: 100 (sub), 1000 (pub row channel), 100 (pub message channel)
