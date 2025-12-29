package s3

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/stut/s3dir/pkg/storage"
)

// Handler handles S3 API requests
type Handler struct {
	storage  *storage.Storage
	readOnly bool
	verbose  bool
}

// NewHandler creates a new S3 handler
func NewHandler(storage *storage.Storage, readOnly, verbose bool) *Handler {
	return &Handler{
		storage:  storage,
		readOnly: readOnly,
		verbose:  verbose,
	}
}

// ServeHTTP handles HTTP requests and routes them to appropriate handlers
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.verbose {
		fmt.Printf("%s %s\n", r.Method, r.URL.Path)
	}

	// Parse bucket and key from path
	bucket, key := h.parsePath(r.URL.Path)

	// Route to appropriate handler
	if bucket == "" {
		// Service-level operation (list buckets)
		h.handleServiceOperation(w, r)
		return
	}

	if key == "" {
		// Bucket-level operation
		h.handleBucketOperation(w, r, bucket)
		return
	}

	// Object-level operation
	h.handleObjectOperation(w, r, bucket, key)
}

// handleServiceOperation handles service-level operations
func (h *Handler) handleServiceOperation(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listBuckets(w, r)
	default:
		writeError(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleBucketOperation handles bucket-level operations
func (h *Handler) handleBucketOperation(w http.ResponseWriter, r *http.Request, bucket string) {
	switch r.Method {
	case http.MethodGet:
		h.listObjects(w, r, bucket)
	case http.MethodHead:
		h.headBucket(w, r, bucket)
	case http.MethodPut:
		if h.readOnly {
			writeError(w, "AccessDenied", "Read-only mode", http.StatusForbidden)
			return
		}
		h.createBucket(w, r, bucket)
	case http.MethodDelete:
		if h.readOnly {
			writeError(w, "AccessDenied", "Read-only mode", http.StatusForbidden)
			return
		}
		h.deleteBucket(w, r, bucket)
	default:
		writeError(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleObjectOperation handles object-level operations
func (h *Handler) handleObjectOperation(w http.ResponseWriter, r *http.Request, bucket, key string) {
	switch r.Method {
	case http.MethodGet:
		h.getObject(w, r, bucket, key)
	case http.MethodHead:
		h.headObject(w, r, bucket, key)
	case http.MethodPut:
		if h.readOnly {
			writeError(w, "AccessDenied", "Read-only mode", http.StatusForbidden)
			return
		}
		h.putObject(w, r, bucket, key)
	case http.MethodDelete:
		if h.readOnly {
			writeError(w, "AccessDenied", "Read-only mode", http.StatusForbidden)
			return
		}
		h.deleteObject(w, r, bucket, key)
	default:
		writeError(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listBuckets lists all buckets
func (h *Handler) listBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := h.storage.ListBuckets()
	if err != nil {
		writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	var bucketList []Bucket
	for _, name := range buckets {
		bucketList = append(bucketList, Bucket{
			Name:         name,
			CreationDate: time.Now().Format(time.RFC3339),
		})
	}

	response := ListBucketsResponse{
		Buckets: BucketList{Buckets: bucketList},
		Owner: Owner{
			ID:          "s3dir",
			DisplayName: "s3dir",
		},
	}

	writeXML(w, response, http.StatusOK)
}

// listObjects lists objects in a bucket
func (h *Handler) listObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	query := r.URL.Query()
	prefix := query.Get("prefix")
	delimiter := query.Get("delimiter")
	maxKeys := 1000
	if mk := query.Get("max-keys"); mk != "" {
		if n, err := strconv.Atoi(mk); err == nil && n > 0 {
			maxKeys = n
		}
	}

	objects, commonPrefixes, err := h.storage.ListObjects(bucket, prefix, delimiter, maxKeys)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchBucket", "The specified bucket does not exist", http.StatusNotFound)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	var contents []Object
	for _, obj := range objects {
		contents = append(contents, Object{
			Key:          obj.Key,
			LastModified: obj.LastModified.Format(time.RFC3339),
			ETag:         obj.ETag,
			Size:         obj.Size,
			StorageClass: "STANDARD",
		})
	}

	var prefixes []CommonPrefix
	for _, cp := range commonPrefixes {
		prefixes = append(prefixes, CommonPrefix{Prefix: cp})
	}

	response := ListObjectsResponse{
		Name:           bucket,
		Prefix:         prefix,
		Delimiter:      delimiter,
		MaxKeys:        maxKeys,
		IsTruncated:    false,
		Contents:       contents,
		CommonPrefixes: prefixes,
	}

	writeXML(w, response, http.StatusOK)
}

// getObject retrieves an object
func (h *Handler) getObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	reader, info, err := h.storage.GetObject(bucket, key)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchKey", "The specified key does not exist", http.StatusNotFound)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}
	defer reader.Close()

	// Set headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.Header().Set("ETag", info.ETag)
	w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))

	// Copy object data to response
	io.Copy(w, reader)
}

// headObject retrieves object metadata
func (h *Handler) headObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	info, err := h.storage.HeadObject(bucket, key)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchKey", "The specified key does not exist", http.StatusNotFound)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Set headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.Header().Set("ETag", info.ETag)
	w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
}

// putObject stores an object
func (h *Handler) putObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	contentLength := r.ContentLength
	if contentLength < 0 {
		writeError(w, "MissingContentLength", "Content-Length header is required", http.StatusLengthRequired)
		return
	}

	if err := h.storage.PutObject(bucket, key, r.Body, contentLength); err != nil {
		writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate ETag (simplified)
	etag := fmt.Sprintf("\"%x\"", time.Now().Unix())
	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
}

// deleteObject deletes an object
func (h *Handler) deleteObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if err := h.storage.DeleteObject(bucket, key); err != nil {
		writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// createBucket creates a new bucket
func (h *Handler) createBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if err := h.storage.CreateBucket(bucket); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, "BucketAlreadyExists", "The bucket already exists", http.StatusConflict)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

// deleteBucket deletes a bucket
func (h *Handler) deleteBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if err := h.storage.DeleteBucket(bucket); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchBucket", "The specified bucket does not exist", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "not empty") {
			writeError(w, "BucketNotEmpty", "The bucket is not empty", http.StatusConflict)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// headBucket checks if a bucket exists
func (h *Handler) headBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if err := h.storage.HeadBucket(bucket); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchBucket", "The specified bucket does not exist", http.StatusNotFound)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

// parsePath parses the bucket and key from the request path
func (h *Handler) parsePath(path string) (bucket, key string) {
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		return "", ""
	}

	parts := strings.SplitN(path, "/", 2)
	bucket = parts[0]

	// Remove trailing slash from bucket name only
	bucket = strings.TrimSuffix(bucket, "/")

	if len(parts) > 1 {
		key = parts[1]
	}

	return bucket, key
}

// writeXML writes an XML response
func writeXML(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(statusCode)

	output, err := xml.MarshalIndent(data, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(xml.Header))
	w.Write(output)
}

// writeError writes an S3 error response
func writeError(w http.ResponseWriter, code, message string, statusCode int) {
	errorResponse := ErrorResponse{
		Code:    code,
		Message: message,
	}

	writeXML(w, errorResponse, statusCode)
}
