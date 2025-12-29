#!/usr/bin/env python3
"""
Example Python client for S3Dir using boto3.

Usage:
    pip install boto3
    python examples/python-client.py
"""

import boto3
from botocore.exceptions import ClientError

# Configure S3 client for S3Dir
s3 = boto3.client(
    's3',
    endpoint_url='http://localhost:8000',
    aws_access_key_id='dummy',
    aws_secret_access_key='dummy',
    region_name='us-east-1'
)

def main():
    bucket_name = 'example-bucket'

    # Create bucket
    print(f"Creating bucket '{bucket_name}'...")
    try:
        s3.create_bucket(Bucket=bucket_name)
        print("✓ Bucket created")
    except ClientError as e:
        if e.response['Error']['Code'] == 'BucketAlreadyExists':
            print("✓ Bucket already exists")
        else:
            raise

    # Upload a file
    print("\nUploading object...")
    s3.put_object(
        Bucket=bucket_name,
        Key='hello.txt',
        Body=b'Hello from Python!'
    )
    print("✓ Object uploaded")

    # List objects
    print("\nListing objects...")
    response = s3.list_objects_v2(Bucket=bucket_name)
    for obj in response.get('Contents', []):
        print(f"  - {obj['Key']} ({obj['Size']} bytes)")

    # Download object
    print("\nDownloading object...")
    response = s3.get_object(Bucket=bucket_name, Key='hello.txt')
    content = response['Body'].read().decode('utf-8')
    print(f"✓ Content: {content}")

    # Upload with metadata
    print("\nUploading object with metadata...")
    s3.put_object(
        Bucket=bucket_name,
        Key='data.json',
        Body=b'{"message": "Hello, World!"}',
        ContentType='application/json'
    )
    print("✓ Object with metadata uploaded")

    # Upload from file
    print("\nUploading file...")
    with open(__file__, 'rb') as f:
        s3.upload_fileobj(f, bucket_name, 'script.py')
    print("✓ File uploaded")

    # List with prefix
    print("\nListing objects with prefix...")
    response = s3.list_objects_v2(Bucket=bucket_name, Prefix='data')
    for obj in response.get('Contents', []):
        print(f"  - {obj['Key']}")

    # Generate presigned URL (note: won't work without proper auth)
    print("\nGenerating presigned URL...")
    try:
        url = s3.generate_presigned_url(
            'get_object',
            Params={'Bucket': bucket_name, 'Key': 'hello.txt'},
            ExpiresIn=3600
        )
        print(f"✓ URL: {url}")
    except Exception as e:
        print(f"⚠ Presigned URLs require authentication: {e}")

    # Clean up
    print("\nCleaning up...")

    # Delete objects
    response = s3.list_objects_v2(Bucket=bucket_name)
    for obj in response.get('Contents', []):
        s3.delete_object(Bucket=bucket_name, Key=obj['Key'])
        print(f"  ✓ Deleted {obj['Key']}")

    # Delete bucket
    s3.delete_bucket(Bucket=bucket_name)
    print(f"✓ Bucket '{bucket_name}' deleted")

    print("\n✅ All operations completed successfully!")

if __name__ == '__main__':
    main()
