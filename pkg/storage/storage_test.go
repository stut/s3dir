package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func setupTestStorage(t *testing.T) (*Storage, func()) {
	tmpDir, err := os.MkdirTemp("", "s3dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	storage, err := New(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create storage: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return storage, cleanup
}

func TestNew(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	if storage.baseDir == "" {
		t.Error("baseDir should not be empty")
	}
}

func TestCreateBucket(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	err := storage.CreateBucket("test-bucket")
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Check that bucket directory exists
	bucketPath := filepath.Join(storage.baseDir, "test-bucket")
	stat, err := os.Stat(bucketPath)
	if err != nil {
		t.Fatalf("Bucket directory not found: %v", err)
	}

	if !stat.IsDir() {
		t.Error("Bucket path is not a directory")
	}

	// Try to create the same bucket again
	err = storage.CreateBucket("test-bucket")
	if err == nil {
		t.Error("Expected error when creating existing bucket")
	}
}

func TestListBuckets(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create multiple buckets
	buckets := []string{"bucket1", "bucket2", "bucket3"}
	for _, bucket := range buckets {
		if err := storage.CreateBucket(bucket); err != nil {
			t.Fatalf("Failed to create bucket %s: %v", bucket, err)
		}
	}

	// List buckets
	listed, err := storage.ListBuckets()
	if err != nil {
		t.Fatalf("Failed to list buckets: %v", err)
	}

	if len(listed) != len(buckets) {
		t.Errorf("Expected %d buckets, got %d", len(buckets), len(listed))
	}

	// Check all buckets are present
	bucketMap := make(map[string]bool)
	for _, b := range listed {
		bucketMap[b] = true
	}

	for _, bucket := range buckets {
		if !bucketMap[bucket] {
			t.Errorf("Bucket %s not found in listing", bucket)
		}
	}
}

func TestHeadBucket(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create a bucket
	if err := storage.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Head existing bucket
	err := storage.HeadBucket("test-bucket")
	if err != nil {
		t.Errorf("HeadBucket failed for existing bucket: %v", err)
	}

	// Head non-existing bucket
	err = storage.HeadBucket("non-existing")
	if err == nil {
		t.Error("Expected error for non-existing bucket")
	}
}

func TestDeleteBucket(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create a bucket
	if err := storage.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Delete empty bucket
	err := storage.DeleteBucket("test-bucket")
	if err != nil {
		t.Errorf("Failed to delete empty bucket: %v", err)
	}

	// Verify bucket is deleted
	err = storage.HeadBucket("test-bucket")
	if err == nil {
		t.Error("Bucket should not exist after deletion")
	}

	// Create bucket with object
	if err := storage.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	data := bytes.NewReader([]byte("test data"))
	if err := storage.PutObject("test-bucket", "test.txt", data, int64(data.Len())); err != nil {
		t.Fatalf("Failed to put object: %v", err)
	}

	// Try to delete non-empty bucket
	err = storage.DeleteBucket("test-bucket")
	if err == nil {
		t.Error("Expected error when deleting non-empty bucket")
	}
}

func TestPutObject(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	if err := storage.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	testData := []byte("Hello, S3Dir!")
	data := bytes.NewReader(testData)

	err := storage.PutObject("test-bucket", "test.txt", data, int64(len(testData)))
	if err != nil {
		t.Fatalf("Failed to put object: %v", err)
	}

	// Verify file exists
	objectPath := filepath.Join(storage.baseDir, "test-bucket", "test.txt")
	stat, err := os.Stat(objectPath)
	if err != nil {
		t.Fatalf("Object file not found: %v", err)
	}

	if stat.Size() != int64(len(testData)) {
		t.Errorf("Expected size %d, got %d", len(testData), stat.Size())
	}
}

func TestPutObjectWithPath(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	if err := storage.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	testData := []byte("test data")
	data := bytes.NewReader(testData)

	// Put object with path
	err := storage.PutObject("test-bucket", "path/to/object.txt", data, int64(len(testData)))
	if err != nil {
		t.Fatalf("Failed to put object with path: %v", err)
	}

	// Verify directory structure
	objectPath := filepath.Join(storage.baseDir, "test-bucket", "path", "to", "object.txt")
	if _, err := os.Stat(objectPath); err != nil {
		t.Fatalf("Object file not found: %v", err)
	}
}

func TestGetObject(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	if err := storage.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	testData := []byte("Hello, S3Dir!")
	data := bytes.NewReader(testData)

	if err := storage.PutObject("test-bucket", "test.txt", data, int64(len(testData))); err != nil {
		t.Fatalf("Failed to put object: %v", err)
	}

	// Get the object
	reader, info, err := storage.GetObject("test-bucket", "test.txt")
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}
	defer reader.Close()

	// Verify info
	if info.Key != "test.txt" {
		t.Errorf("Expected key 'test.txt', got '%s'", info.Key)
	}

	if info.Size != int64(len(testData)) {
		t.Errorf("Expected size %d, got %d", len(testData), info.Size)
	}

	// Read and verify content
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read object: %v", err)
	}

	if !bytes.Equal(content, testData) {
		t.Errorf("Content mismatch: expected %s, got %s", testData, content)
	}
}

func TestGetNonExistingObject(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	if err := storage.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	_, _, err := storage.GetObject("test-bucket", "non-existing.txt")
	if err == nil {
		t.Error("Expected error for non-existing object")
	}
}

func TestHeadObject(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	if err := storage.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	testData := []byte("test data")
	data := bytes.NewReader(testData)

	if err := storage.PutObject("test-bucket", "test.txt", data, int64(len(testData))); err != nil {
		t.Fatalf("Failed to put object: %v", err)
	}

	// Head the object
	info, err := storage.HeadObject("test-bucket", "test.txt")
	if err != nil {
		t.Fatalf("Failed to head object: %v", err)
	}

	if info.Key != "test.txt" {
		t.Errorf("Expected key 'test.txt', got '%s'", info.Key)
	}

	if info.Size != int64(len(testData)) {
		t.Errorf("Expected size %d, got %d", len(testData), info.Size)
	}
}

func TestDeleteObject(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	if err := storage.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	testData := []byte("test data")
	data := bytes.NewReader(testData)

	if err := storage.PutObject("test-bucket", "test.txt", data, int64(len(testData))); err != nil {
		t.Fatalf("Failed to put object: %v", err)
	}

	// Delete the object
	err := storage.DeleteObject("test-bucket", "test.txt")
	if err != nil {
		t.Fatalf("Failed to delete object: %v", err)
	}

	// Verify object is deleted
	_, _, err = storage.GetObject("test-bucket", "test.txt")
	if err == nil {
		t.Error("Object should not exist after deletion")
	}

	// Deleting non-existing object should not error
	err = storage.DeleteObject("test-bucket", "non-existing.txt")
	if err != nil {
		t.Errorf("Delete non-existing object should not error: %v", err)
	}
}

func TestListObjects(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	if err := storage.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Create test objects
	objects := map[string]string{
		"file1.txt":      "data1",
		"file2.txt":      "data2",
		"dir/file3.txt":  "data3",
		"dir/file4.txt":  "data4",
		"dir2/file5.txt": "data5",
	}

	for key, content := range objects {
		data := bytes.NewReader([]byte(content))
		if err := storage.PutObject("test-bucket", key, data, int64(len(content))); err != nil {
			t.Fatalf("Failed to put object %s: %v", key, err)
		}
	}

	// List all objects
	listed, _, err := storage.ListObjects("test-bucket", "", "", 0)
	if err != nil {
		t.Fatalf("Failed to list objects: %v", err)
	}

	if len(listed) != len(objects) {
		t.Errorf("Expected %d objects, got %d", len(objects), len(listed))
	}

	// List with prefix
	listed, _, err = storage.ListObjects("test-bucket", "dir/", "", 0)
	if err != nil {
		t.Fatalf("Failed to list objects with prefix: %v", err)
	}

	if len(listed) != 2 {
		t.Errorf("Expected 2 objects with prefix 'dir/', got %d", len(listed))
	}

	// List with max keys
	listed, _, err = storage.ListObjects("test-bucket", "", "", 2)
	if err != nil {
		t.Fatalf("Failed to list objects with max keys: %v", err)
	}

	if len(listed) != 2 {
		t.Errorf("Expected 2 objects with max keys 2, got %d", len(listed))
	}
}

func TestListObjectsWithDelimiter(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	if err := storage.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Create test objects
	objects := map[string]string{
		"file1.txt":          "data1",
		"dir1/file2.txt":     "data2",
		"dir1/file3.txt":     "data3",
		"dir2/file4.txt":     "data4",
		"dir2/sub/file5.txt": "data5",
	}

	for key, content := range objects {
		data := bytes.NewReader([]byte(content))
		if err := storage.PutObject("test-bucket", key, data, int64(len(content))); err != nil {
			t.Fatalf("Failed to put object %s: %v", key, err)
		}
	}

	// List with delimiter
	listed, prefixes, err := storage.ListObjects("test-bucket", "", "/", 0)
	if err != nil {
		t.Fatalf("Failed to list objects with delimiter: %v", err)
	}

	// Should return file1.txt and common prefixes dir1/, dir2/
	if len(listed) != 1 {
		t.Errorf("Expected 1 object, got %d", len(listed))
	}

	if len(prefixes) != 2 {
		t.Errorf("Expected 2 common prefixes, got %d", len(prefixes))
	}
}

func TestListObjectsEmptyBucket(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	if err := storage.CreateBucket("test-bucket"); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// List empty bucket
	listed, _, err := storage.ListObjects("test-bucket", "", "", 0)
	if err != nil {
		t.Fatalf("Failed to list empty bucket: %v", err)
	}

	if len(listed) != 0 {
		t.Errorf("Expected 0 objects in empty bucket, got %d", len(listed))
	}
}

func TestListObjectsNonExistingBucket(t *testing.T) {
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	_, _, err := storage.ListObjects("non-existing", "", "", 0)
	if err == nil {
		t.Error("Expected error for non-existing bucket")
	}
}
