package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMultipartUploadWorkflow(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "s3dir-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create storage
	storage, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create bucket
	bucket := "test-bucket"
	if err := storage.CreateBucket(bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Initiate multipart upload
	key := "test-object.txt"
	uploadID, err := storage.InitiateMultipartUpload(bucket, key)
	if err != nil {
		t.Fatalf("Failed to initiate multipart upload: %v", err)
	}

	if uploadID == "" {
		t.Fatal("Upload ID should not be empty")
	}

	// Upload parts
	part1Data := []byte("part1-content-")
	part2Data := []byte("part2-content-")
	part3Data := []byte("part3-content")

	etag1, err := storage.UploadPart(uploadID, 1, bytes.NewReader(part1Data), int64(len(part1Data)))
	if err != nil {
		t.Fatalf("Failed to upload part 1: %v", err)
	}
	if etag1 == "" {
		t.Fatal("Part 1 ETag should not be empty")
	}

	etag2, err := storage.UploadPart(uploadID, 2, bytes.NewReader(part2Data), int64(len(part2Data)))
	if err != nil {
		t.Fatalf("Failed to upload part 2: %v", err)
	}
	if etag2 == "" {
		t.Fatal("Part 2 ETag should not be empty")
	}

	etag3, err := storage.UploadPart(uploadID, 3, bytes.NewReader(part3Data), int64(len(part3Data)))
	if err != nil {
		t.Fatalf("Failed to upload part 3: %v", err)
	}
	if etag3 == "" {
		t.Fatal("Part 3 ETag should not be empty")
	}

	// List parts
	parts, err := storage.ListMultipartUploadParts(uploadID)
	if err != nil {
		t.Fatalf("Failed to list parts: %v", err)
	}
	if len(parts) != 3 {
		t.Fatalf("Expected 3 parts, got %d", len(parts))
	}

	// Verify part data
	for _, part := range parts {
		if part.PartNumber < 1 || part.PartNumber > 3 {
			t.Errorf("Invalid part number: %d", part.PartNumber)
		}
		if part.ETag == "" {
			t.Errorf("Part %d has empty ETag", part.PartNumber)
		}
		if part.Size == 0 {
			t.Errorf("Part %d has zero size", part.PartNumber)
		}
		if part.LastModified.IsZero() {
			t.Errorf("Part %d has zero LastModified", part.PartNumber)
		}
	}

	// Complete multipart upload
	completeParts := []CompletePart{
		{PartNumber: 1, ETag: etag1},
		{PartNumber: 2, ETag: etag2},
		{PartNumber: 3, ETag: etag3},
	}

	finalETag, err := storage.CompleteMultipartUpload(uploadID, completeParts)
	if err != nil {
		t.Fatalf("Failed to complete multipart upload: %v", err)
	}
	if finalETag == "" {
		t.Fatal("Final ETag should not be empty")
	}

	// Verify object was created
	info, err := storage.HeadObject(bucket, key)
	if err != nil {
		t.Fatalf("Failed to get object info: %v", err)
	}

	expectedSize := int64(len(part1Data) + len(part2Data) + len(part3Data))
	if info.Size != expectedSize {
		t.Errorf("Expected object size %d, got %d", expectedSize, info.Size)
	}

	// Read object and verify content
	reader, _, err := storage.GetObject(bucket, key)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}
	defer reader.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		t.Fatalf("Failed to read object: %v", err)
	}

	expectedContent := string(part1Data) + string(part2Data) + string(part3Data)
	if buf.String() != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, buf.String())
	}

	// Verify multipart directory was cleaned up
	multipartDir := filepath.Join(tmpDir, ".multipart", uploadID)
	if _, err := os.Stat(multipartDir); !os.IsNotExist(err) {
		t.Error("Multipart directory should be cleaned up after completion")
	}
}

func TestAbortMultipartUpload(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	bucket := "test-bucket"
	if err := storage.CreateBucket(bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Initiate upload
	uploadID, err := storage.InitiateMultipartUpload(bucket, "test.txt")
	if err != nil {
		t.Fatalf("Failed to initiate upload: %v", err)
	}

	// Upload a part
	data := []byte("test data")
	if _, err := storage.UploadPart(uploadID, 1, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("Failed to upload part: %v", err)
	}

	// Abort upload
	if err := storage.AbortMultipartUpload(uploadID); err != nil {
		t.Fatalf("Failed to abort upload: %v", err)
	}

	// Verify upload is gone
	if _, err := storage.ListMultipartUploadParts(uploadID); err == nil {
		t.Error("Expected error when listing parts of aborted upload")
	}

	// Verify parts directory was cleaned up
	multipartDir := filepath.Join(tmpDir, ".multipart", uploadID)
	if _, err := os.Stat(multipartDir); !os.IsNotExist(err) {
		t.Error("Multipart directory should be cleaned up after abort")
	}
}

func TestMultipartUploadErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	bucket := "test-bucket"
	if err := storage.CreateBucket(bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	t.Run("upload to non-existing bucket", func(t *testing.T) {
		_, err := storage.InitiateMultipartUpload("non-existing", "test.txt")
		if err == nil {
			t.Error("Expected error when initiating upload to non-existing bucket")
		}
	})

	t.Run("upload part to non-existing upload", func(t *testing.T) {
		data := []byte("test")
		_, err := storage.UploadPart("non-existing-upload-id", 1, bytes.NewReader(data), int64(len(data)))
		if err == nil {
			t.Error("Expected error when uploading part to non-existing upload")
		}
	})

	t.Run("complete non-existing upload", func(t *testing.T) {
		parts := []CompletePart{{PartNumber: 1, ETag: "test"}}
		_, err := storage.CompleteMultipartUpload("non-existing-upload-id", parts)
		if err == nil {
			t.Error("Expected error when completing non-existing upload")
		}
	})

	t.Run("complete with missing parts", func(t *testing.T) {
		uploadID, err := storage.InitiateMultipartUpload(bucket, "test.txt")
		if err != nil {
			t.Fatalf("Failed to initiate upload: %v", err)
		}

		// Upload only part 1
		data := []byte("test")
		etag, err := storage.UploadPart(uploadID, 1, bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("Failed to upload part: %v", err)
		}

		// Try to complete with part 2 that doesn't exist
		parts := []CompletePart{
			{PartNumber: 1, ETag: etag},
			{PartNumber: 2, ETag: "invalid"},
		}
		_, err = storage.CompleteMultipartUpload(uploadID, parts)
		if err == nil {
			t.Error("Expected error when completing with missing part")
		}
		if !strings.Contains(err.Error(), "part") {
			t.Errorf("Expected error message to contain 'part', got: %v", err)
		}
	})

	t.Run("complete with wrong ETag", func(t *testing.T) {
		uploadID, err := storage.InitiateMultipartUpload(bucket, "test2.txt")
		if err != nil {
			t.Fatalf("Failed to initiate upload: %v", err)
		}

		data := []byte("test")
		_, err = storage.UploadPart(uploadID, 1, bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("Failed to upload part: %v", err)
		}

		// Try to complete with wrong ETag
		parts := []CompletePart{{PartNumber: 1, ETag: "wrong-etag"}}
		_, err = storage.CompleteMultipartUpload(uploadID, parts)
		if err == nil {
			t.Error("Expected error when completing with wrong ETag")
		}
	})

	t.Run("abort non-existing upload", func(t *testing.T) {
		err := storage.AbortMultipartUpload("non-existing-upload-id")
		if err == nil {
			t.Error("Expected error when aborting non-existing upload")
		}
	})

	t.Run("list parts of non-existing upload", func(t *testing.T) {
		_, err := storage.ListMultipartUploadParts("non-existing-upload-id")
		if err == nil {
			t.Error("Expected error when listing parts of non-existing upload")
		}
	})
}

func TestListMultipartUploads(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	bucket := "test-bucket"
	if err := storage.CreateBucket(bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Initially, no uploads
	uploads := storage.ListMultipartUploads(bucket)
	if len(uploads) != 0 {
		t.Errorf("Expected 0 uploads, got %d", len(uploads))
	}

	// Create multiple uploads
	upload1, err := storage.InitiateMultipartUpload(bucket, "file1.txt")
	if err != nil {
		t.Fatalf("Failed to initiate upload 1: %v", err)
	}

	upload2, err := storage.InitiateMultipartUpload(bucket, "file2.txt")
	if err != nil {
		t.Fatalf("Failed to initiate upload 2: %v", err)
	}

	upload3, err := storage.InitiateMultipartUpload(bucket, "dir/file3.txt")
	if err != nil {
		t.Fatalf("Failed to initiate upload 3: %v", err)
	}

	// List uploads
	uploads = storage.ListMultipartUploads(bucket)
	if len(uploads) != 3 {
		t.Errorf("Expected 3 uploads, got %d", len(uploads))
	}

	// Verify upload details
	foundUploads := make(map[string]bool)
	for _, upload := range uploads {
		if upload.Bucket != bucket {
			t.Errorf("Expected bucket %q, got %q", bucket, upload.Bucket)
		}
		if upload.UploadID == "" {
			t.Error("Upload ID should not be empty")
		}
		if upload.Initiated.IsZero() {
			t.Error("Initiated time should not be zero")
		}
		foundUploads[upload.UploadID] = true
	}

	if !foundUploads[upload1] || !foundUploads[upload2] || !foundUploads[upload3] {
		t.Error("Not all uploads were listed")
	}

	// Complete one upload
	data := []byte("test")
	etag, err := storage.UploadPart(upload1, 1, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Failed to upload part: %v", err)
	}

	parts := []CompletePart{{PartNumber: 1, ETag: etag}}
	if _, err := storage.CompleteMultipartUpload(upload1, parts); err != nil {
		t.Fatalf("Failed to complete upload: %v", err)
	}

	// List should now show 2 uploads
	uploads = storage.ListMultipartUploads(bucket)
	if len(uploads) != 2 {
		t.Errorf("Expected 2 uploads after completion, got %d", len(uploads))
	}

	// Abort one upload
	if err := storage.AbortMultipartUpload(upload2); err != nil {
		t.Fatalf("Failed to abort upload: %v", err)
	}

	// List should now show 1 upload
	uploads = storage.ListMultipartUploads(bucket)
	if len(uploads) != 1 {
		t.Errorf("Expected 1 upload after abort, got %d", len(uploads))
	}
	if uploads[0].UploadID != upload3 {
		t.Errorf("Expected remaining upload to be %q, got %q", upload3, uploads[0].UploadID)
	}
}

func TestMultipartUploadOutOfOrder(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	bucket := "test-bucket"
	if err := storage.CreateBucket(bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	uploadID, err := storage.InitiateMultipartUpload(bucket, "test.txt")
	if err != nil {
		t.Fatalf("Failed to initiate upload: %v", err)
	}

	// Upload parts out of order
	part3Data := []byte("333")
	part1Data := []byte("111")
	part2Data := []byte("222")

	etag3, err := storage.UploadPart(uploadID, 3, bytes.NewReader(part3Data), int64(len(part3Data)))
	if err != nil {
		t.Fatalf("Failed to upload part 3: %v", err)
	}

	etag1, err := storage.UploadPart(uploadID, 1, bytes.NewReader(part1Data), int64(len(part1Data)))
	if err != nil {
		t.Fatalf("Failed to upload part 1: %v", err)
	}

	etag2, err := storage.UploadPart(uploadID, 2, bytes.NewReader(part2Data), int64(len(part2Data)))
	if err != nil {
		t.Fatalf("Failed to upload part 2: %v", err)
	}

	// Complete with parts in correct order
	parts := []CompletePart{
		{PartNumber: 1, ETag: etag1},
		{PartNumber: 2, ETag: etag2},
		{PartNumber: 3, ETag: etag3},
	}

	if _, err := storage.CompleteMultipartUpload(uploadID, parts); err != nil {
		t.Fatalf("Failed to complete upload: %v", err)
	}

	// Verify content is in correct order
	reader, _, err := storage.GetObject(bucket, "test.txt")
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}
	defer reader.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		t.Fatalf("Failed to read object: %v", err)
	}

	expectedContent := "111222333"
	if buf.String() != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, buf.String())
	}
}

func TestMultipartUploadReplaceExistingPart(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	bucket := "test-bucket"
	if err := storage.CreateBucket(bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	uploadID, err := storage.InitiateMultipartUpload(bucket, "test.txt")
	if err != nil {
		t.Fatalf("Failed to initiate upload: %v", err)
	}

	// Upload part 1
	oldData := []byte("old content")
	_, err = storage.UploadPart(uploadID, 1, bytes.NewReader(oldData), int64(len(oldData)))
	if err != nil {
		t.Fatalf("Failed to upload part 1 first time: %v", err)
	}

	// Replace part 1
	newData := []byte("new content")
	newEtag, err := storage.UploadPart(uploadID, 1, bytes.NewReader(newData), int64(len(newData)))
	if err != nil {
		t.Fatalf("Failed to upload part 1 second time: %v", err)
	}

	// Complete upload
	parts := []CompletePart{{PartNumber: 1, ETag: newEtag}}
	if _, err := storage.CompleteMultipartUpload(uploadID, parts); err != nil {
		t.Fatalf("Failed to complete upload: %v", err)
	}

	// Verify new content
	reader, _, err := storage.GetObject(bucket, "test.txt")
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}
	defer reader.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		t.Fatalf("Failed to read object: %v", err)
	}

	if buf.String() != string(newData) {
		t.Errorf("Expected content %q, got %q", newData, buf.String())
	}
}

func TestCleanupStaleUploads(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.multipart.Stop()

	bucket := "test-bucket"
	if err := storage.CreateBucket(bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Create an upload
	uploadID, err := storage.InitiateMultipartUpload(bucket, "test.txt")
	if err != nil {
		t.Fatalf("Failed to initiate upload: %v", err)
	}

	// Upload a part
	data := []byte("test data")
	if _, err := storage.UploadPart(uploadID, 1, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("Failed to upload part: %v", err)
	}

	// Verify upload exists
	uploads := storage.ListMultipartUploads(bucket)
	if len(uploads) != 1 {
		t.Fatalf("Expected 1 upload, got %d", len(uploads))
	}

	// Manually set LastActivity to 25 hours ago (past threshold)
	storage.multipart.mu.Lock()
	upload := storage.multipart.uploads[uploadID]
	upload.mu.Lock()
	upload.LastActivity = time.Now().Add(-25 * time.Hour)
	upload.mu.Unlock()
	storage.multipart.mu.Unlock()

	// Run cleanup
	storage.multipart.cleanupStaleUploads()

	// Verify upload was removed
	uploads = storage.ListMultipartUploads(bucket)
	if len(uploads) != 0 {
		t.Errorf("Expected 0 uploads after cleanup, got %d", len(uploads))
	}

	// Verify parts directory was removed
	partsDir := filepath.Join(tmpDir, ".multipart", uploadID)
	if _, err := os.Stat(partsDir); !os.IsNotExist(err) {
		t.Error("Parts directory should be removed after cleanup")
	}
}

func TestCleanupOrphanedUploads(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create orphaned upload directories (simulating leftover from previous run)
	multipartDir := filepath.Join(tmpDir, ".multipart")
	orphanedUpload1 := filepath.Join(multipartDir, "orphaned-upload-1")
	orphanedUpload2 := filepath.Join(multipartDir, "orphaned-upload-2")

	if err := os.MkdirAll(orphanedUpload1, 0755); err != nil {
		t.Fatalf("Failed to create orphaned dir: %v", err)
	}
	if err := os.MkdirAll(orphanedUpload2, 0755); err != nil {
		t.Fatalf("Failed to create orphaned dir: %v", err)
	}

	// Create some files in orphaned directories
	if err := os.WriteFile(filepath.Join(orphanedUpload1, "part-1"), []byte("data"), 0644); err != nil {
		t.Fatalf("Failed to create orphaned file: %v", err)
	}

	// Verify orphaned directories exist
	if _, err := os.Stat(orphanedUpload1); os.IsNotExist(err) {
		t.Fatal("Orphaned directory should exist before cleanup")
	}

	// Create storage (this triggers cleanupOrphanedUploads)
	storage, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.multipart.Stop()

	// Verify orphaned directories were removed
	if _, err := os.Stat(orphanedUpload1); !os.IsNotExist(err) {
		t.Error("Orphaned directory 1 should be removed on startup")
	}
	if _, err := os.Stat(orphanedUpload2); !os.IsNotExist(err) {
		t.Error("Orphaned directory 2 should be removed on startup")
	}
}

func TestLastActivityTracking(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.multipart.Stop()

	bucket := "test-bucket"
	if err := storage.CreateBucket(bucket); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Initiate upload
	uploadID, err := storage.InitiateMultipartUpload(bucket, "test.txt")
	if err != nil {
		t.Fatalf("Failed to initiate upload: %v", err)
	}

	// Check LastActivity is set
	storage.multipart.mu.RLock()
	upload := storage.multipart.uploads[uploadID]
	upload.mu.RLock()
	initiatedTime := upload.Initiated
	lastActivity1 := upload.LastActivity
	upload.mu.RUnlock()
	storage.multipart.mu.RUnlock()

	if lastActivity1.IsZero() {
		t.Error("LastActivity should be set after initiation")
	}
	if !lastActivity1.Equal(initiatedTime) {
		t.Error("LastActivity should equal Initiated time initially")
	}

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Upload a part
	data := []byte("test data")
	if _, err := storage.UploadPart(uploadID, 1, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("Failed to upload part: %v", err)
	}

	// Check LastActivity was updated
	storage.multipart.mu.RLock()
	upload.mu.RLock()
	lastActivity2 := upload.LastActivity
	upload.mu.RUnlock()
	storage.multipart.mu.RUnlock()

	if !lastActivity2.After(lastActivity1) {
		t.Error("LastActivity should be updated after uploading a part")
	}
}
