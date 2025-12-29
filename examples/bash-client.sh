#!/bin/bash

###############################################################################
# Example bash script using AWS CLI to interact with S3Dir
#
# Prerequisites:
#   - AWS CLI installed (aws --version)
#   - S3Dir running on localhost:8000
#
# Usage:
#   chmod +x examples/bash-client.sh
#   ./examples/bash-client.sh
###############################################################################

set -e  # Exit on error

# Configuration
ENDPOINT="http://localhost:8000"
BUCKET="example-bucket"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}S3Dir Example - Bash/AWS CLI${NC}\n"

# Configure AWS CLI with dummy credentials
echo "Configuring AWS CLI..."
aws configure set aws_access_key_id dummy
aws configure set aws_secret_access_key dummy
echo -e "${GREEN}✓${NC} AWS CLI configured\n"

# Create bucket
echo "Creating bucket '$BUCKET'..."
aws --endpoint-url=$ENDPOINT s3 mb s3://$BUCKET 2>/dev/null || echo "Bucket already exists"
echo -e "${GREEN}✓${NC} Bucket ready\n"

# Upload a file
echo "Uploading objects..."
echo "Hello from Bash!" > /tmp/hello.txt
aws --endpoint-url=$ENDPOINT s3 cp /tmp/hello.txt s3://$BUCKET/
echo -e "${GREEN}✓${NC} Object uploaded\n"

# Create directory structure
echo "Creating directory structure..."
echo "File 1" > /tmp/file1.txt
echo "File 2" > /tmp/file2.txt
echo "File 3" > /tmp/file3.txt

aws --endpoint-url=$ENDPOINT s3 cp /tmp/file1.txt s3://$BUCKET/dir1/file1.txt
aws --endpoint-url=$ENDPOINT s3 cp /tmp/file2.txt s3://$BUCKET/dir1/file2.txt
aws --endpoint-url=$ENDPOINT s3 cp /tmp/file3.txt s3://$BUCKET/dir2/file3.txt
echo -e "${GREEN}✓${NC} Directory structure created\n"

# List buckets
echo "Listing buckets..."
aws --endpoint-url=$ENDPOINT s3 ls
echo ""

# List objects
echo "Listing all objects in bucket..."
aws --endpoint-url=$ENDPOINT s3 ls s3://$BUCKET --recursive
echo ""

# List objects with prefix
echo "Listing objects in 'dir1/'..."
aws --endpoint-url=$ENDPOINT s3 ls s3://$BUCKET/dir1/
echo ""

# Download object
echo "Downloading object..."
aws --endpoint-url=$ENDPOINT s3 cp s3://$BUCKET/hello.txt /tmp/downloaded.txt
cat /tmp/downloaded.txt
echo -e "\n${GREEN}✓${NC} Object downloaded\n"

# Sync directory
echo "Syncing local directory to S3..."
mkdir -p /tmp/sync-example
echo "Synced file 1" > /tmp/sync-example/sync1.txt
echo "Synced file 2" > /tmp/sync-example/sync2.txt
aws --endpoint-url=$ENDPOINT s3 sync /tmp/sync-example s3://$BUCKET/synced/
echo -e "${GREEN}✓${NC} Directory synced\n"

# Get object metadata
echo "Getting object metadata..."
aws --endpoint-url=$ENDPOINT s3api head-object \
    --bucket $BUCKET \
    --key hello.txt
echo ""

# Copy object
echo "Copying object..."
aws --endpoint-url=$ENDPOINT s3 cp \
    s3://$BUCKET/hello.txt \
    s3://$BUCKET/hello-copy.txt
echo -e "${GREEN}✓${NC} Object copied\n"

# List objects with human-readable sizes
echo "Listing objects with details..."
aws --endpoint-url=$ENDPOINT s3 ls s3://$BUCKET --recursive --human-readable --summarize
echo ""

# Clean up
echo "Cleaning up..."
aws --endpoint-url=$ENDPOINT s3 rm s3://$BUCKET --recursive
echo -e "${GREEN}✓${NC} All objects deleted"

aws --endpoint-url=$ENDPOINT s3 rb s3://$BUCKET
echo -e "${GREEN}✓${NC} Bucket deleted"

# Clean up temp files
rm -f /tmp/hello.txt /tmp/downloaded.txt /tmp/file*.txt
rm -rf /tmp/sync-example

echo -e "\n${GREEN}✅ All operations completed successfully!${NC}"
