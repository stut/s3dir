# S3Dir - S3-Compatible Directory Server

S3Dir is a lightweight, high-performance S3-compatible API server that exposes a local directory as an S3 bucket. It implements core S3 API operations, making it perfect for local development, testing, and scenarios where you need S3-compatible storage without the complexity of cloud services.

## Features

- **S3-Compatible API**: Implements core S3 operations (GET, PUT, DELETE, HEAD, LIST)
- **Multipart Uploads**: Full support for large file uploads via S3 multipart upload protocol
- **File-based Storage**: Uses the local filesystem for simple, transparent storage
- **Multiple Buckets**: Support for creating and managing multiple buckets
- **Authentication**: Optional AWS Signature V4 authentication support
- **Read-Only Mode**: Run in read-only mode for serving static content
- **CORS Support**: Built-in CORS headers for browser compatibility
- **Lightweight**: No external dependencies, single binary deployment
- **Fast**: Direct filesystem operations with minimal overhead

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/s3dir/s3dir
cd s3dir

# Build the binary
go build -o s3dir ./cmd/s3dir

# Run the server
./s3dir
```

The server will start on `http://0.0.0.0:8000` by default, using `./data` as the storage directory.

### Basic Usage

```bash
# Start the server
./s3dir

# In another terminal, use the AWS CLI or any S3-compatible client
# Configure AWS CLI with dummy credentials (if auth is disabled)
aws configure set aws_access_key_id dummy
aws configure set aws_secret_access_key dummy

# Create a bucket
aws --endpoint-url=http://localhost:8000 s3 mb s3://my-bucket

# Upload a file
aws --endpoint-url=http://localhost:8000 s3 cp myfile.txt s3://my-bucket/

# List files
aws --endpoint-url=http://localhost:8000 s3 ls s3://my-bucket/

# Download a file
aws --endpoint-url=http://localhost:8000 s3 cp s3://my-bucket/myfile.txt downloaded.txt

# Delete a file
aws --endpoint-url=http://localhost:8000 s3 rm s3://my-bucket/myfile.txt
```

## Configuration

S3Dir is configured using environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `S3DIR_HOST` | Server bind address | `0.0.0.0` |
| `S3DIR_PORT` | Server port | `8000` |
| `S3DIR_DATA_DIR` | Data storage directory | `./data` |
| `S3DIR_ACCESS_KEY_ID` | Access key for authentication | `` (disabled) |
| `S3DIR_SECRET_ACCESS_KEY` | Secret key for authentication | `` (disabled) |
| `S3DIR_ENABLE_AUTH` | Enable authentication | `false` |
| `S3DIR_READ_ONLY` | Enable read-only mode | `false` |
| `S3DIR_VERBOSE` | Enable verbose logging | `false` |

### Examples

#### Run on a custom port with verbose logging

```bash
S3DIR_PORT=9000 S3DIR_VERBOSE=true ./s3dir
```

#### Run with authentication enabled

```bash
S3DIR_ENABLE_AUTH=true \
S3DIR_ACCESS_KEY_ID=myaccesskey \
S3DIR_SECRET_ACCESS_KEY=mysecretkey \
./s3dir
```

Then configure your S3 client:

```bash
aws configure set aws_access_key_id myaccesskey
aws configure set aws_secret_access_key mysecretkey
```

#### Run in read-only mode

```bash
S3DIR_READ_ONLY=true ./s3dir
```

## Supported S3 Operations

### Service Operations

- **ListBuckets**: List all buckets (top-level directories)

### Bucket Operations

- **CreateBucket** (PUT): Create a new bucket
- **DeleteBucket** (DELETE): Delete an empty bucket
- **HeadBucket** (HEAD): Check if a bucket exists
- **ListObjects** (GET): List objects in a bucket with support for:
  - Prefix filtering
  - Delimiter-based hierarchical listing
  - Max keys limitation

### Object Operations

- **PutObject** (PUT): Upload an object
- **GetObject** (GET): Download an object
- **DeleteObject** (DELETE): Delete an object
- **HeadObject** (HEAD): Get object metadata

### Multipart Upload Operations

- **InitiateMultipartUpload** (POST): Start a multipart upload
- **UploadPart** (PUT): Upload a part of a multipart upload
- **CompleteMultipartUpload** (POST): Complete a multipart upload
- **AbortMultipartUpload** (DELETE): Abort an in-progress multipart upload
- **ListParts** (GET): List parts of a multipart upload
- **ListMultipartUploads** (GET): List in-progress multipart uploads

## Use Cases

### Local Development

Replace cloud S3 with a local instance for faster development and testing:

```bash
# Start S3Dir
S3DIR_PORT=9000 ./s3dir

# Point your application to localhost:9000 instead of s3.amazonaws.com
```

### Testing

Perfect for integration tests and CI/CD pipelines:

```bash
# Start S3Dir in background
S3DIR_PORT=9000 ./s3dir &
S3DIR_PID=$!

# Run your tests
go test ./...

# Cleanup
kill $S3DIR_PID
```

### Static File Serving

Serve static files through an S3-compatible interface:

```bash
# Copy your files to the data directory
mkdir -p data/website
cp -r public/* data/website/

# Start in read-only mode
S3DIR_DATA_DIR=data S3DIR_READ_ONLY=true ./s3dir
```

### Backup and Archive

Use S3Dir as a local S3-compatible backup target:

```bash
# Start S3Dir
S3DIR_DATA_DIR=/mnt/backups ./s3dir

# Use any S3 backup tool
restic -r s3:http://localhost:8000/backups init
restic -r s3:http://localhost:8000/backups backup /home
```

## Client Examples

### AWS CLI

```bash
# List buckets
aws --endpoint-url=http://localhost:8000 s3 ls

# Sync a directory
aws --endpoint-url=http://localhost:8000 s3 sync ./local-dir s3://my-bucket/remote-dir/

# Copy with metadata
aws --endpoint-url=http://localhost:8000 s3 cp file.txt s3://my-bucket/ --metadata key1=value1,key2=value2
```

### Python (boto3)

```python
import boto3

# Create S3 client
s3 = boto3.client(
    's3',
    endpoint_url='http://localhost:8000',
    aws_access_key_id='dummy',
    aws_secret_access_key='dummy',
)

# Upload file
s3.upload_file('local-file.txt', 'my-bucket', 'remote-file.txt')

# Download file
s3.download_file('my-bucket', 'remote-file.txt', 'downloaded.txt')

# List objects
response = s3.list_objects_v2(Bucket='my-bucket')
for obj in response.get('Contents', []):
    print(obj['Key'])
```

### Go (AWS SDK)

```go
package main

import (
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
)

func main() {
    sess := session.Must(session.NewSession(&aws.Config{
        Endpoint:         aws.String("http://localhost:8000"),
        Region:           aws.String("us-east-1"),
        Credentials:      credentials.NewStaticCredentials("dummy", "dummy", ""),
        S3ForcePathStyle: aws.Bool(true),
    }))

    svc := s3.New(sess)

    // List buckets
    result, err := svc.ListBuckets(nil)
    if err != nil {
        panic(err)
    }

    for _, b := range result.Buckets {
        println(*b.Name)
    }
}
```

### Node.js (AWS SDK)

```javascript
const AWS = require('aws-sdk');

const s3 = new AWS.S3({
    endpoint: 'http://localhost:8000',
    accessKeyId: 'dummy',
    secretAccessKey: 'dummy',
    s3ForcePathStyle: true,
    signatureVersion: 'v4',
});

// Upload file
s3.putObject({
    Bucket: 'my-bucket',
    Key: 'file.txt',
    Body: 'Hello, World!',
}, (err, data) => {
    if (err) console.error(err);
    else console.log('Upload successful:', data);
});

// List objects
s3.listObjectsV2({
    Bucket: 'my-bucket',
}, (err, data) => {
    if (err) console.error(err);
    else console.log('Objects:', data.Contents);
});
```

## Multipart Uploads

S3Dir supports multipart uploads for uploading large files efficiently. Files are uploaded in parts and then assembled on the server.

### Automatic Cleanup

S3Dir includes automatic cleanup mechanisms to prevent orphaned uploads from consuming disk space:

- **Startup Cleanup**: On server startup, all incomplete multipart uploads from previous runs are automatically cleaned up
- **Background Cleanup**: A background process runs every hour to remove uploads with no activity for more than 24 hours
- **Manual Abort**: Clients can explicitly abort uploads using the AbortMultipartUpload API

This ensures that abandoned uploads (due to client crashes, network disconnects, etc.) don't persist indefinitely.

### AWS CLI

```bash
# Upload a large file using multipart upload (automatic)
aws --endpoint-url=http://localhost:8000 s3 cp large-file.bin s3://my-bucket/

# The AWS CLI automatically uses multipart upload for files larger than 8MB
```

### Python (boto3)

```python
import boto3

s3 = boto3.client(
    's3',
    endpoint_url='http://localhost:8000',
    aws_access_key_id='dummy',
    aws_secret_access_key='dummy',
)

# Automatic multipart upload for large files
s3.upload_file('large-file.bin', 'my-bucket', 'large-file.bin')

# Manual multipart upload
response = s3.create_multipart_upload(Bucket='my-bucket', Key='manual-upload.bin')
upload_id = response['UploadId']

# Upload parts
parts = []
with open('large-file.bin', 'rb') as f:
    part_number = 1
    while True:
        data = f.read(5 * 1024 * 1024)  # 5MB chunks
        if not data:
            break
        
        part = s3.upload_part(
            Bucket='my-bucket',
            Key='manual-upload.bin',
            PartNumber=part_number,
            UploadId=upload_id,
            Body=data
        )
        
        parts.append({
            'PartNumber': part_number,
            'ETag': part['ETag']
        })
        part_number += 1

# Complete the upload
s3.complete_multipart_upload(
    Bucket='my-bucket',
    Key='manual-upload.bin',
    UploadId=upload_id,
    MultipartUpload={'Parts': parts}
)

# Abort a multipart upload if needed
# s3.abort_multipart_upload(Bucket='my-bucket', Key='manual-upload.bin', UploadId=upload_id)
```

### Go (AWS SDK)

```go
package main

import (
    "os"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func main() {
    sess := session.Must(session.NewSession(&aws.Config{
        Endpoint:         aws.String("http://localhost:8000"),
        Region:           aws.String("us-east-1"),
        Credentials:      credentials.NewStaticCredentials("dummy", "dummy", ""),
        S3ForcePathStyle: aws.Bool(true),
    }))

    uploader := s3manager.NewUploader(sess)

    file, err := os.Open("large-file.bin")
    if err != nil {
        panic(err)
    }
    defer file.Close()

    // Automatic multipart upload
    result, err := uploader.Upload(&s3manager.UploadInput{
        Bucket: aws.String("my-bucket"),
        Key:    aws.String("large-file.bin"),
        Body:   file,
    })
    
    if err != nil {
        panic(err)
    }
    
    println("Upload successful:", *result.Location)
}
```

## Architecture

S3Dir uses a layered architecture:

```
┌─────────────────────────────────────┐
│         HTTP Handler (S3 API)        │
│  - Request parsing                   │
│  - XML response formatting           │
│  - Error handling                    │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│      Storage Layer (Filesystem)      │
│  - Bucket management                 │
│  - Object CRUD operations            │
│  - Directory traversal               │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│         Local Filesystem             │
│  - Buckets as directories            │
│  - Objects as files                  │
└─────────────────────────────────────┘
```

## Limitations

- **Authentication**: Currently implements basic access key validation. Full AWS Signature V4 verification is simplified.
- **Object Metadata**: Custom metadata is not persisted (filesystem limitations).
- **Versioning**: Not supported.
- **ACLs**: Not supported.
- **Lifecycle Policies**: Not supported.
- **Server-Side Encryption**: Not supported.

## Performance

S3Dir is designed for speed:

- Direct filesystem I/O with minimal overhead
- Efficient directory walking for listings
- Atomic file operations using temporary files
- No database overhead

Typical performance (on modern hardware):
- Small files (< 1MB): 1000+ ops/sec
- Large files (> 100MB): Limited by disk I/O
- Listings: 10,000+ objects/sec

## Security Considerations

- **Local Use**: S3Dir is designed for local development and testing
- **Authentication**: Enable authentication for any network-accessible deployment
- **HTTPS**: S3Dir does not provide HTTPS. Use a reverse proxy (nginx, caddy) for production
- **File Permissions**: Respects filesystem permissions of the data directory
- **Path Traversal**: All paths are validated and constrained to the data directory

## Troubleshooting

### Server won't start

**Problem**: Permission denied on data directory

```bash
# Solution: Check directory permissions
chmod 755 ./data
```

**Problem**: Port already in use

```bash
# Solution: Use a different port
S3DIR_PORT=9000 ./s3dir
```

### Operations failing

**Problem**: Authentication errors

```bash
# Solution: Disable auth for local testing
S3DIR_ENABLE_AUTH=false ./s3dir
```

**Problem**: Cannot write objects

```bash
# Solution: Check if read-only mode is enabled
# Ensure S3DIR_READ_ONLY is not set to true
```

### Performance issues

**Problem**: Slow listings on large directories

```bash
# Solution: Use prefix filtering to narrow results
aws --endpoint-url=http://localhost:8000 s3 ls s3://bucket/prefix/
```

**Problem**: High memory usage when uploading very large files (>1GB)

```bash
# Solution: Use multipart uploads instead of single PUT requests
# AWS CLI automatically uses multipart for files >8MB:
aws --endpoint-url=http://localhost:8000 s3 cp large-file.bin s3://bucket/

# For other clients, configure multipart threshold:
# boto3: Set TransferConfig(multipart_threshold=...)
# AWS SDK v2: Use transfer manager with appropriate part size

# Why: Single PUT requests may buffer data in memory due to HTTP/TCP overhead.
# Multipart uploads use streaming for each part, keeping memory usage constant.
```

**Problem**: Timeout completing multipart upload of very large files (>10GB)

```bash
# Error: "Read timeout on endpoint URL"
# This happens during the final assembly step for large multipart uploads

# Solution: Increase the client timeout
# AWS CLI v2:
aws configure set s3.multipart_threshold 8MB
aws configure set s3.max_concurrent_requests 10
# Or set in environment:
export AWS_CLI_READ_TIMEOUT=300  # 5 minutes

# Python boto3:
from botocore.config import Config
config = Config(read_timeout=300)
s3 = boto3.client('s3', config=config, ...)

# Note: S3Dir optimizes assembly using:
# - 1MB buffer size for fast I/O
# - MD5-of-MD5s calculation (not full-file hash)
# - Typical assembly speed: ~500MB/sec on modern hardware
```

## Contributing

Contributions are welcome! Please see [DEVELOPMENT.md](DEVELOPMENT.md) for developer documentation and guidelines.

## License

MIT License - see LICENSE file for details

## Alternatives

- **MinIO**: Full-featured S3-compatible server with clustering and advanced features
- **LocalStack**: Complete AWS service emulation including S3
- **s3proxy**: S3 API proxy for various storage backends
- **fake-s3**: Ruby-based S3 simulator

S3Dir focuses on simplicity, performance, and ease of deployment for local development scenarios.
