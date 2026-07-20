package s3

import (
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPutObjectETagIsMD5(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	content := "consistent etag content"
	expectedETag := fmt.Sprintf("\"%x\"", md5.Sum([]byte(content)))

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/test.txt", strings.NewReader(content))
	req.ContentLength = int64(len(content))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	if got := w.Header().Get("ETag"); got != expectedETag {
		t.Errorf("PUT: expected ETag %s, got %s", expectedETag, got)
	}

	// HEAD, GET and List must agree
	req = httptest.NewRequest(http.MethodHead, "/test-bucket/test.txt", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if got := w.Header().Get("ETag"); got != expectedETag {
		t.Errorf("HEAD: expected ETag %s, got %s", expectedETag, got)
	}

	req = httptest.NewRequest(http.MethodGet, "/test-bucket/test.txt", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if got := w.Header().Get("ETag"); got != expectedETag {
		t.Errorf("GET: expected ETag %s, got %s", expectedETag, got)
	}

	req = httptest.NewRequest(http.MethodGet, "/test-bucket", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	var listing ListObjectsResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &listing); err != nil {
		t.Fatalf("Failed to parse listing: %v", err)
	}
	if len(listing.Contents) != 1 || listing.Contents[0].ETag != expectedETag {
		t.Errorf("List: expected ETag %s, got %+v", expectedETag, listing.Contents)
	}
}

func TestObjectMetadataRoundTrip(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/doc.html", strings.NewReader("<html></html>"))
	req.ContentLength = 13
	req.Header.Set("Content-Type", "text/html")
	req.Header.Set("x-amz-meta-owner", "stuart")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodHead, "/test-bucket/doc.html", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Content-Type"); got != "text/html" {
		t.Errorf("Expected Content-Type text/html, got %s", got)
	}
	if got := w.Header().Get("x-amz-meta-owner"); got != "stuart" {
		t.Errorf("Expected x-amz-meta-owner stuart, got %s", got)
	}
}

func TestCopyObjectMetadata(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/source.css", strings.NewReader("body{}"))
	req.ContentLength = 6
	req.Header.Set("Content-Type", "text/css")
	req.Header.Set("x-amz-meta-origin", "hand-written")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Default COPY directive carries metadata over
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/copied.css", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/source.css")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Copy failed: %d %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodHead, "/test-bucket/copied.css", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if got := w.Header().Get("Content-Type"); got != "text/css" {
		t.Errorf("COPY: expected Content-Type text/css, got %s", got)
	}
	if got := w.Header().Get("x-amz-meta-origin"); got != "hand-written" {
		t.Errorf("COPY: expected x-amz-meta-origin hand-written, got %s", got)
	}

	// REPLACE directive uses the request's metadata
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/replaced.css", nil)
	req.Header.Set("x-amz-copy-source", "/test-bucket/source.css")
	req.Header.Set("x-amz-metadata-directive", "REPLACE")
	req.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Copy with REPLACE failed: %d %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodHead, "/test-bucket/replaced.css", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if got := w.Header().Get("Content-Type"); got != "text/plain" {
		t.Errorf("REPLACE: expected Content-Type text/plain, got %s", got)
	}
	if got := w.Header().Get("x-amz-meta-origin"); got != "" {
		t.Errorf("REPLACE: expected no x-amz-meta-origin, got %s", got)
	}
}

func TestMultipartUploadMetadata(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	// Initiate with metadata
	req := httptest.NewRequest(http.MethodPost, "/test-bucket/big.bin?uploads", nil)
	req.Header.Set("Content-Type", "application/x-custom")
	req.Header.Set("x-amz-meta-source", "multipart")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Initiate failed: %d", w.Code)
	}
	var initResult InitiateMultipartUploadResult
	if err := xml.Unmarshal(w.Body.Bytes(), &initResult); err != nil {
		t.Fatalf("Failed to parse initiate response: %v", err)
	}

	// Upload one part and complete
	content := "part content"
	req = httptest.NewRequest(http.MethodPut,
		fmt.Sprintf("/test-bucket/big.bin?partNumber=1&uploadId=%s", initResult.UploadID),
		strings.NewReader(content))
	req.ContentLength = int64(len(content))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Upload part failed: %d", w.Code)
	}
	etag := w.Header().Get("ETag")

	completeXML := fmt.Sprintf(
		"<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>%s</ETag></Part></CompleteMultipartUpload>", etag)
	req = httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/test-bucket/big.bin?uploadId=%s", initResult.UploadID),
		strings.NewReader(completeXML))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Complete failed: %d %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodHead, "/test-bucket/big.bin", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if got := w.Header().Get("Content-Type"); got != "application/x-custom" {
		t.Errorf("Expected Content-Type application/x-custom, got %s", got)
	}
	if got := w.Header().Get("x-amz-meta-source"); got != "multipart" {
		t.Errorf("Expected x-amz-meta-source multipart, got %s", got)
	}
	// Multipart ETag has the -N suffix
	if got := w.Header().Get("ETag"); !strings.HasSuffix(got, "-1\"") {
		t.Errorf("Expected multipart ETag with -1 suffix, got %s", got)
	}
}

func TestGetObjectRange(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")
	content := "0123456789abcdefghij"
	putTestObject(t, handler, "test-bucket", "data.bin", content)

	tests := []struct {
		name          string
		rangeHeader   string
		expectedCode  int
		expectedBody  string
		expectedRange string
	}{
		{"first five", "bytes=0-4", http.StatusPartialContent, "01234", "bytes 0-4/20"},
		{"middle", "bytes=5-9", http.StatusPartialContent, "56789", "bytes 5-9/20"},
		{"last byte", "bytes=19-19", http.StatusPartialContent, "j", "bytes 19-19/20"},
		{"open ended", "bytes=15-", http.StatusPartialContent, "fghij", "bytes 15-19/20"},
		{"suffix", "bytes=-5", http.StatusPartialContent, "fghij", "bytes 15-19/20"},
		{"suffix larger than object", "bytes=-100", http.StatusPartialContent, content, "bytes 0-19/20"},
		{"end clamped to size", "bytes=10-100", http.StatusPartialContent, "abcdefghij", "bytes 10-19/20"},
		{"start beyond size", "bytes=20-25", http.StatusRequestedRangeNotSatisfiable, "", ""},
		{"malformed ignored", "bytes=abc", http.StatusOK, content, ""},
		{"multi-range ignored", "bytes=0-1,5-6", http.StatusOK, content, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test-bucket/data.bin", nil)
			req.Header.Set("Range", tt.rangeHeader)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Fatalf("Expected status %d, got %d: %s", tt.expectedCode, w.Code, w.Body.String())
			}
			if tt.expectedCode == http.StatusRequestedRangeNotSatisfiable {
				if got := w.Header().Get("Content-Range"); got != "bytes */20" {
					t.Errorf("Expected Content-Range bytes */20, got %s", got)
				}
				return
			}
			if w.Body.String() != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, w.Body.String())
			}
			if tt.expectedRange != "" {
				if got := w.Header().Get("Content-Range"); got != tt.expectedRange {
					t.Errorf("Expected Content-Range %s, got %s", tt.expectedRange, got)
				}
			}
		})
	}
}

func TestConditionalRequests(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")
	putTestObject(t, handler, "test-bucket", "test.txt", "conditional content")

	// Get the current ETag
	req := httptest.NewRequest(http.MethodHead, "/test-bucket/test.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	etag := w.Header().Get("ETag")

	// Matching If-None-Match returns 304 on GET and HEAD
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req = httptest.NewRequest(method, "/test-bucket/test.txt", nil)
		req.Header.Set("If-None-Match", etag)
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusNotModified {
			t.Errorf("%s If-None-Match: expected 304, got %d", method, w.Code)
		}
	}

	// Non-matching If-None-Match returns the object
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/test.txt", nil)
	req.Header.Set("If-None-Match", "\"different\"")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Non-matching If-None-Match: expected 200, got %d", w.Code)
	}

	// If-Modified-Since in the future returns 304
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/test.txt", nil)
	req.Header.Set("If-Modified-Since", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotModified {
		t.Errorf("Future If-Modified-Since: expected 304, got %d", w.Code)
	}

	// If-Modified-Since in the past returns the object
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/test.txt", nil)
	req.Header.Set("If-Modified-Since", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Past If-Modified-Since: expected 200, got %d", w.Code)
	}
}

func listKeys(t *testing.T, handler *Handler, url string) ([]string, ListObjectsV2Response) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("List %s failed: %d %s", url, w.Code, w.Body.String())
	}

	var response ListObjectsV2Response
	if err := xml.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse listing: %v", err)
	}

	var keys []string
	for _, obj := range response.Contents {
		keys = append(keys, obj.Key)
	}
	return keys, response
}

func TestListObjectsV1Pagination(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")
	for _, key := range []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt"} {
		putTestObject(t, handler, "test-bucket", key, "x")
	}

	// First page
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?max-keys=2", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	var page1 ListObjectsResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &page1); err != nil {
		t.Fatalf("Failed to parse page 1: %v", err)
	}
	if len(page1.Contents) != 2 || !page1.IsTruncated {
		t.Fatalf("Expected 2 keys and truncation, got %d keys truncated=%v", len(page1.Contents), page1.IsTruncated)
	}
	if page1.NextMarker != "b.txt" {
		t.Errorf("Expected NextMarker b.txt, got %s", page1.NextMarker)
	}

	// Second page resumes after the marker
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?max-keys=2&marker="+page1.NextMarker, nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	var page2 ListObjectsResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &page2); err != nil {
		t.Fatalf("Failed to parse page 2: %v", err)
	}
	if len(page2.Contents) != 2 || page2.Contents[0].Key != "c.txt" || page2.Contents[1].Key != "d.txt" {
		t.Errorf("Expected [c.txt d.txt], got %+v", page2.Contents)
	}

	// Final page
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?max-keys=2&marker="+page2.NextMarker, nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	var page3 ListObjectsResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &page3); err != nil {
		t.Fatalf("Failed to parse page 3: %v", err)
	}
	if len(page3.Contents) != 1 || page3.Contents[0].Key != "e.txt" || page3.IsTruncated {
		t.Errorf("Expected final page [e.txt] untruncated, got %+v truncated=%v", page3.Contents, page3.IsTruncated)
	}
}

func TestListObjectsV2(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")
	for _, key := range []string{"a.txt", "b.txt", "c.txt", "dir/nested.txt", "e.txt"} {
		putTestObject(t, handler, "test-bucket", key, "x")
	}

	// Full listing includes KeyCount
	keys, response := listKeys(t, handler, "/test-bucket?list-type=2")
	if len(keys) != 5 || response.KeyCount != 5 {
		t.Errorf("Expected 5 keys with KeyCount 5, got %d keys KeyCount=%d", len(keys), response.KeyCount)
	}

	// Paginate via continuation tokens, collecting everything
	var collected []string
	token := ""
	for page := 0; page < 10; page++ {
		url := "/test-bucket?list-type=2&max-keys=2"
		if token != "" {
			url += "&continuation-token=" + token
		}
		keys, response := listKeys(t, handler, url)
		collected = append(collected, keys...)
		if !response.IsTruncated {
			break
		}
		if response.NextContinuationToken == "" {
			t.Fatal("Truncated response missing NextContinuationToken")
		}
		token = response.NextContinuationToken
	}
	expected := []string{"a.txt", "b.txt", "c.txt", "dir/nested.txt", "e.txt"}
	if strings.Join(collected, ",") != strings.Join(expected, ",") {
		t.Errorf("Expected %v, got %v", expected, collected)
	}

	// start-after
	keys, _ = listKeys(t, handler, "/test-bucket?list-type=2&start-after=b.txt")
	if strings.Join(keys, ",") != "c.txt,dir/nested.txt,e.txt" {
		t.Errorf("Expected keys after b.txt, got %v", keys)
	}

	// Delimiter rolls up common prefixes, which count toward KeyCount
	keys, response = listKeys(t, handler, "/test-bucket?list-type=2&delimiter=/")
	if len(keys) != 4 || len(response.CommonPrefixes) != 1 || response.CommonPrefixes[0].Prefix != "dir/" {
		t.Errorf("Expected 4 keys + prefix dir/, got %v %+v", keys, response.CommonPrefixes)
	}
	if response.KeyCount != 5 {
		t.Errorf("Expected KeyCount 5 (4 keys + 1 prefix), got %d", response.KeyCount)
	}
}

func TestListObjectsDelimiterPagination(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")
	for _, key := range []string{"a/1.txt", "a/2.txt", "b/1.txt", "c.txt", "d/1.txt"} {
		putTestObject(t, handler, "test-bucket", key, "x")
	}

	// Page through with delimiter; prefixes and keys interleave in key order
	var entries []string
	token := ""
	for page := 0; page < 10; page++ {
		url := "/test-bucket?list-type=2&delimiter=/&max-keys=2"
		if token != "" {
			url += "&continuation-token=" + token
		}
		keys, response := listKeys(t, handler, url)
		entries = append(entries, keys...)
		for _, cp := range response.CommonPrefixes {
			entries = append(entries, cp.Prefix)
		}
		if !response.IsTruncated {
			break
		}
		token = response.NextContinuationToken
	}

	expected := map[string]bool{"a/": true, "b/": true, "c.txt": true, "d/": true}
	if len(entries) != 4 {
		t.Fatalf("Expected 4 entries, got %v", entries)
	}
	for _, e := range entries {
		if !expected[e] {
			t.Errorf("Unexpected entry %s in %v", e, entries)
		}
	}
}

func TestGetBucketLocation(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	req := httptest.NewRequest(http.MethodGet, "/test-bucket?location", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "LocationConstraint") {
		t.Errorf("Expected LocationConstraint response, got %s", w.Body.String())
	}

	// Missing bucket
	req = httptest.NewRequest(http.MethodGet, "/no-such-bucket?location", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for missing bucket, got %d", w.Code)
	}
}

func TestBucketSubresources(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")

	tests := []struct {
		query        string
		expectedCode int
		expectedBody string
	}{
		{"versioning", http.StatusOK, "VersioningConfiguration"},
		{"acl", http.StatusOK, "AccessControlPolicy"},
		{"tagging", http.StatusNotFound, "NoSuchTagSet"},
		{"lifecycle", http.StatusNotFound, "NoSuchLifecycleConfiguration"},
		{"cors", http.StatusNotFound, "NoSuchCORSConfiguration"},
		{"policy", http.StatusNotFound, "NoSuchBucketPolicy"},
		{"encryption", http.StatusNotFound, "ServerSideEncryptionConfigurationNotFoundError"},
		{"object-lock", http.StatusNotFound, "ObjectLockConfigurationNotFoundError"},
	}

	for _, tt := range tests {
		t.Run("GET "+tt.query, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test-bucket?"+tt.query, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status %d, got %d", tt.expectedCode, w.Code)
			}
			if !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("Expected %s in response, got %s", tt.expectedBody, w.Body.String())
			}
		})
	}

	// PUT of a subresource is a no-op, not a bucket creation
	req := httptest.NewRequest(http.MethodPut, "/test-bucket?versioning", strings.NewReader("<VersioningConfiguration/>"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PUT ?versioning: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// DELETE of a subresource must NOT delete the bucket
	req = httptest.NewRequest(http.MethodDelete, "/test-bucket?lifecycle", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE ?lifecycle: expected 204, got %d", w.Code)
	}
	if err := store.HeadBucket("test-bucket"); err != nil {
		t.Error("Bucket was deleted by a subresource DELETE")
	}
}

func TestObjectSubresources(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("test-bucket")
	content := "protected content"
	putTestObject(t, handler, "test-bucket", "test.txt", content)

	// GET ?acl returns a stub policy
	req := httptest.NewRequest(http.MethodGet, "/test-bucket/test.txt?acl", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "AccessControlPolicy") {
		t.Errorf("GET ?acl: expected 200 with policy, got %d: %s", w.Code, w.Body.String())
	}

	// GET ?tagging returns an empty tag set
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/test.txt?tagging", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Tagging") {
		t.Errorf("GET ?tagging: expected 200 with Tagging, got %d: %s", w.Code, w.Body.String())
	}

	// PUT ?acl must not overwrite the object data
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/test.txt?acl", strings.NewReader("<AccessControlPolicy/>"))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PUT ?acl: expected 200, got %d", w.Code)
	}
	code, body := getTestObject(t, handler, "test-bucket", "test.txt")
	if code != http.StatusOK || body != content {
		t.Errorf("Object corrupted by PUT ?acl: %d %q", code, body)
	}

	// Missing object
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/no-such-key?acl", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("GET ?acl on missing key: expected 404, got %d", w.Code)
	}
}

func TestListBucketsHidesInternalDirs(t *testing.T) {
	handler, store, cleanup := setupTestHandler(t)
	defer cleanup()

	store.CreateBucket("visible-bucket")
	// Object metadata and multipart state must not surface as buckets
	putTestObject(t, handler, "visible-bucket", "file.txt", "content")
	if _, err := store.InitiateMultipartUpload("visible-bucket", "pending.bin"); err != nil {
		t.Fatalf("Failed to initiate upload: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var response ListBucketsResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(response.Buckets.Buckets) != 1 || response.Buckets.Buckets[0].Name != "visible-bucket" {
		t.Errorf("Expected only visible-bucket, got %+v", response.Buckets.Buckets)
	}
}
