package integration

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stut/s3dir/pkg/auth"
	"github.com/stut/s3dir/pkg/s3"
	"github.com/stut/s3dir/pkg/storage"
)

func setupIntegrationTest(t *testing.T) (*httptest.Server, func()) {
	tmpDir, err := os.MkdirTemp("", "s3dir-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	store, err := storage.New(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create storage: %v", err)
	}

	handler := s3.NewHandler(store, false, false)
	authenticator := auth.New("", "", false)

	var httpHandler http.Handler = handler
	httpHandler = authenticator.Middleware(httpHandler)

	server := httptest.NewServer(httpHandler)

	cleanup := func() {
		server.Close()
		os.RemoveAll(tmpDir)
	}

	return server, cleanup
}

func TestFullWorkflow(t *testing.T) {
	server, cleanup := setupIntegrationTest(t)
	defer cleanup()

	client := &http.Client{}

	// Step 1: List buckets (should be empty)
	resp, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("Failed to list buckets: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Step 2: Create a bucket
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/test-bucket", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for bucket creation, got %d", resp.StatusCode)
	}

	// Step 3: Head bucket
	req, _ = http.NewRequest(http.MethodHead, server.URL+"/test-bucket", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to head bucket: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for head bucket, got %d", resp.StatusCode)
	}

	// Step 4: Put an object
	testData := []byte("Hello, World!")
	req, _ = http.NewRequest(http.MethodPut, server.URL+"/test-bucket/hello.txt", bytes.NewReader(testData))
	req.ContentLength = int64(len(testData))
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to put object: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for put object, got %d", resp.StatusCode)
	}

	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header in put response")
	}

	// Step 5: Get the object
	resp, err = client.Get(server.URL + "/test-bucket/hello.txt")
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for get object, got %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read object content: %v", err)
	}

	if !bytes.Equal(content, testData) {
		t.Errorf("Content mismatch: expected %s, got %s", testData, content)
	}

	// Step 6: Head the object
	req, _ = http.NewRequest(http.MethodHead, server.URL+"/test-bucket/hello.txt", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to head object: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for head object, got %d", resp.StatusCode)
	}

	if resp.Header.Get("ETag") == "" {
		t.Error("Expected ETag header in head response")
	}

	if resp.Header.Get("Content-Length") == "" {
		t.Error("Expected Content-Length header in head response")
	}

	// Step 7: List objects
	resp, err = client.Get(server.URL + "/test-bucket")
	if err != nil {
		t.Fatalf("Failed to list objects: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for list objects, got %d", resp.StatusCode)
	}

	// Step 8: Delete the object
	req, _ = http.NewRequest(http.MethodDelete, server.URL+"/test-bucket/hello.txt", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete object: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204 for delete object, got %d", resp.StatusCode)
	}

	// Step 9: Verify object is deleted
	resp, err = client.Get(server.URL + "/test-bucket/hello.txt")
	if err != nil {
		t.Fatalf("Failed to get deleted object: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for deleted object, got %d", resp.StatusCode)
	}

	// Step 10: Delete the bucket
	req, _ = http.NewRequest(http.MethodDelete, server.URL+"/test-bucket", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete bucket: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204 for delete bucket, got %d", resp.StatusCode)
	}
}

func TestMultipleObjects(t *testing.T) {
	server, cleanup := setupIntegrationTest(t)
	defer cleanup()

	client := &http.Client{}

	// Create bucket
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/test-bucket", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}
	resp.Body.Close()

	// Upload multiple objects
	objects := map[string]string{
		"file1.txt":      "Content 1",
		"file2.txt":      "Content 2",
		"dir/file3.txt":  "Content 3",
		"dir/file4.txt":  "Content 4",
		"dir2/file5.txt": "Content 5",
	}

	for key, content := range objects {
		data := []byte(content)
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/test-bucket/"+key, bytes.NewReader(data))
		req.ContentLength = int64(len(data))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to put object %s: %v", key, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for put %s, got %d", key, resp.StatusCode)
		}
	}

	// List all objects
	resp, err = client.Get(server.URL + "/test-bucket")
	if err != nil {
		t.Fatalf("Failed to list objects: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// List with prefix
	resp, err = client.Get(server.URL + "/test-bucket?prefix=dir/")
	if err != nil {
		t.Fatalf("Failed to list objects with prefix: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify each object can be retrieved
	for key, expectedContent := range objects {
		resp, err := client.Get(server.URL + "/test-bucket/" + key)
		if err != nil {
			t.Fatalf("Failed to get object %s: %v", key, err)
		}

		content, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if string(content) != expectedContent {
			t.Errorf("Content mismatch for %s: expected %s, got %s", key, expectedContent, content)
		}
	}
}

func TestErrorCases(t *testing.T) {
	server, cleanup := setupIntegrationTest(t)
	defer cleanup()

	client := &http.Client{}

	// Try to get object from non-existing bucket
	resp, err := client.Get(server.URL + "/non-existing/file.txt")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existing bucket, got %d", resp.StatusCode)
	}

	// Create bucket
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/test-bucket", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}
	resp.Body.Close()

	// Try to get non-existing object
	resp, err = client.Get(server.URL + "/test-bucket/non-existing.txt")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existing object, got %d", resp.StatusCode)
	}

	// Try to put object without Content-Length
	req, _ = http.NewRequest(http.MethodPut, server.URL+"/test-bucket/test.txt", bytes.NewReader([]byte("test")))
	req.ContentLength = -1
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusLengthRequired {
		t.Errorf("Expected status 411 for missing Content-Length, got %d", resp.StatusCode)
	}

	// Try to delete non-empty bucket
	data := []byte("test")
	req, _ = http.NewRequest(http.MethodPut, server.URL+"/test-bucket/test.txt", bytes.NewReader(data))
	req.ContentLength = int64(len(data))
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to put object: %v", err)
	}
	resp.Body.Close()

	req, _ = http.NewRequest(http.MethodDelete, server.URL+"/test-bucket", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("Expected status 409 for deleting non-empty bucket, got %d", resp.StatusCode)
	}
}

func TestLargeObject(t *testing.T) {
	server, cleanup := setupIntegrationTest(t)
	defer cleanup()

	client := &http.Client{}

	// Create bucket
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/test-bucket", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}
	resp.Body.Close()

	// Create large data (1MB)
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	// Upload large object
	req, _ = http.NewRequest(http.MethodPut, server.URL+"/test-bucket/large.bin", bytes.NewReader(largeData))
	req.ContentLength = int64(len(largeData))
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to put large object: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for put large object, got %d", resp.StatusCode)
	}

	// Download and verify
	resp, err = client.Get(server.URL + "/test-bucket/large.bin")
	if err != nil {
		t.Fatalf("Failed to get large object: %v", err)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read large object: %v", err)
	}

	if !bytes.Equal(content, largeData) {
		t.Error("Large object content mismatch")
	}
}

func TestSpecialCharactersInKeys(t *testing.T) {
	server, cleanup := setupIntegrationTest(t)
	defer cleanup()

	client := &http.Client{}

	// Create bucket
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/test-bucket", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}
	resp.Body.Close()

	// Test keys with special characters
	testKeys := []string{
		"file with spaces.txt",
		"file-with-dashes.txt",
		"file_with_underscores.txt",
		"path/to/nested/file.txt",
	}

	for _, key := range testKeys {
		t.Run(key, func(t *testing.T) {
			data := []byte(fmt.Sprintf("Content for %s", key))

			// Put object
			req, _ := http.NewRequest(http.MethodPut, server.URL+"/test-bucket/"+key, bytes.NewReader(data))
			req.ContentLength = int64(len(data))
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to put object: %v", err)
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			// Get object
			resp, err = client.Get(server.URL + "/test-bucket/" + key)
			if err != nil {
				t.Fatalf("Failed to get object: %v", err)
			}

			content, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if !bytes.Equal(content, data) {
				t.Error("Content mismatch")
			}
		})
	}
}
