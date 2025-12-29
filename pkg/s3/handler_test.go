package s3

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stut/s3dir/pkg/storage"
)

func setupTestHandler(t *testing.T) (*Handler, *storage.Storage, func()) {
	tmpDir, err := os.MkdirTemp("", "s3dir-handler-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	store, err := storage.New(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create storage: %v", err)
	}

	handler := NewHandler(store, false, false)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return handler, store, cleanup
}

func TestListBuckets(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create test buckets
	store.CreateBucket("bucket1")
	store.CreateBucket("bucket2")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response ListBucketsResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(response.Buckets.Buckets) != 2 {
		t.Errorf("Expected 2 buckets, got %d", len(response.Buckets.Buckets))
	}
}

func TestCreateBucket(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPut, "/test-bucket", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Try to create the same bucket again
	req = httptest.NewRequest(http.MethodPut, "/test-bucket", nil)
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409 for duplicate bucket, got %d", w.Code)
	}
}

func TestHeadBucket(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	req := httptest.NewRequest(http.MethodHead, "/test-bucket", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Head non-existing bucket
	req = httptest.NewRequest(http.MethodHead, "/non-existing", nil)
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existing bucket, got %d", w.Code)
	}
}

func TestDeleteBucket(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	req := httptest.NewRequest(http.MethodDelete, "/test-bucket", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	// Verify bucket is deleted
	err := store.HeadBucket("test-bucket")
	if err == nil {
		t.Error("Bucket should not exist after deletion")
	}
}

func TestPutObject(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	testData := []byte("Hello, S3Dir!")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test.txt", bytes.NewReader(testData))
	req.ContentLength = int64(len(testData))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify ETag header
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header")
	}

	// Verify object was stored
	_, info, err := store.GetObject("test-bucket", "test.txt")
	if err != nil {
		t.Errorf("Failed to get stored object: %v", err)
	}

	if info.Size != int64(len(testData)) {
		t.Errorf("Expected size %d, got %d", len(testData), info.Size)
	}
}

func TestPutObjectWithoutContentLength(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	testData := []byte("test data")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test.txt", bytes.NewReader(testData))
	req.ContentLength = -1 // Simulate missing Content-Length
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusLengthRequired {
		t.Errorf("Expected status 411, got %d", w.Code)
	}
}

func TestGetObject(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	testData := []byte("Hello, S3Dir!")
	store.PutObject("test-bucket", "test.txt", bytes.NewReader(testData), int64(len(testData)))

	req := httptest.NewRequest(http.MethodGet, "/test-bucket/test.txt", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify headers
	if w.Header().Get("Content-Length") == "" {
		t.Error("Expected Content-Length header")
	}

	if w.Header().Get("ETag") == "" {
		t.Error("Expected ETag header")
	}

	if w.Header().Get("Last-Modified") == "" {
		t.Error("Expected Last-Modified header")
	}

	// Verify content
	content := w.Body.Bytes()
	if !bytes.Equal(content, testData) {
		t.Errorf("Content mismatch: expected %s, got %s", testData, content)
	}
}

func TestGetNonExistingObject(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	req := httptest.NewRequest(http.MethodGet, "/test-bucket/non-existing.txt", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	var errorResp ErrorResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &errorResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errorResp.Code != "NoSuchKey" {
		t.Errorf("Expected error code 'NoSuchKey', got '%s'", errorResp.Code)
	}
}

func TestHeadObject(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	testData := []byte("test data")
	store.PutObject("test-bucket", "test.txt", bytes.NewReader(testData), int64(len(testData)))

	req := httptest.NewRequest(http.MethodHead, "/test-bucket/test.txt", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify headers
	if w.Header().Get("Content-Length") == "" {
		t.Error("Expected Content-Length header")
	}

	if w.Header().Get("ETag") == "" {
		t.Error("Expected ETag header")
	}

	// Verify no body
	if w.Body.Len() > 0 {
		t.Error("HEAD response should have no body")
	}
}

func TestDeleteObject(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	testData := []byte("test data")
	store.PutObject("test-bucket", "test.txt", bytes.NewReader(testData), int64(len(testData)))

	req := httptest.NewRequest(http.MethodDelete, "/test-bucket/test.txt", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	// Verify object is deleted
	_, _, err := store.GetObject("test-bucket", "test.txt")
	if err == nil {
		t.Error("Object should not exist after deletion")
	}
}

func TestListObjects(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	// Create test objects
	objects := []string{"file1.txt", "file2.txt", "dir/file3.txt"}
	for _, key := range objects {
		data := []byte("test")
		store.PutObject("test-bucket", key, bytes.NewReader(data), int64(len(data)))
	}

	req := httptest.NewRequest(http.MethodGet, "/test-bucket", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response ListObjectsResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(response.Contents) != 3 {
		t.Errorf("Expected 3 objects, got %d", len(response.Contents))
	}

	if response.Name != "test-bucket" {
		t.Errorf("Expected bucket name 'test-bucket', got '%s'", response.Name)
	}
}

func TestListObjectsWithPrefix(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	// Create test objects
	objects := []string{"file1.txt", "dir/file2.txt", "dir/file3.txt"}
	for _, key := range objects {
		data := []byte("test")
		store.PutObject("test-bucket", key, bytes.NewReader(data), int64(len(data)))
	}

	req := httptest.NewRequest(http.MethodGet, "/test-bucket?prefix=dir/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response ListObjectsResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(response.Contents) != 2 {
		t.Errorf("Expected 2 objects with prefix 'dir/', got %d", len(response.Contents))
	}

	if response.Prefix != "dir/" {
		t.Errorf("Expected prefix 'dir/', got '%s'", response.Prefix)
	}
}

func TestReadOnlyMode(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	// Enable read-only mode
	handler.readOnly = true

	store.CreateBucket("test-bucket")

	// Try to PUT object in read-only mode
	testData := []byte("test")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test.txt", bytes.NewReader(testData))
	req.ContentLength = int64(len(testData))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 in read-only mode, got %d", w.Code)
	}

	// Try to DELETE in read-only mode
	req = httptest.NewRequest(http.MethodDelete, "/test-bucket/test.txt", nil)
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 in read-only mode, got %d", w.Code)
	}

	// GET should still work
	data := []byte("test")
	store.PutObject("test-bucket", "existing.txt", bytes.NewReader(data), int64(len(data)))

	req = httptest.NewRequest(http.MethodGet, "/test-bucket/existing.txt", nil)
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for GET in read-only mode, got %d", w.Code)
	}
}

func TestParsePath(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		path       string
		wantBucket string
		wantKey    string
	}{
		{"/", "", ""},
		{"/bucket", "bucket", ""},
		{"/bucket/", "bucket", ""},
		{"/bucket/key", "bucket", "key"},
		{"/bucket/path/to/key", "bucket", "path/to/key"},
		{"/bucket/key/", "bucket", "key/"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			bucket, key := handler.parsePath(tt.path)
			if bucket != tt.wantBucket {
				t.Errorf("parsePath(%s) bucket = %s, want %s", tt.path, bucket, tt.wantBucket)
			}
			if key != tt.wantKey {
				t.Errorf("parsePath(%s) key = %s, want %s", tt.path, key, tt.wantKey)
			}
		})
	}
}
