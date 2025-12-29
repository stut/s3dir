# S3Dir Examples

This directory contains example client implementations for interacting with S3Dir using various programming languages and tools.

## Prerequisites

1. **Start S3Dir server**:
   ```bash
   cd ..
   go run ./cmd/s3dir
   ```
   
   Or with custom configuration:
   ```bash
   S3DIR_PORT=8000 S3DIR_VERBOSE=true go run ./cmd/s3dir
   ```

2. **Ensure the server is running** on `http://localhost:8000`

## Examples

### Bash (AWS CLI)

Uses the AWS CLI to interact with S3Dir.

**Prerequisites**:
```bash
# Install AWS CLI
# macOS: brew install awscli
# Ubuntu: apt install awscli
# Or: pip install awscli
```

**Run**:
```bash
chmod +x bash-client.sh
./bash-client.sh
```

**What it demonstrates**:
- Creating buckets
- Uploading files
- Listing objects
- Directory sync
- Object metadata
- Cleanup

---

### Python (boto3)

Uses the boto3 library to interact with S3Dir.

**Prerequisites**:
```bash
pip install boto3
```

**Run**:
```bash
python python-client.py
```

**What it demonstrates**:
- Bucket operations
- Object upload/download
- Listing with prefix
- Metadata handling
- Error handling

---

### Go (AWS SDK)

Uses the AWS SDK for Go.

**Prerequisites**:
```bash
go get github.com/aws/aws-sdk-go/aws
go get github.com/aws/aws-sdk-go/service/s3
```

**Run**:
```bash
go run go-client.go
```

**What it demonstrates**:
- Full S3 API usage from Go
- Bucket and object operations
- Prefix and delimiter filtering
- Metadata access
- Cleanup operations

---

### Node.js (AWS SDK v3)

Uses the AWS SDK for JavaScript v3.

**Prerequisites**:
```bash
npm install @aws-sdk/client-s3
```

**Run**:
```bash
node nodejs-client.js
```

**What it demonstrates**:
- Modern AWS SDK v3 usage
- Async/await patterns
- Stream handling
- Object operations
- Error handling

---

## Common Operations

### Create a Bucket

**AWS CLI**:
```bash
aws --endpoint-url=http://localhost:8000 s3 mb s3://my-bucket
```

**Python**:
```python
s3.create_bucket(Bucket='my-bucket')
```

**Go**:
```go
svc.CreateBucket(&s3.CreateBucketInput{
    Bucket: aws.String("my-bucket"),
})
```

**Node.js**:
```javascript
await s3Client.send(new CreateBucketCommand({ Bucket: 'my-bucket' }));
```

### Upload a File

**AWS CLI**:
```bash
aws --endpoint-url=http://localhost:8000 s3 cp file.txt s3://my-bucket/
```

**Python**:
```python
s3.upload_file('file.txt', 'my-bucket', 'file.txt')
```

**Go**:
```go
svc.PutObject(&s3.PutObjectInput{
    Bucket: aws.String("my-bucket"),
    Key:    aws.String("file.txt"),
    Body:   bytes.NewReader(data),
})
```

**Node.js**:
```javascript
await s3Client.send(new PutObjectCommand({
    Bucket: 'my-bucket',
    Key: 'file.txt',
    Body: data,
}));
```

### List Objects

**AWS CLI**:
```bash
aws --endpoint-url=http://localhost:8000 s3 ls s3://my-bucket/
```

**Python**:
```python
response = s3.list_objects_v2(Bucket='my-bucket')
for obj in response['Contents']:
    print(obj['Key'])
```

**Go**:
```go
result, _ := svc.ListObjectsV2(&s3.ListObjectsV2Input{
    Bucket: aws.String("my-bucket"),
})
for _, obj := range result.Contents {
    fmt.Println(*obj.Key)
}
```

**Node.js**:
```javascript
const result = await s3Client.send(new ListObjectsV2Command({
    Bucket: 'my-bucket',
}));
result.Contents.forEach(obj => console.log(obj.Key));
```

## Testing with Authentication

If S3Dir is running with authentication enabled:

```bash
# Start S3Dir with auth
S3DIR_ENABLE_AUTH=true \
S3DIR_ACCESS_KEY_ID=mykey \
S3DIR_SECRET_ACCESS_KEY=mysecret \
./s3dir
```

Update your client configuration:

**AWS CLI**:
```bash
aws configure set aws_access_key_id mykey
aws configure set aws_secret_access_key mysecret
```

**Python**:
```python
s3 = boto3.client(
    's3',
    endpoint_url='http://localhost:8000',
    aws_access_key_id='mykey',
    aws_secret_access_key='mysecret',
)
```

**Go**:
```go
credentials.NewStaticCredentials("mykey", "mysecret", "")
```

**Node.js**:
```javascript
credentials: {
    accessKeyId: 'mykey',
    secretAccessKey: 'mysecret',
}
```

## Troubleshooting

### Connection Refused

**Problem**: `Connection refused` error

**Solution**: Ensure S3Dir is running:
```bash
# In another terminal
cd ..
./s3dir
```

### Authentication Errors

**Problem**: `Access Denied` or `403 Forbidden`

**Solution**: 
1. Check if auth is enabled on the server
2. Verify credentials match server configuration
3. For testing, disable auth: `S3DIR_ENABLE_AUTH=false ./s3dir`

### Bucket Not Found

**Problem**: `NoSuchBucket` error

**Solution**: Create the bucket first:
```bash
aws --endpoint-url=http://localhost:8000 s3 mb s3://my-bucket
```

## Additional Resources

- [AWS CLI S3 Documentation](https://docs.aws.amazon.com/cli/latest/reference/s3/)
- [boto3 Documentation](https://boto3.amazonaws.com/v1/documentation/api/latest/index.html)
- [AWS SDK for Go](https://aws.github.io/aws-sdk-go-v2/docs/)
- [AWS SDK for JavaScript v3](https://docs.aws.amazon.com/AWSJavaScriptSDK/v3/latest/)

## Contributing

Feel free to add more examples for other languages or use cases!
