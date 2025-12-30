# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

S3Dir is a lightweight S3-compatible API server that exposes a local directory as S3 storage. It's designed for local development, testing, and scenarios requiring S3-compatible storage without cloud services.

**Key characteristics:**
- Pure Go, standard library only (except AWS SDK v2 in examples)
- Direct filesystem I/O - buckets are directories, objects are files
- Single binary deployment
- Production code is in `cmd/`, `pkg/`, and `internal/`

## Build, Test, and Lint Commands

```bash
# Build
go build -o s3dir ./cmd/s3dir

# Run locally
S3DIR_VERBOSE=true ./s3dir
# Server listens on http://0.0.0.0:8000 by default

# Run all tests
go test -v ./...

# Run tests with race detection (IMPORTANT: always use before committing)
go test -race ./...

# Run specific test
go test -v ./pkg/storage -run TestMultipartUploadWorkflow

# Run specific package tests
go test -v ./pkg/s3
go test -v ./pkg/storage

# Run tests multiple times to catch race conditions
go test -race -count=5 ./pkg/storage

# Generate coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Lint (must pass before committing)
golangci-lint run --timeout 5m

# Format code
go fmt ./...
```

## Architecture

### Request Flow
```
HTTP Request 
  → CORS Middleware 
  → Auth Middleware (optional)
  → S3 Handler (pkg/s3/handler.go:ServeHTTP)
  → Path parsing (bucket/key extraction)
  → Route to handler method
  → Storage layer (pkg/storage/)
  → Filesystem operations
  → XML/binary response
```

### Core Components

**`cmd/s3dir/main.go`**
- Server initialization and lifecycle
- Middleware setup (CORS, auth)
- HTTP server configuration with optimized settings for large file uploads
- Graceful shutdown handling

**`pkg/s3/handler.go`** (598 lines)
- S3 API HTTP handlers for all operations
- Path parsing: `parsePath()` extracts bucket/key from URL path
- Three-level routing: service (list buckets) → bucket (list objects) → object (CRUD)
- Multipart upload handlers (lines 413-598)
- Query parameter handling for multipart operations using `Has()` not `Get()`

**`pkg/s3/types.go`**
- XML request/response structs for S3 API
- Multipart upload types (7 structs for multipart operations)

**`pkg/storage/storage.go`**
- Filesystem-based storage operations
- Buckets = directories under `baseDir`
- Objects = files with path preserved
- Uses atomic writes: CreateTemp → Write → Rename
- Streaming I/O with fixed buffers (32KB for uploads, 1MB for assembly)

**`pkg/storage/multipart.go`** (442 lines)
- Multipart upload manager with thread-safe operations (sync.RWMutex)
- Parts stored in `.multipart/{uploadID}/part-{N}` under baseDir
- **Critical optimization**: Assembly uses MD5-of-MD5s (not full-file hash) to prevent timeouts on large files
- Background cleanup: hourly scan removes uploads inactive >24 hours
- Startup cleanup: removes all `.multipart` directories (orphaned uploads)

**`pkg/auth/auth.go`**
- AWS Signature V4 authentication (simplified)
- Middleware for authentication enforcement
- Currently validates access key only (not full signature verification)

**`internal/config/config.go`**
- Environment variable configuration
- Validation and defaults

### Storage Layer Details

**Directory structure:**
```
{baseDir}/
  {bucket}/              # Each bucket is a directory
    {key}                # Objects are files (path preserved)
    path/to/object.txt   # Nested paths become nested directories
  .multipart/            # Temporary multipart upload storage
    {uploadID}/
      part-1
      part-2
      metadata.json
```

**Multipart upload lifecycle:**
1. `InitiateUpload` → Creates `.multipart/{uploadID}/` directory
2. `UploadPart` → Writes `part-{N}` files, calculates MD5 per part
3. `CompleteUpload` → Assembles parts into final object using 1MB buffer, deletes parts
4. `AbortUpload` → Deletes parts directory

**Critical implementation notes:**
- `UploadPart` has defensive `MkdirAll` to prevent race conditions in tests
- Assembly uses `io.CopyBuffer` with 1MB buffer (not 32KB) for speed
- ETag for multipart = MD5(concatenated part MD5s) + "-" + part count
- This prevents timeout on large files by avoiding full-file hashing

### Memory and Performance Optimizations

**Recent critical fixes (for 22GB+ file uploads):**
1. Changed from full-file MD5 to MD5-of-MD5s during assembly (500,000x faster)
2. Increased assembly buffer from 32KB to 1MB (3x faster assembly)
3. Use `io.CopyBuffer` with fixed buffers everywhere (constant memory)
4. Single-pass assembly (was reading parts twice - once for copy, once for hash)

**Memory usage:**
- Regular uploads: <100MB constant (regardless of file size)
- Multipart uploads: ~32KB per concurrent part upload
- Assembly: ~1MB buffer + minimal overhead

## Testing Strategy

**Test organization:**
- Unit tests in `*_test.go` files alongside source
- Integration tests in `test/integration/`
- Each test uses `os.MkdirTemp()` for isolation

**Key test patterns:**
- Multipart tests verify full workflow, error cases, out-of-order parts, cleanup
- Race detection is mandatory (`-race` flag)
- Coverage tests use pattern: `./cmd/... ./internal/... ./pkg/... ./test/...`

**Common test issues:**
- "File exists" errors: Usually from missing `MkdirAll` before file operations
- Race conditions: Use `go test -race -count=N` to catch
- Cleanup: Tests must use unique temp directories

## Common Development Scenarios

### Adding a new S3 API operation

1. Add XML types to `pkg/s3/types.go` if needed
2. Add handler method to `pkg/s3/handler.go`
3. Wire it up in routing (handleServiceOperation/handleBucketOperation/handleObjectOperation)
4. Implement storage logic in `pkg/storage/storage.go` if needed
5. Add tests to appropriate `*_test.go` file
6. Test with AWS CLI: `aws --endpoint-url=http://localhost:8000 s3 ...`

### Modifying multipart upload behavior

**Files to modify:**
- `pkg/storage/multipart.go` - Core logic
- `pkg/s3/handler.go` - HTTP handlers (lines 413-598)
- `pkg/s3/types.go` - XML types if changing API contract

**Critical considerations:**
- Maintain thread safety (all uploads map access under mutex)
- Update both `LastActivity` timestamp for cleanup tracking
- Preserve MD5-of-MD5s ETag calculation (don't hash full file)
- Test with large files to ensure no timeout

### Debugging multipart upload issues

```bash
# Enable verbose logging
S3DIR_VERBOSE=true ./s3dir

# Watch for these log patterns:
# POST /{bucket}/{key}?uploads - Initiate
# PUT /{bucket}/{key}?uploadId=X&partNumber=N - Upload part
# POST /{bucket}/{key}?uploadId=X - Complete
# DELETE /{bucket}/{key}?uploadId=X - Abort

# Check temp files
ls -la ./data/.multipart/

# Test with AWS CLI verbose
aws --endpoint-url=http://localhost:8000 s3 cp --debug large-file s3://bucket/
```

## CI/CD

GitHub Actions workflows in `.github/workflows/`:
- `ci.yml` - Tests on Ubuntu/macOS with Go 1.24/1.25
- `docker-publish.yml` - Docker image builds
- `release.yml` - Release binaries for multiple platforms

**Important:**
- All tests must pass with `-race` flag
- golangci-lint must pass
- Go 1.25 is the primary version

## Dependencies

- **Production**: Zero external dependencies (standard library only)
- **Examples**: AWS SDK v2 (`github.com/aws/aws-sdk-go-v2`)
- **Development**: golangci-lint for linting

## Configuration

Environment variables (see `internal/config/config.go`):
- `S3DIR_HOST` - Bind address (default: 0.0.0.0)
- `S3DIR_PORT` - Port (default: 8000)
- `S3DIR_DATA_DIR` - Storage directory (default: ./data)
- `S3DIR_ENABLE_AUTH` - Enable authentication (default: false)
- `S3DIR_ACCESS_KEY_ID` - Access key if auth enabled
- `S3DIR_SECRET_ACCESS_KEY` - Secret key if auth enabled
- `S3DIR_READ_ONLY` - Read-only mode (default: false)
- `S3DIR_VERBOSE` - Verbose logging (default: false)

## Known Limitations and Design Decisions

1. **Path-style URLs only** - No virtual-host style (`bucket.s3.amazonaws.com`)
2. **Simplified authentication** - Only access key validation, not full SigV4
3. **No versioning** - Objects are overwritten, no version tracking
4. **No ACLs** - No per-object access control
5. **Case-sensitive paths** - Filesystem dependent
6. **Large file uploads** - Clients should use multipart for >1GB files to minimize memory

## Performance Expectations

On modern hardware:
- Small files (<1MB): 1000+ ops/sec
- Large files (>100MB): Limited by disk I/O (~500MB/sec assembly)
- Listings: 10,000+ objects/sec
- Multipart assembly: ~500MB/sec typical (22GB file ~30-40 seconds)
