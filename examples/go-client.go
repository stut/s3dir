package main

/*
Example Go client for S3Dir using AWS SDK for Go.

Usage:
    go run examples/go-client.go
*/

import (
	"bytes"
	"fmt"
	"io"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func main() {
	// Create S3 client
	sess := session.Must(session.NewSession(&aws.Config{
		Endpoint:         aws.String("http://localhost:8000"),
		Region:           aws.String("us-east-1"),
		Credentials:      credentials.NewStaticCredentials("dummy", "dummy", ""),
		S3ForcePathStyle: aws.Bool(true),
	}))

	svc := s3.New(sess)
	bucketName := "example-bucket"

	// Create bucket
	fmt.Println("Creating bucket...")
	_, err := svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.Printf("Bucket creation warning: %v\n", err)
	} else {
		fmt.Println("✓ Bucket created")
	}

	// Upload object
	fmt.Println("\nUploading object...")
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("hello.txt"),
		Body:   bytes.NewReader([]byte("Hello from Go!")),
	})
	if err != nil {
		log.Fatalf("Upload failed: %v", err)
	}
	fmt.Println("✓ Object uploaded")

	// List buckets
	fmt.Println("\nListing buckets...")
	bucketsResult, err := svc.ListBuckets(nil)
	if err != nil {
		log.Fatalf("List buckets failed: %v", err)
	}
	for _, b := range bucketsResult.Buckets {
		fmt.Printf("  - %s (created: %s)\n", *b.Name, b.CreationDate)
	}

	// List objects
	fmt.Println("\nListing objects...")
	objectsResult, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.Fatalf("List objects failed: %v", err)
	}
	for _, obj := range objectsResult.Contents {
		fmt.Printf("  - %s (%d bytes)\n", *obj.Key, *obj.Size)
	}

	// Download object
	fmt.Println("\nDownloading object...")
	getResult, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("hello.txt"),
	})
	if err != nil {
		log.Fatalf("Download failed: %v", err)
	}
	defer getResult.Body.Close()

	content, err := io.ReadAll(getResult.Body)
	if err != nil {
		log.Fatalf("Read failed: %v", err)
	}
	fmt.Printf("✓ Content: %s\n", string(content))

	// Head object
	fmt.Println("\nGetting object metadata...")
	headResult, err := svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("hello.txt"),
	})
	if err != nil {
		log.Fatalf("Head failed: %v", err)
	}
	fmt.Printf("  - Size: %d bytes\n", *headResult.ContentLength)
	fmt.Printf("  - ETag: %s\n", *headResult.ETag)
	fmt.Printf("  - Last Modified: %s\n", headResult.LastModified)

	// Upload multiple objects
	fmt.Println("\nUploading multiple objects...")
	objects := map[string]string{
		"dir/file1.txt": "Content 1",
		"dir/file2.txt": "Content 2",
		"other.txt":     "Other content",
	}
	for key, content := range objects {
		_, err := svc.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
			Body:   bytes.NewReader([]byte(content)),
		})
		if err != nil {
			log.Fatalf("Upload %s failed: %v", key, err)
		}
		fmt.Printf("  ✓ Uploaded %s\n", key)
	}

	// List with prefix
	fmt.Println("\nListing objects with prefix 'dir/'...")
	prefixResult, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String("dir/"),
	})
	if err != nil {
		log.Fatalf("List with prefix failed: %v", err)
	}
	for _, obj := range prefixResult.Contents {
		fmt.Printf("  - %s\n", *obj.Key)
	}

	// List with delimiter
	fmt.Println("\nListing with delimiter '/'...")
	delimiterResult, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(bucketName),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		log.Fatalf("List with delimiter failed: %v", err)
	}
	fmt.Println("  Objects:")
	for _, obj := range delimiterResult.Contents {
		fmt.Printf("    - %s\n", *obj.Key)
	}
	fmt.Println("  Common Prefixes:")
	for _, prefix := range delimiterResult.CommonPrefixes {
		fmt.Printf("    - %s\n", *prefix.Prefix)
	}

	// Clean up
	fmt.Println("\nCleaning up...")

	// Delete all objects
	listResult, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.Fatalf("List for cleanup failed: %v", err)
	}

	for _, obj := range listResult.Contents {
		_, err := svc.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    obj.Key,
		})
		if err != nil {
			log.Printf("Delete %s failed: %v", *obj.Key, err)
		} else {
			fmt.Printf("  ✓ Deleted %s\n", *obj.Key)
		}
	}

	// Delete bucket
	_, err = svc.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.Fatalf("Delete bucket failed: %v", err)
	}
	fmt.Printf("✓ Bucket '%s' deleted\n", bucketName)

	fmt.Println("\n✅ All operations completed successfully!")
}
