# S3Dir Development Guide

This document provides comprehensive information for developers who want to contribute to S3Dir, understand its internals, or extend its functionality.

## Table of Contents

- [Project Structure](#project-structure)
- [Architecture](#architecture)
- [Development Setup](#development-setup)
- [Testing](#testing)
- [Code Style](#code-style)
- [Adding New Features](#adding-new-features)
- [API Implementation](#api-implementation)
- [Debugging](#debugging)
- [Performance Optimization](#performance-optimization)
- [Release Process](#release-process)

## Project Structure

```
s3dir/
├── cmd/
│   └── s3dir/          # Main application entry point
│       └── main.go     # Server initialization and startup
├── pkg/
│   ├── auth/           # Authentication layer
│   │   └── auth.go     # AWS Signature V4 authentication
│   ├── s3/             # S3 API handlers
│   │   ├── handler.go  # HTTP request handlers
│   │   └── types.go    # XML response types
│   └── storage/        # Storage layer
│       └── storage.go  # Filesystem-based storage implementation
├── internal/
│   └── config/         # Configuration management
│       └── config.go   # Environment-based configuration
├── test/
│   └── integration/    # Integration tests
│       └── integration_test.go
├── go.mod              # Go module definition
├── go.sum              # Go module checksums
├── README.md           # User documentation
└── DEVELOPMENT.md      # This file
```

### Package Responsibilities

#### `cmd/s3dir`
- Application entry point
- Server lifecycle management
- Middleware configuration (CORS, auth)
- Graceful shutdown handling

#### `pkg/auth`
- Request authentication
- AWS Signature V4 validation (simplified)
- Authentication middleware

#### `pkg/s3`
- S3 API request handling
- URL path parsing
- XML response serialization
- Error responses
- Routing logic

#### `pkg/storage`
- Filesystem operations
- Bucket management (directories)
- Object CRUD (file operations)
- Directory traversal and listing
- Prefix and delimiter filtering

#### `internal/config`
- Configuration loading from environment
- Configuration validation
- Default values

## Architecture

### Request Flow

```
1. HTTP Request
   ↓
2. CORS Middleware
   ↓
3. Authentication Middleware (optional)
   ↓
4. S3 Handler (ServeHTTP)
   ↓
5. Path Parsing (bucket/key extraction)
   ↓
6. Operation Routing (service/bucket/object level)
   ↓
7. Handler Method (listBuckets, getObject, etc.)
   ↓
8. Storage Layer (filesystem operations)
   ↓
9. Response (XML or binary data)
```

### Data Flow

```
HTTP Request → Handler → Storage → Filesystem
                  ↓
             XML Response
```

### Key Design Decisions

1. **Filesystem as Storage**: Uses the local filesystem directly for simplicity and transparency
2. **Path-style URLs**: Supports only path-style URLs (`/bucket/key`), not virtual-host style
3. **Atomic Writes**: Uses temporary files and rename for atomic object creation
4. **No Database**: All metadata derived from filesystem (stat calls)
5. **Minimal Dependencies**: Standard library only, no external dependencies

## Development Setup

### Prerequisites

- Go 1.19 or later
- Git
- Make (optional, for build automation)

### Clone and Build

```bash
# Clone the repository
git clone https://github.com/s3dir/s3dir
cd s3dir

# Download dependencies
go mod download

# Build
go build -o s3dir ./cmd/s3dir

# Run
./s3dir
```

### Development Mode

```bash
# Run with auto-reload (using air or similar)
go install github.com/cosmtrek/air@latest
air

# Or run directly with go run
S3DIR_VERBOSE=true go run ./cmd/s3dir
```

### IDE Setup

#### VS Code

Install the Go extension and add to `.vscode/settings.json`:

```json
{
  "go.lintTool": "golangci-lint",
  "go.testFlags": ["-v"],
  "go.coverOnSave": true
}
```

#### GoLand/IntelliJ

The project should work out-of-the-box. Configure run configurations for easier debugging.

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test
go test -v ./pkg/storage -run TestListObjects

# Run tests with race detection
go test -race ./...
```

### Test Structure

Tests are organized by package:

- **Unit Tests**: Test individual functions and methods
  - `pkg/storage/storage_test.go`: Storage layer tests
  - `pkg/s3/handler_test.go`: HTTP handler tests
  - `internal/config/config_test.go`: Configuration tests

- **Integration Tests**: Test complete workflows
  - `test/integration/integration_test.go`: End-to-end API tests

### Writing Tests

#### Unit Test Example

```go
func TestPutObject(t *testing.T) {
    storage, cleanup := setupTestStorage(t)
    defer cleanup()

    // Setup
    storage.CreateBucket("test-bucket")
    testData := []byte("Hello, World!")
    
    // Execute
    err := storage.PutObject("test-bucket", "test.txt", 
        bytes.NewReader(testData), int64(len(testData)))
    
    // Assert
    if err != nil {
        t.Fatalf("Failed to put object: %v", err)
    }
    
    // Verify
    reader, info, err := storage.GetObject("test-bucket", "test.txt")
    if err != nil {
        t.Fatalf("Failed to get object: %v", err)
    }
    defer reader.Close()
    
    content, _ := io.ReadAll(reader)
    if !bytes.Equal(content, testData) {
        t.Errorf("Content mismatch")
    }
}
```

#### Integration Test Example

```go
func TestFullWorkflow(t *testing.T) {
    server, cleanup := setupIntegrationTest(t)
    defer cleanup()

    client := &http.Client{}

    // Test complete workflow
    // 1. Create bucket
    // 2. Upload object
    // 3. Download object
    // 4. Delete object
    // 5. Delete bucket
}
```

### Test Coverage Goals

- Minimum 80% code coverage
- 100% coverage for critical paths (storage operations, auth)
- All error paths tested
- Edge cases covered

## Code Style

### Go Best Practices

Follow standard Go conventions:

- Use `gofmt` for formatting
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use meaningful variable names
- Keep functions small and focused
- Document exported functions and types

### Linting

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run

# Auto-fix issues
golangci-lint run --fix
```

### Code Organization

1. **Package-level documentation**: Every package should have a doc comment
2. **Function documentation**: All exported functions must be documented
3. **Error handling**: Always check and handle errors appropriately
4. **Logging**: Use fmt.Printf for simple logging (verbose mode)
5. **Constants**: Use constants for magic numbers and strings

### Example Code Style

```go
// Good
func (s *Storage) PutObject(bucket, key string, reader io.Reader, size int64) error {
    objectPath := s.objectPath(bucket, key)
    
    if err := os.MkdirAll(filepath.Dir(objectPath), 0755); err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }
    
    // ... implementation
}

// Bad - unclear variable names, no error wrapping
func (s *Storage) PutObject(b, k string, r io.Reader, sz int64) error {
    p := s.objectPath(b, k)
    
    if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
        return err // No context
    }
    
    // ... implementation
}
```

## Adding New Features

### Adding a New S3 Operation

Example: Adding CopyObject support

1. **Add storage method** (`pkg/storage/storage.go`):

```go
func (s *Storage) CopyObject(srcBucket, srcKey, dstBucket, dstKey string) error {
    srcPath := s.objectPath(srcBucket, srcKey)
    dstPath := s.objectPath(dstBucket, dstKey)
    
    // Read source
    data, err := os.ReadFile(srcPath)
    if err != nil {
        return fmt.Errorf("failed to read source: %w", err)
    }
    
    // Write destination
    if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }
    
    if err := os.WriteFile(dstPath, data, 0644); err != nil {
        return fmt.Errorf("failed to write destination: %w", err)
    }
    
    return nil
}
```

2. **Add handler method** (`pkg/s3/handler.go`):

```go
func (h *Handler) copyObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
    // Parse copy source from x-amz-copy-source header
    copySource := r.Header.Get("x-amz-copy-source")
    if copySource == "" {
        writeError(w, "InvalidRequest", "Missing copy source", http.StatusBadRequest)
        return
    }
    
    // Parse source bucket and key
    srcBucket, srcKey := parseCopySource(copySource)
    
    // Perform copy
    if err := h.storage.CopyObject(srcBucket, srcKey, bucket, key); err != nil {
        writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Return success response
    w.WriteHeader(http.StatusOK)
}
```

3. **Update routing** (`pkg/s3/handler.go`):

```go
func (h *Handler) handleObjectOperation(w http.ResponseWriter, r *http.Request, bucket, key string) {
    // Check for copy operation
    if r.Header.Get("x-amz-copy-source") != "" && r.Method == http.MethodPut {
        h.copyObject(w, r, bucket, key)
        return
    }
    
    // ... existing routing
}
```

4. **Add tests**:

```go
func TestCopyObject(t *testing.T) {
    storage, cleanup := setupTestStorage(t)
    defer cleanup()
    
    // Create bucket and source object
    storage.CreateBucket("bucket")
    data := []byte("test data")
    storage.PutObject("bucket", "source.txt", bytes.NewReader(data), int64(len(data)))
    
    // Copy object
    err := storage.CopyObject("bucket", "source.txt", "bucket", "dest.txt")
    if err != nil {
        t.Fatalf("Failed to copy: %v", err)
    }
    
    // Verify copy
    reader, info, err := storage.GetObject("bucket", "dest.txt")
    if err != nil {
        t.Fatalf("Failed to get copy: %v", err)
    }
    defer reader.Close()
    
    content, _ := io.ReadAll(reader)
    if !bytes.Equal(content, data) {
        t.Error("Copy content mismatch")
    }
}
```

## API Implementation

### S3 API Specification

Refer to the [AWS S3 API Reference](https://docs.aws.amazon.com/AmazonS3/latest/API/Welcome.html) for operation specifications.

### Error Responses

S3 error responses follow this format:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist</Message>
</Error>
```

Common error codes:
- `NoSuchBucket`: Bucket not found
- `NoSuchKey`: Object not found
- `BucketAlreadyExists`: Bucket already exists
- `BucketNotEmpty`: Cannot delete non-empty bucket
- `InvalidRequest`: Malformed request
- `InternalError`: Server error

### Success Responses

Different operations return different responses:

- **PutObject**: 200 OK with ETag header
- **GetObject**: 200 OK with object content
- **DeleteObject**: 204 No Content
- **HeadObject**: 200 OK with metadata headers
- **ListObjects**: 200 OK with XML listing

## Debugging

### Verbose Logging

```bash
S3DIR_VERBOSE=true ./s3dir
```

This logs all incoming requests.

### Using Debugger

```bash
# VS Code: Set breakpoints and press F5

# Delve
dlv debug ./cmd/s3dir
```

### Common Issues

**Problem**: Objects not being created

```bash
# Check permissions
ls -la data/

# Check logs
S3DIR_VERBOSE=true ./s3dir
```

**Problem**: Authentication failing

```bash
# Disable auth for debugging
S3DIR_ENABLE_AUTH=false ./s3dir
```

## Performance Optimization

### Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=. ./pkg/storage
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=. ./pkg/storage
go tool pprof mem.prof
```

### Optimization Tips

1. **Use io.Copy**: Efficient for streaming large files
2. **Minimize allocations**: Reuse buffers where possible
3. **Batch operations**: Use filepath.Walk for listings instead of multiple stat calls
4. **Atomic operations**: Use rename for atomic file updates

### Benchmarking

```go
func BenchmarkPutObject(b *testing.B) {
    storage, cleanup := setupTestStorage(b)
    defer cleanup()
    
    storage.CreateBucket("bench")
    data := make([]byte, 1024*1024) // 1MB
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        key := fmt.Sprintf("object-%d", i)
        storage.PutObject("bench", key, bytes.NewReader(data), int64(len(data)))
    }
}
```

## Release Process

### Version Numbering

Follow [Semantic Versioning](https://semver.org/):
- Major: Breaking changes
- Minor: New features (backward compatible)
- Patch: Bug fixes

### Creating a Release

```bash
# Tag the release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# Build binaries
GOOS=linux GOARCH=amd64 go build -o s3dir-linux-amd64 ./cmd/s3dir
GOOS=darwin GOARCH=amd64 go build -o s3dir-darwin-amd64 ./cmd/s3dir
GOOS=windows GOARCH=amd64 go build -o s3dir-windows-amd64.exe ./cmd/s3dir

# Create checksums
sha256sum s3dir-* > checksums.txt
```

### Pre-release Checklist

- [ ] All tests passing
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Version bumped in code
- [ ] No security vulnerabilities (`go list -m all | nancy sleuth`)

## Contributing Guidelines

1. **Fork the repository**
2. **Create a feature branch**: `git checkout -b feature/my-feature`
3. **Write tests** for new functionality
4. **Ensure tests pass**: `go test ./...`
5. **Follow code style**: `golangci-lint run`
6. **Commit changes**: Use conventional commits
7. **Push to fork**: `git push origin feature/my-feature`
8. **Create Pull Request**

### Commit Message Format

```
type(scope): subject

body

footer
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `chore`

Example:
```
feat(storage): add support for object metadata

Implement custom metadata storage using extended file attributes.
This allows preserving S3 metadata when using filesystem storage.

Closes #123
```

## Questions?

- Open an issue on GitHub
- Check existing documentation
- Review test files for usage examples

## Resources

- [AWS S3 API Reference](https://docs.aws.amazon.com/AmazonS3/latest/API/Welcome.html)
- [Go Documentation](https://golang.org/doc/)
- [Effective Go](https://golang.org/doc/effective_go.html)
- [AWS SDK Examples](https://github.com/aws/aws-sdk-go/tree/main/example)
