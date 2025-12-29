#!/usr/bin/env node

/**
 * Example Node.js client for S3Dir using AWS SDK v3.
 *
 * Installation:
 *   npm install @aws-sdk/client-s3
 *
 * Usage:
 *   node examples/nodejs-client.js
 */

const {
  S3Client,
  CreateBucketCommand,
  ListBucketsCommand,
  ListObjectsV2Command,
  PutObjectCommand,
  GetObjectCommand,
  HeadObjectCommand,
  DeleteObjectCommand,
  DeleteBucketCommand,
} = require('@aws-sdk/client-s3');

// Configure S3 client
const s3Client = new S3Client({
  endpoint: 'http://localhost:8000',
  region: 'us-east-1',
  credentials: {
    accessKeyId: 'dummy',
    secretAccessKey: 'dummy',
  },
  forcePathStyle: true,
});

const BUCKET_NAME = 'example-bucket';

// Utility to convert stream to string
async function streamToString(stream) {
  const chunks = [];
  return new Promise((resolve, reject) => {
    stream.on('data', (chunk) => chunks.push(Buffer.from(chunk)));
    stream.on('error', (err) => reject(err));
    stream.on('end', () => resolve(Buffer.concat(chunks).toString('utf-8')));
  });
}

async function main() {
  console.log('S3Dir Example - Node.js\n');

  try {
    // Create bucket
    console.log(`Creating bucket '${BUCKET_NAME}'...`);
    try {
      await s3Client.send(new CreateBucketCommand({ Bucket: BUCKET_NAME }));
      console.log('✓ Bucket created\n');
    } catch (err) {
      if (err.name === 'BucketAlreadyExists') {
        console.log('✓ Bucket already exists\n');
      } else {
        throw err;
      }
    }

    // Upload object
    console.log('Uploading object...');
    await s3Client.send(
      new PutObjectCommand({
        Bucket: BUCKET_NAME,
        Key: 'hello.txt',
        Body: 'Hello from Node.js!',
      })
    );
    console.log('✓ Object uploaded\n');

    // List buckets
    console.log('Listing buckets...');
    const bucketsResult = await s3Client.send(new ListBucketsCommand({}));
    bucketsResult.Buckets.forEach((bucket) => {
      console.log(`  - ${bucket.Name}`);
    });
    console.log();

    // Upload multiple objects
    console.log('Uploading multiple objects...');
    const objects = {
      'dir/file1.txt': 'Content 1',
      'dir/file2.txt': 'Content 2',
      'other.txt': 'Other content',
    };

    for (const [key, content] of Object.entries(objects)) {
      await s3Client.send(
        new PutObjectCommand({
          Bucket: BUCKET_NAME,
          Key: key,
          Body: content,
        })
      );
      console.log(`  ✓ Uploaded ${key}`);
    }
    console.log();

    // List objects
    console.log('Listing objects...');
    const objectsResult = await s3Client.send(
      new ListObjectsV2Command({ Bucket: BUCKET_NAME })
    );
    objectsResult.Contents.forEach((obj) => {
      console.log(`  - ${obj.Key} (${obj.Size} bytes)`);
    });
    console.log();

    // List with prefix
    console.log("Listing objects with prefix 'dir/'...");
    const prefixResult = await s3Client.send(
      new ListObjectsV2Command({
        Bucket: BUCKET_NAME,
        Prefix: 'dir/',
      })
    );
    prefixResult.Contents.forEach((obj) => {
      console.log(`  - ${obj.Key}`);
    });
    console.log();

    // List with delimiter
    console.log("Listing with delimiter '/'...");
    const delimiterResult = await s3Client.send(
      new ListObjectsV2Command({
        Bucket: BUCKET_NAME,
        Delimiter: '/',
      })
    );
    console.log('  Objects:');
    delimiterResult.Contents?.forEach((obj) => {
      console.log(`    - ${obj.Key}`);
    });
    console.log('  Common Prefixes:');
    delimiterResult.CommonPrefixes?.forEach((prefix) => {
      console.log(`    - ${prefix.Prefix}`);
    });
    console.log();

    // Download object
    console.log('Downloading object...');
    const getResult = await s3Client.send(
      new GetObjectCommand({
        Bucket: BUCKET_NAME,
        Key: 'hello.txt',
      })
    );
    const content = await streamToString(getResult.Body);
    console.log(`✓ Content: ${content}\n`);

    // Head object
    console.log('Getting object metadata...');
    const headResult = await s3Client.send(
      new HeadObjectCommand({
        Bucket: BUCKET_NAME,
        Key: 'hello.txt',
      })
    );
    console.log(`  - Size: ${headResult.ContentLength} bytes`);
    console.log(`  - ETag: ${headResult.ETag}`);
    console.log(`  - Last Modified: ${headResult.LastModified}`);
    console.log();

    // Clean up
    console.log('Cleaning up...');

    // Delete all objects
    const listForCleanup = await s3Client.send(
      new ListObjectsV2Command({ Bucket: BUCKET_NAME })
    );

    for (const obj of listForCleanup.Contents) {
      await s3Client.send(
        new DeleteObjectCommand({
          Bucket: BUCKET_NAME,
          Key: obj.Key,
        })
      );
      console.log(`  ✓ Deleted ${obj.Key}`);
    }

    // Delete bucket
    await s3Client.send(new DeleteBucketCommand({ Bucket: BUCKET_NAME }));
    console.log(`✓ Bucket '${BUCKET_NAME}' deleted\n`);

    console.log('✅ All operations completed successfully!');
  } catch (error) {
    console.error('Error:', error.message);
    process.exit(1);
  }
}

// Run the example
main();
