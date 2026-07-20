package s3

import (
	"bytes"
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func putTestObject(t *testing.T, handler *Handler, bucket, key, content string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, strings.NewReader(content))
	req.ContentLength = int64(len(content))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to put test object %s/%s: status %d", bucket, key, w.Code)
	}
}

func getTestObject(t *testing.T, handler *Handler, bucket, key string) (int, string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	return w.Code, w.Body.String()
}

func TestCopyObject(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("src-bucket")
	store.CreateBucket("dst-bucket")

	content := "hello copy object"
	putTestObject(t, handler, "src-bucket", "source.txt", content)

	tests := []struct {
		name       string
		copySource string
	}{
		{"leading slash", "/src-bucket/source.txt"},
		{"no leading slash", "src-bucket/source.txt"},
		{"url-encoded", "/src-bucket/source%2Etxt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/dst-bucket/dest.txt", nil)
			req.Header.Set("x-amz-copy-source", tt.copySource)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
			}

			if ct := w.Header().Get("Content-Type"); ct != "application/xml" {
				t.Errorf("Expected Content-Type application/xml, got %s", ct)
			}

			var result CopyObjectResult
			if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			expectedETag := fmt.Sprintf("\"%x\"", md5.Sum([]byte(content)))
			if result.ETag != expectedETag {
				t.Errorf("Expected ETag %s, got %s", expectedETag, result.ETag)
			}
			if result.LastModified == "" {
				t.Error("Expected LastModified to be set")
			}

			code, body := getTestObject(t, handler, "dst-bucket", "dest.txt")
			if code != http.StatusOK {
				t.Fatalf("Expected status 200 for copied object, got %d", code)
			}
			if body != content {
				t.Errorf("Expected content %q, got %q", content, body)
			}
		})
	}
}

func TestCopyObjectWithEncodedKey(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	content := "content with spaces in key"
	putTestObject(t, handler, "test-bucket", "path/my%20file.txt", content)

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/copied.txt", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/path/my%20file.txt")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	code, body := getTestObject(t, handler, "test-bucket", "copied.txt")
	if code != http.StatusOK {
		t.Fatalf("Expected status 200 for copied object, got %d", code)
	}
	if body != content {
		t.Errorf("Expected content %q, got %q", content, body)
	}
}

func TestCopyObjectErrors(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	// Missing source object
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/dest.txt", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/no-such-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for missing source, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "NoSuchKey") {
		t.Errorf("Expected NoSuchKey error, got %s", w.Body.String())
	}

	// Malformed copy source (no key)
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/dest.txt", nil)
	req.Header.Set("x-amz-copy-source", "/only-a-bucket")
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for malformed copy source, got %d", w.Code)
	}
}

func TestCopyObjectReadOnlyMode(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")
	putTestObject(t, handler, "test-bucket", "source.txt", "content")

	readOnlyHandler := NewHandler(store, true, false)

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/dest.txt", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/source.txt")
	w := httptest.NewRecorder()

	readOnlyHandler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 in read-only mode, got %d", w.Code)
	}
}

func TestUploadPartCopy(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	content := "0123456789abcdefghij"
	putTestObject(t, handler, "test-bucket", "source.txt", content)

	tests := []struct {
		name        string
		rangeHeader string
		expected    string
	}{
		{"whole object", "", content},
		{"first byte", "bytes=0-0", "0"},
		{"last byte", "bytes=19-19", "j"},
		{"middle range", "bytes=5-9", "56789"},
		{"full range", "bytes=0-19", content},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uploadID, err := store.InitiateMultipartUpload("test-bucket", "assembled.txt")
			if err != nil {
				t.Fatalf("Failed to initiate upload: %v", err)
			}

			url := fmt.Sprintf("/test-bucket/assembled.txt?partNumber=1&uploadId=%s", uploadID)
			req := httptest.NewRequest(http.MethodPut, url, nil)
			req.Header.Set("x-amz-copy-source", "/test-bucket/source.txt")
			if tt.rangeHeader != "" {
				req.Header.Set("x-amz-copy-source-range", tt.rangeHeader)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
			}

			var result CopyPartResult
			if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			expectedETag := fmt.Sprintf("\"%x\"", md5.Sum([]byte(tt.expected)))
			if result.ETag != expectedETag {
				t.Errorf("Expected ETag %s, got %s", expectedETag, result.ETag)
			}
			if result.LastModified == "" {
				t.Error("Expected LastModified to be set")
			}

			// Complete the upload and verify the assembled content
			completeXML := fmt.Sprintf(
				"<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>%s</ETag></Part></CompleteMultipartUpload>",
				result.ETag)
			req = httptest.NewRequest(http.MethodPost,
				fmt.Sprintf("/test-bucket/assembled.txt?uploadId=%s", uploadID),
				bytes.NewReader([]byte(completeXML)))
			w = httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Failed to complete upload: status %d: %s", w.Code, w.Body.String())
			}

			code, body := getTestObject(t, handler, "test-bucket", "assembled.txt")
			if code != http.StatusOK {
				t.Fatalf("Expected status 200 for assembled object, got %d", code)
			}
			if body != tt.expected {
				t.Errorf("Expected content %q, got %q", tt.expected, body)
			}
		})
	}
}

func TestUploadPartCopyMultipleParts(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	content := "0123456789abcdefghij"
	putTestObject(t, handler, "test-bucket", "source.txt", content)

	uploadID, err := store.InitiateMultipartUpload("test-bucket", "assembled.txt")
	if err != nil {
		t.Fatalf("Failed to initiate upload: %v", err)
	}

	// Copy the source in two ranged parts
	ranges := []string{"bytes=0-9", "bytes=10-19"}
	etags := make([]string, len(ranges))
	for i, rangeHeader := range ranges {
		url := fmt.Sprintf("/test-bucket/assembled.txt?partNumber=%d&uploadId=%s", i+1, uploadID)
		req := httptest.NewRequest(http.MethodPut, url, nil)
		req.Header.Set("x-amz-copy-source", "/test-bucket/source.txt")
		req.Header.Set("x-amz-copy-source-range", rangeHeader)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200 for part %d, got %d: %s", i+1, w.Code, w.Body.String())
		}

		var result CopyPartResult
		if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}
		etags[i] = result.ETag
	}

	var completeXML strings.Builder
	completeXML.WriteString("<CompleteMultipartUpload>")
	for i, etag := range etags {
		fmt.Fprintf(&completeXML, "<Part><PartNumber>%d</PartNumber><ETag>%s</ETag></Part>", i+1, etag)
	}
	completeXML.WriteString("</CompleteMultipartUpload>")

	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/test-bucket/assembled.txt?uploadId=%s", uploadID),
		strings.NewReader(completeXML.String()))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to complete upload: status %d: %s", w.Code, w.Body.String())
	}

	code, body := getTestObject(t, handler, "test-bucket", "assembled.txt")
	if code != http.StatusOK {
		t.Fatalf("Expected status 200 for assembled object, got %d", code)
	}
	if body != content {
		t.Errorf("Expected content %q, got %q", content, body)
	}
}

func TestUploadPartCopyErrors(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	content := "0123456789"
	putTestObject(t, handler, "test-bucket", "source.txt", content)

	uploadID, err := store.InitiateMultipartUpload("test-bucket", "assembled.txt")
	if err != nil {
		t.Fatalf("Failed to initiate upload: %v", err)
	}

	uploadURL := fmt.Sprintf("/test-bucket/assembled.txt?partNumber=1&uploadId=%s", uploadID)

	tests := []struct {
		name         string
		copySource   string
		rangeHeader  string
		expectedCode int
		expectedErr  string
	}{
		{"missing source", "/test-bucket/no-such-key", "", http.StatusNotFound, "NoSuchKey"},
		{"range end out of bounds", "/test-bucket/source.txt", "bytes=0-10", http.StatusRequestedRangeNotSatisfiable, "InvalidRange"},
		{"range start out of bounds", "/test-bucket/source.txt", "bytes=10-15", http.StatusRequestedRangeNotSatisfiable, "InvalidRange"},
		{"malformed range", "/test-bucket/source.txt", "bytes=abc", http.StatusBadRequest, "InvalidArgument"},
		{"inverted range", "/test-bucket/source.txt", "bytes=5-2", http.StatusBadRequest, "InvalidArgument"},
		{"malformed source", "no-key", "", http.StatusBadRequest, "InvalidArgument"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, uploadURL, nil)
			req.Header.Set("x-amz-copy-source", tt.copySource)
			if tt.rangeHeader != "" {
				req.Header.Set("x-amz-copy-source-range", tt.rangeHeader)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status %d, got %d: %s", tt.expectedCode, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tt.expectedErr) {
				t.Errorf("Expected %s error, got %s", tt.expectedErr, w.Body.String())
			}
		})
	}

	// Nonexistent upload
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/assembled.txt?partNumber=1&uploadId=no-such-upload", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/source.txt")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for missing upload, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "NoSuchUpload") {
		t.Errorf("Expected NoSuchUpload error, got %s", w.Body.String())
	}
}

func TestUploadPartCopyReadOnlyMode(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")
	putTestObject(t, handler, "test-bucket", "source.txt", "content")

	readOnlyHandler := NewHandler(store, true, false)

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/assembled.txt?partNumber=1&uploadId=any", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/source.txt")
	w := httptest.NewRecorder()

	readOnlyHandler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 in read-only mode, got %d", w.Code)
	}
}

func TestDeleteObjects(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")
	putTestObject(t, handler, "test-bucket", "exists-1.txt", "one")
	putTestObject(t, handler, "test-bucket", "exists-2.txt", "two")

	// Mix of existing and missing keys - all should count as deleted
	body := "<Delete><Quiet>false</Quiet>" +
		"<Object><Key>exists-1.txt</Key></Object>" +
		"<Object><Key>missing.txt</Key></Object>" +
		"<Object><Key>exists-2.txt</Key></Object>" +
		"</Delete>"

	req := httptest.NewRequest(http.MethodPost, "/test-bucket?delete", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result DeleteResult
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result.Deleted) != 3 {
		t.Errorf("Expected 3 deleted entries, got %d", len(result.Deleted))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(result.Errors))
	}

	// Objects must actually be gone
	for _, key := range []string{"exists-1.txt", "exists-2.txt"} {
		code, _ := getTestObject(t, handler, "test-bucket", key)
		if code != http.StatusNotFound {
			t.Errorf("Expected %s to be deleted, got status %d", key, code)
		}
	}
}

func TestDeleteObjectsQuietMode(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")
	putTestObject(t, handler, "test-bucket", "exists.txt", "content")

	body := "<Delete><Quiet>true</Quiet>" +
		"<Object><Key>exists.txt</Key></Object>" +
		"<Object><Key>missing.txt</Key></Object>" +
		"</Delete>"

	req := httptest.NewRequest(http.MethodPost, "/test-bucket?delete", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result DeleteResult
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result.Deleted) != 0 {
		t.Errorf("Expected no deleted entries in quiet mode, got %d", len(result.Deleted))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(result.Errors))
	}

	code, _ := getTestObject(t, handler, "test-bucket", "exists.txt")
	if code != http.StatusNotFound {
		t.Errorf("Expected object to be deleted, got status %d", code)
	}
}

func TestDeleteObjectsMalformedXML(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	req := httptest.NewRequest(http.MethodPost, "/test-bucket?delete", strings.NewReader("not xml"))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for malformed XML, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "MalformedXML") {
		t.Errorf("Expected MalformedXML error, got %s", w.Body.String())
	}
}

func TestDeleteObjectsReadOnlyMode(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")
	putTestObject(t, handler, "test-bucket", "exists.txt", "content")

	readOnlyHandler := NewHandler(store, true, false)

	body := "<Delete><Object><Key>exists.txt</Key></Object></Delete>"
	req := httptest.NewRequest(http.MethodPost, "/test-bucket?delete", strings.NewReader(body))
	w := httptest.NewRecorder()

	readOnlyHandler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 in read-only mode, got %d", w.Code)
	}

	// Object must still exist
	code, _ := getTestObject(t, handler, "test-bucket", "exists.txt")
	if code != http.StatusOK {
		t.Errorf("Expected object to still exist, got status %d", code)
	}
}
