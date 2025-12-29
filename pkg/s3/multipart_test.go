package s3

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stut/s3dir/pkg/storage"
)

func TestMultipartUploadAPI(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-api-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	handler := NewHandler(store, false, false)

	bucket := "test-bucket"
	key := "test-file.txt"

	// Create bucket
	req := httptest.NewRequest(http.MethodPut, "/"+bucket, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create bucket: %d", w.Code)
	}

	// Initiate multipart upload
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/%s?uploads", bucket, key), nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to initiate multipart upload: %d - %s", w.Code, w.Body.String())
	}

	var initiateResp InitiateMultipartUploadResult
	if err := xml.Unmarshal(w.Body.Bytes(), &initiateResp); err != nil {
		t.Fatalf("Failed to parse initiate response: %v", err)
	}

	uploadID := initiateResp.UploadID
	if uploadID == "" {
		t.Fatal("Upload ID should not be empty")
	}
	if initiateResp.Bucket != bucket {
		t.Errorf("Expected bucket %q, got %q", bucket, initiateResp.Bucket)
	}
	if initiateResp.Key != key {
		t.Errorf("Expected key %q, got %q", key, initiateResp.Key)
	}

	// Upload part 1
	part1Data := "This is part 1 content. "
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/%s?uploadId=%s&partNumber=1", bucket, key, uploadID), strings.NewReader(part1Data))
	req.ContentLength = int64(len(part1Data))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to upload part 1: %d - %s", w.Code, w.Body.String())
	}
	etag1 := w.Header().Get("ETag")
	if etag1 == "" {
		t.Fatal("Part 1 ETag should not be empty")
	}

	// Upload part 2
	part2Data := "This is part 2 content."
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/%s?uploadId=%s&partNumber=2", bucket, key, uploadID), strings.NewReader(part2Data))
	req.ContentLength = int64(len(part2Data))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to upload part 2: %d - %s", w.Code, w.Body.String())
	}
	etag2 := w.Header().Get("ETag")
	if etag2 == "" {
		t.Fatal("Part 2 ETag should not be empty")
	}

	// List parts
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s?uploadId=%s", bucket, key, uploadID), nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to list parts: %d - %s", w.Code, w.Body.String())
	}

	var listPartsResp ListPartsResult
	if err := xml.Unmarshal(w.Body.Bytes(), &listPartsResp); err != nil {
		t.Fatalf("Failed to parse list parts response: %v", err)
	}

	if len(listPartsResp.Parts) != 2 {
		t.Errorf("Expected 2 parts, got %d", len(listPartsResp.Parts))
	}

	// List multipart uploads
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s?uploads", bucket), nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to list uploads: %d - %s", w.Code, w.Body.String())
	}

	var listUploadsResp ListMultipartUploadsResult
	if err := xml.Unmarshal(w.Body.Bytes(), &listUploadsResp); err != nil {
		t.Fatalf("Failed to parse list uploads response: %v", err)
	}

	if len(listUploadsResp.Uploads) != 1 {
		t.Errorf("Expected 1 upload, got %d", len(listUploadsResp.Uploads))
	}
	if listUploadsResp.Uploads[0].UploadID != uploadID {
		t.Errorf("Expected upload ID %q, got %q", uploadID, listUploadsResp.Uploads[0].UploadID)
	}

	// Complete multipart upload
	completeReq := CompleteMultipartUpload{
		Parts: []CompletePart{
			{PartNumber: 1, ETag: etag1},
			{PartNumber: 2, ETag: etag2},
		},
	}

	completeXML, err := xml.Marshal(completeReq)
	if err != nil {
		t.Fatalf("Failed to marshal complete request: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/%s?uploadId=%s", bucket, key, uploadID), bytes.NewReader(completeXML))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to complete multipart upload: %d - %s", w.Code, w.Body.String())
	}

	var completeResp CompleteMultipartUploadResult
	if err := xml.Unmarshal(w.Body.Bytes(), &completeResp); err != nil {
		t.Fatalf("Failed to parse complete response: %v", err)
	}

	if completeResp.Bucket != bucket {
		t.Errorf("Expected bucket %q, got %q", bucket, completeResp.Bucket)
	}
	if completeResp.Key != key {
		t.Errorf("Expected key %q, got %q", key, completeResp.Key)
	}
	if completeResp.ETag == "" {
		t.Error("Complete response ETag should not be empty")
	}

	// Verify object exists and has correct content
	req = httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to get object: %d", w.Code)
	}

	expectedContent := part1Data + part2Data
	if w.Body.String() != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, w.Body.String())
	}

	// Verify upload is no longer listed
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s?uploads", bucket), nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to list uploads: %d - %s", w.Code, w.Body.String())
	}

	var listUploadsResp2 ListMultipartUploadsResult
	if err := xml.Unmarshal(w.Body.Bytes(), &listUploadsResp2); err != nil {
		t.Fatalf("Failed to parse list uploads response: %v", err)
	}

	if len(listUploadsResp2.Uploads) != 0 {
		t.Errorf("Expected 0 uploads after completion, got %d", len(listUploadsResp2.Uploads))
	}
}

func TestAbortMultipartUploadAPI(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-api-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	handler := NewHandler(store, false, false)

	bucket := "test-bucket"
	key := "test-file.txt"

	// Create bucket
	req := httptest.NewRequest(http.MethodPut, "/"+bucket, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create bucket: %d", w.Code)
	}

	// Initiate multipart upload
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/%s?uploads", bucket, key), nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var initiateResp InitiateMultipartUploadResult
	if err := xml.Unmarshal(w.Body.Bytes(), &initiateResp); err != nil {
		t.Fatalf("Failed to parse initiate response: %v", err)
	}
	uploadID := initiateResp.UploadID

	// Upload a part
	partData := "Test data"
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/%s?uploadId=%s&partNumber=1", bucket, key, uploadID), strings.NewReader(partData))
	req.ContentLength = int64(len(partData))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to upload part: %d", w.Code)
	}

	// Abort the upload
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/%s/%s?uploadId=%s", bucket, key, uploadID), nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("Failed to abort upload: %d - %s", w.Code, w.Body.String())
	}

	// Verify upload is no longer listed
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s?uploads", bucket), nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var listUploadsResp ListMultipartUploadsResult
	if err := xml.Unmarshal(w.Body.Bytes(), &listUploadsResp); err != nil {
		t.Fatalf("Failed to parse list uploads response: %v", err)
	}

	if len(listUploadsResp.Uploads) != 0 {
		t.Errorf("Expected 0 uploads after abort, got %d", len(listUploadsResp.Uploads))
	}

	// Verify parts cannot be listed
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s?uploadId=%s", bucket, key, uploadID), nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 when listing parts of aborted upload, got %d", w.Code)
	}
}

func TestMultipartUploadAPIErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-api-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	handler := NewHandler(store, false, false)

	bucket := "test-bucket"

	t.Run("initiate on non-existing bucket", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/non-existing/key?uploads", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected 500, got %d", w.Code)
		}
	})

	// Create bucket for remaining tests
	req := httptest.NewRequest(http.MethodPut, "/"+bucket, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	t.Run("upload part without part number", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/key?uploadId=test-id", bucket), strings.NewReader("data"))
		req.ContentLength = 4
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("upload part with invalid part number", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/key?uploadId=test-id&partNumber=invalid", bucket), strings.NewReader("data"))
		req.ContentLength = 4
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("upload part without content-length", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/key?uploadId=test-id&partNumber=1", bucket), strings.NewReader("data"))
		req.ContentLength = -1
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusLengthRequired {
			t.Errorf("Expected 411, got %d", w.Code)
		}
	})

	t.Run("upload part to non-existing upload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/key?uploadId=non-existing&partNumber=1", bucket), strings.NewReader("data"))
		req.ContentLength = 4
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})

	t.Run("complete with invalid XML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/key?uploadId=test-id", bucket), strings.NewReader("invalid xml"))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("complete non-existing upload", func(t *testing.T) {
		completeReq := CompleteMultipartUpload{
			Parts: []CompletePart{{PartNumber: 1, ETag: "test"}},
		}
		completeXML, _ := xml.Marshal(completeReq)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/key?uploadId=non-existing", bucket), bytes.NewReader(completeXML))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected 404, got %d", w.Code)
		}
	})
}

func TestMultipartUploadReadOnlyMode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-api-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create handler in read-only mode
	handler := NewHandler(store, true, false)

	bucket := "test-bucket"

	t.Run("initiate multipart upload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/key?uploads", bucket), nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", w.Code)
		}
	})

	t.Run("upload part", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/key?uploadId=test&partNumber=1", bucket), strings.NewReader("data"))
		req.ContentLength = 4
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", w.Code)
		}
	})

	t.Run("complete upload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/key?uploadId=test", bucket), strings.NewReader("<CompleteMultipartUpload></CompleteMultipartUpload>"))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", w.Code)
		}
	})

	t.Run("abort upload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/%s/key?uploadId=test", bucket), nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", w.Code)
		}
	})
}

func TestMultipartUploadLargeFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3dir-api-multipart-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	handler := NewHandler(store, false, false)

	bucket := "test-bucket"
	key := "large-file.bin"

	// Create bucket
	req := httptest.NewRequest(http.MethodPut, "/"+bucket, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Initiate upload
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/%s?uploads", bucket, key), nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var initiateResp InitiateMultipartUploadResult
	xml.Unmarshal(w.Body.Bytes(), &initiateResp)
	uploadID := initiateResp.UploadID

	// Upload 5 parts of 1MB each
	partSize := 1024 * 1024 // 1MB
	numParts := 5
	var etags []string

	for i := 1; i <= numParts; i++ {
		partData := bytes.Repeat([]byte(fmt.Sprintf("%d", i)), partSize)
		req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/%s?uploadId=%s&partNumber=%d", bucket, key, uploadID, i), bytes.NewReader(partData))
		req.ContentLength = int64(len(partData))
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Failed to upload part %d: %d", i, w.Code)
		}
		etags = append(etags, w.Header().Get("ETag"))
	}

	// Complete upload
	var parts []CompletePart
	for i, etag := range etags {
		parts = append(parts, CompletePart{PartNumber: i + 1, ETag: etag})
	}

	completeReq := CompleteMultipartUpload{Parts: parts}
	completeXML, _ := xml.Marshal(completeReq)

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/%s?uploadId=%s", bucket, key, uploadID), bytes.NewReader(completeXML))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to complete upload: %d - %s", w.Code, w.Body.String())
	}

	// Verify object size
	req = httptest.NewRequest(http.MethodHead, "/"+bucket+"/"+key, nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to head object: %d", w.Code)
	}

	expectedSize := fmt.Sprintf("%d", partSize*numParts)
	if w.Header().Get("Content-Length") != expectedSize {
		t.Errorf("Expected size %s, got %s", expectedSize, w.Header().Get("Content-Length"))
	}

	// Verify we can read the object
	req = httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to get object: %d", w.Code)
	}

	data, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if len(data) != partSize*numParts {
		t.Errorf("Expected %d bytes, got %d", partSize*numParts, len(data))
	}
}
