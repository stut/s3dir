package s3

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
		fmt.Printf("%s %s\n", r.Method, r.URL.RequestURI())
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

// bucketSubresources are the bucket configuration subresources s3dir
// recognises but does not implement. GETs receive a stub or the S3 error code
// a real bucket without that configuration would return; PUTs and DELETEs are
// accepted as no-ops so clients cannot accidentally create or delete the
// bucket itself through them
var bucketSubresources = []string{
	"accelerate", "acl", "cors", "encryption", "lifecycle", "location",
	"logging", "notification", "object-lock", "policy", "replication",
	"requestPayment", "tagging", "versioning", "website",
}

// hasBucketSubresource reports whether the query addresses a recognised
// bucket configuration subresource
func hasBucketSubresource(query url.Values) bool {
	for _, sub := range bucketSubresources {
		if query.Has(sub) {
			return true
		}
	}
	return false
}

// handleBucketOperation handles bucket-level operations
func (h *Handler) handleBucketOperation(w http.ResponseWriter, r *http.Request, bucket string) {
	query := r.URL.Query()

	// Check for multipart uploads listing
	if r.Method == http.MethodGet && query.Has("uploads") {
		h.listMultipartUploads(w, r, bucket)
		return
	}

	// Check for batch delete
	if r.Method == http.MethodPost && query.Has("delete") {
		if h.readOnly {
			writeError(w, "AccessDenied", "Read-only mode", http.StatusForbidden)
			return
		}
		h.deleteObjects(w, r, bucket)
		return
	}

	// Bucket configuration subresources
	if hasBucketSubresource(query) {
		h.handleBucketSubresource(w, r, bucket)
		return
	}

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
	query := r.URL.Query()

	// Handle multipart upload operations
	if uploadID := query.Get("uploadId"); uploadID != "" {
		if h.readOnly {
			writeError(w, "AccessDenied", "Read-only mode", http.StatusForbidden)
			return
		}

		switch r.Method {
		case http.MethodGet:
			// List parts
			h.listParts(w, r, bucket, key, uploadID)
		case http.MethodPut:
			// Upload part (from the request body or copied from an existing object)
			if partNumber := query.Get("partNumber"); partNumber != "" {
				if r.Header.Get("x-amz-copy-source") != "" {
					h.uploadPartCopy(w, r, bucket, key, uploadID, partNumber)
				} else {
					h.uploadPart(w, r, bucket, key, uploadID, partNumber)
				}
			} else {
				writeError(w, "InvalidRequest", "partNumber is required", http.StatusBadRequest)
			}
		case http.MethodPost:
			// Complete multipart upload
			h.completeMultipartUpload(w, r, bucket, key, uploadID)
		case http.MethodDelete:
			// Abort multipart upload
			h.abortMultipartUpload(w, r, bucket, key, uploadID)
		default:
			writeError(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Initiate multipart upload
	if query.Has("uploads") {
		if h.readOnly {
			writeError(w, "AccessDenied", "Read-only mode", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodPost {
			h.initiateMultipartUpload(w, r, bucket, key)
		} else {
			writeError(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Object subresources (?acl, ?tagging): served as stubs so clients don't
	// corrupt object data through the plain PUT path
	if query.Has("acl") || query.Has("tagging") {
		h.handleObjectSubresource(w, r, bucket, key)
		return
	}

	// Standard object operations
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
		if r.Header.Get("x-amz-copy-source") != "" {
			h.copyObject(w, r, bucket, key)
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

// handleBucketSubresource handles requests addressing bucket configuration
// subresources (?location, ?versioning, ?acl, ?lifecycle, ...)
func (h *Handler) handleBucketSubresource(w http.ResponseWriter, r *http.Request, bucket string) {
	if err := h.storage.HeadBucket(bucket); err != nil {
		writeError(w, "NoSuchBucket", "The specified bucket does not exist", http.StatusNotFound)
		return
	}

	query := r.URL.Query()

	switch r.Method {
	case http.MethodGet:
		switch {
		case query.Has("location"):
			// Empty value means us-east-1, matching AWS
			writeXML(w, LocationConstraint{}, http.StatusOK)
		case query.Has("versioning"):
			writeXML(w, VersioningConfiguration{}, http.StatusOK)
		case query.Has("acl"):
			writeXML(w, ownerFullControlACL(), http.StatusOK)
		case query.Has("tagging"):
			writeError(w, "NoSuchTagSet", "The TagSet does not exist", http.StatusNotFound)
		case query.Has("lifecycle"):
			writeError(w, "NoSuchLifecycleConfiguration", "The lifecycle configuration does not exist", http.StatusNotFound)
		case query.Has("cors"):
			writeError(w, "NoSuchCORSConfiguration", "The CORS configuration does not exist", http.StatusNotFound)
		case query.Has("policy"):
			writeError(w, "NoSuchBucketPolicy", "The bucket policy does not exist", http.StatusNotFound)
		case query.Has("encryption"):
			writeError(w, "ServerSideEncryptionConfigurationNotFoundError", "The server side encryption configuration was not found", http.StatusNotFound)
		case query.Has("object-lock"):
			writeError(w, "ObjectLockConfigurationNotFoundError", "Object Lock configuration does not exist for this bucket", http.StatusNotFound)
		default:
			writeError(w, "NotImplemented", "This bucket subresource is not implemented", http.StatusNotImplemented)
		}
	case http.MethodPut:
		if h.readOnly {
			writeError(w, "AccessDenied", "Read-only mode", http.StatusForbidden)
			return
		}
		// Accept configuration writes as no-ops
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		if h.readOnly {
			writeError(w, "AccessDenied", "Read-only mode", http.StatusForbidden)
			return
		}
		// Accept configuration deletes as no-ops (this must not delete the bucket)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ownerFullControlACL is the stub ACL granting the s3dir owner full control
func ownerFullControlACL() AccessControlPolicy {
	return AccessControlPolicy{
		Owner: Owner{ID: "s3dir", DisplayName: "s3dir"},
		AccessControlList: AccessControlList{
			Grants: []Grant{{
				Grantee: Grantee{
					XMLNSXSI:    "http://www.w3.org/2001/XMLSchema-instance",
					Type:        "CanonicalUser",
					ID:          "s3dir",
					DisplayName: "s3dir",
				},
				Permission: "FULL_CONTROL",
			}},
		},
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
			CreationDate: time.Now().UTC().Format(time.RFC3339),
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

// listObjects lists objects in a bucket, handling both ListObjects (v1) and
// ListObjectsV2 requests
func (h *Handler) listObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	query := r.URL.Query()
	prefix := query.Get("prefix")
	delimiter := query.Get("delimiter")
	maxKeys := 1000
	if mk := query.Get("max-keys"); mk != "" {
		if n, err := strconv.Atoi(mk); err == nil && n >= 0 {
			maxKeys = n
		}
	}

	// Determine where to resume from. V1 uses marker; V2 uses an opaque
	// continuation token (base64 of the last returned entry), falling back to
	// start-after on the first page
	listV2 := query.Get("list-type") == "2"
	marker := query.Get("marker")
	continuationToken := query.Get("continuation-token")
	startAfter := query.Get("start-after")
	if listV2 {
		marker = startAfter
		if continuationToken != "" {
			decoded, err := base64.StdEncoding.DecodeString(continuationToken)
			if err != nil {
				writeError(w, "InvalidArgument", "The continuation token provided is incorrect", http.StatusBadRequest)
				return
			}
			marker = string(decoded)
		}
	}

	objects, commonPrefixes, truncated, nextMarker, err := h.storage.ListObjectsPage(bucket, prefix, delimiter, marker, maxKeys)
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
			LastModified: obj.LastModified.UTC().Format(time.RFC3339),
			ETag:         obj.ETag,
			Size:         obj.Size,
			StorageClass: "STANDARD",
		})
	}

	var prefixes []CommonPrefix
	for _, cp := range commonPrefixes {
		prefixes = append(prefixes, CommonPrefix{Prefix: cp})
	}

	if listV2 {
		response := ListObjectsV2Response{
			Name:              bucket,
			Prefix:            prefix,
			Delimiter:         delimiter,
			StartAfter:        startAfter,
			ContinuationToken: continuationToken,
			KeyCount:          len(contents) + len(prefixes),
			MaxKeys:           maxKeys,
			IsTruncated:       truncated,
			Contents:          contents,
			CommonPrefixes:    prefixes,
		}
		if truncated {
			response.NextContinuationToken = base64.StdEncoding.EncodeToString([]byte(nextMarker))
		}
		writeXML(w, response, http.StatusOK)
		return
	}

	response := ListObjectsResponse{
		Name:           bucket,
		Prefix:         prefix,
		Delimiter:      delimiter,
		Marker:         marker,
		NextMarker:     nextMarker,
		MaxKeys:        maxKeys,
		IsTruncated:    truncated,
		Contents:       contents,
		CommonPrefixes: prefixes,
	}

	writeXML(w, response, http.StatusOK)
}

// setObjectHeaders sets the standard response headers for an object
func setObjectHeaders(w http.ResponseWriter, info *storage.ObjectInfo) {
	contentType := info.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("ETag", info.ETag)
	w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("Accept-Ranges", "bytes")
	for name, value := range info.UserMetadata {
		w.Header().Set("x-amz-meta-"+name, value)
	}
}

// checkNotModified evaluates the If-None-Match and If-Modified-Since request
// headers against the object, reporting whether a 304 should be returned.
// If-None-Match takes precedence when present, per RFC 9110
func checkNotModified(r *http.Request, info *storage.ObjectInfo) bool {
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		if inm == "*" {
			return true
		}
		for _, candidate := range strings.Split(inm, ",") {
			candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
			if candidate == info.ETag {
				return true
			}
		}
		return false
	}

	if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if t, err := http.ParseTime(ims); err == nil {
			// HTTP dates have second precision
			return !info.LastModified.Truncate(time.Second).After(t)
		}
	}

	return false
}

// parseRangeHeader parses a single Range request header against an object of
// the given size. valid is false for headers that should be ignored (malformed
// or multi-range); satisfiable is false when a valid range lies outside the
// object and a 416 must be returned
func parseRangeHeader(value string, size int64) (start, length int64, valid, satisfiable bool) {
	spec, ok := strings.CutPrefix(value, "bytes=")
	if !ok || strings.Contains(spec, ",") {
		return 0, 0, false, false
	}

	startStr, endStr, ok := strings.Cut(spec, "-")
	if !ok {
		return 0, 0, false, false
	}

	if startStr == "" {
		// Suffix range: last N bytes
		suffix, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || suffix < 0 {
			return 0, 0, false, false
		}
		if suffix == 0 || size == 0 {
			return 0, 0, true, false
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, suffix, true, true
	}

	first, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil || first < 0 {
		return 0, 0, false, false
	}

	last := size - 1
	if endStr != "" {
		last, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil || last < first {
			return 0, 0, false, false
		}
		if last > size-1 {
			last = size - 1
		}
	}

	if first >= size {
		return 0, 0, true, false
	}

	return first, last - first + 1, true, true
}

// getObject retrieves an object, honouring Range and conditional request
// headers
func (h *Handler) getObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	info, err := h.storage.HeadObject(bucket, key)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchKey", "The specified key does not exist", http.StatusNotFound)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if checkNotModified(r, info) {
		w.Header().Set("ETag", info.ETag)
		w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Ranged read
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		start, length, valid, satisfiable := parseRangeHeader(rangeHeader, info.Size)
		if valid {
			if !satisfiable {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", info.Size))
				writeError(w, "InvalidRange", "The requested range is not satisfiable", http.StatusRequestedRangeNotSatisfiable)
				return
			}

			reader, _, err := h.storage.GetObjectRange(bucket, key, start, length)
			if err != nil {
				writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
				return
			}
			defer reader.Close()

			setObjectHeaders(w, info)
			w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, start+length-1, info.Size))
			w.WriteHeader(http.StatusPartialContent)

			if _, err := io.Copy(w, reader); err != nil {
				// Error writing to response - headers are already sent
				return
			}
			return
		}
		// Malformed range headers are ignored and the full object returned
	}

	reader, _, err := h.storage.GetObject(bucket, key)
	if err != nil {
		writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	setObjectHeaders(w, info)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))

	// Copy object data to response
	if _, err := io.Copy(w, reader); err != nil {
		// Error writing to response - log but can't send error to client
		// since headers are already sent
		return
	}
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

	if checkNotModified(r, info) {
		w.Header().Set("ETag", info.ETag)
		w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusNotModified)
		return
	}

	setObjectHeaders(w, info)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.WriteHeader(http.StatusOK)
}

// userMetadataFromHeader extracts x-amz-meta-* headers into a metadata map
// with lowercased names
func userMetadataFromHeader(header http.Header) map[string]string {
	var meta map[string]string
	for name, values := range header {
		if strings.HasPrefix(name, "X-Amz-Meta-") && len(values) > 0 {
			if meta == nil {
				meta = make(map[string]string)
			}
			meta[strings.ToLower(strings.TrimPrefix(name, "X-Amz-Meta-"))] = values[0]
		}
	}
	return meta
}

// putObject stores an object
func (h *Handler) putObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	contentLength := r.ContentLength
	if contentLength < 0 {
		writeError(w, "MissingContentLength", "Content-Length header is required", http.StatusLengthRequired)
		return
	}

	etag, err := h.storage.PutObjectWithMetadata(bucket, key, r.Body, contentLength,
		r.Header.Get("Content-Type"), userMetadataFromHeader(r.Header))
	if err != nil {
		writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

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

	if _, err := w.Write([]byte(xml.Header)); err != nil {
		return
	}
	if _, err := w.Write(output); err != nil {
		return
	}
}

// writeError writes an S3 error response
func writeError(w http.ResponseWriter, code, message string, statusCode int) {
	errorResponse := ErrorResponse{
		Code:    code,
		Message: message,
	}

	writeXML(w, errorResponse, statusCode)
}

// handleObjectSubresource handles requests addressing object subresources
// (?acl, ?tagging)
func (h *Handler) handleObjectSubresource(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if _, err := h.storage.HeadObject(bucket, key); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchKey", "The specified key does not exist", http.StatusNotFound)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	query := r.URL.Query()

	switch r.Method {
	case http.MethodGet:
		if query.Has("acl") {
			writeXML(w, ownerFullControlACL(), http.StatusOK)
		} else {
			// Objects have no tags; an empty TagSet is the AWS response
			writeXML(w, Tagging{}, http.StatusOK)
		}
	case http.MethodPut:
		if h.readOnly {
			writeError(w, "AccessDenied", "Read-only mode", http.StatusForbidden)
			return
		}
		// Accept ACL and tagging writes as no-ops
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		if h.readOnly {
			writeError(w, "AccessDenied", "Read-only mode", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, "MethodNotAllowed", "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// parseCopySource parses an x-amz-copy-source header value into bucket and key.
// The value is "/{bucket}/{key}" or "{bucket}/{key}" and may be URL-encoded
func parseCopySource(source string) (bucket, key string, err error) {
	// Strip any query string (e.g. ?versionId=...)
	if idx := strings.Index(source, "?"); idx != -1 {
		source = source[:idx]
	}

	decoded, err := url.PathUnescape(source)
	if err != nil {
		return "", "", fmt.Errorf("invalid copy source encoding")
	}

	decoded = strings.TrimPrefix(decoded, "/")
	parts := strings.SplitN(decoded, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("copy source must be of the form /bucket/key")
	}

	return parts[0], parts[1], nil
}

// parseCopySourceRange parses an x-amz-copy-source-range header value of the
// form "bytes=start-end" (inclusive)
func parseCopySourceRange(value string) (start, end int64, err error) {
	spec, ok := strings.CutPrefix(value, "bytes=")
	if !ok {
		return 0, 0, fmt.Errorf("range must be of the form bytes=start-end")
	}

	startStr, endStr, ok := strings.Cut(spec, "-")
	if !ok {
		return 0, 0, fmt.Errorf("range must be of the form bytes=start-end")
	}

	start, err = strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 {
		return 0, 0, fmt.Errorf("invalid range start")
	}

	end, err = strconv.ParseInt(endStr, 10, 64)
	if err != nil || end < start {
		return 0, 0, fmt.Errorf("invalid range end")
	}

	return start, end, nil
}

// copyObject copies an object server-side
func (h *Handler) copyObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	srcBucket, srcKey, err := parseCopySource(r.Header.Get("x-amz-copy-source"))
	if err != nil {
		writeError(w, "InvalidArgument", err.Error(), http.StatusBadRequest)
		return
	}

	replaceMetadata := strings.EqualFold(r.Header.Get("x-amz-metadata-directive"), "REPLACE")
	info, err := h.storage.CopyObjectWithMetadata(srcBucket, srcKey, bucket, key,
		replaceMetadata, r.Header.Get("Content-Type"), userMetadataFromHeader(r.Header))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchKey", "The specified key does not exist", http.StatusNotFound)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	response := CopyObjectResult{
		LastModified: info.LastModified.UTC().Format(time.RFC3339),
		ETag:         info.ETag,
	}

	writeXML(w, response, http.StatusOK)
}

// uploadPartCopy copies data from an existing object into a part of a multipart upload
func (h *Handler) uploadPartCopy(w http.ResponseWriter, r *http.Request, bucket, key, uploadID, partNumberStr string) {
	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil || partNumber < 1 || partNumber > 10000 {
		writeError(w, "InvalidArgument", "Invalid part number", http.StatusBadRequest)
		return
	}

	srcBucket, srcKey, err := parseCopySource(r.Header.Get("x-amz-copy-source"))
	if err != nil {
		writeError(w, "InvalidArgument", err.Error(), http.StatusBadRequest)
		return
	}

	// No range means copy the whole source object
	rangeStart, rangeEnd := int64(-1), int64(-1)
	if rangeHeader := r.Header.Get("x-amz-copy-source-range"); rangeHeader != "" {
		rangeStart, rangeEnd, err = parseCopySourceRange(rangeHeader)
		if err != nil {
			writeError(w, "InvalidArgument", err.Error(), http.StatusBadRequest)
			return
		}
	}

	etag, err := h.storage.UploadPartCopy(uploadID, partNumber, srcBucket, srcKey, rangeStart, rangeEnd)
	if err != nil {
		if strings.Contains(err.Error(), "upload not found") {
			writeError(w, "NoSuchUpload", "The specified upload does not exist", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchKey", "The specified key does not exist", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "invalid range") {
			writeError(w, "InvalidRange", "The requested range is not satisfiable", http.StatusRequestedRangeNotSatisfiable)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	response := CopyPartResult{
		LastModified: time.Now().UTC().Format(time.RFC3339),
		ETag:         etag,
	}

	writeXML(w, response, http.StatusOK)
}

// deleteObjects deletes multiple objects in a single request
func (h *Handler) deleteObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	var deleteRequest Delete
	if err := xml.NewDecoder(r.Body).Decode(&deleteRequest); err != nil {
		writeError(w, "MalformedXML", "Invalid XML", http.StatusBadRequest)
		return
	}

	var response DeleteResult
	for _, obj := range deleteRequest.Objects {
		// Deleting a nonexistent key counts as success (AWS semantics)
		if err := h.storage.DeleteObject(bucket, obj.Key); err != nil {
			response.Errors = append(response.Errors, DeleteError{
				Key:     obj.Key,
				Code:    "InternalError",
				Message: err.Error(),
			})
			continue
		}
		if !deleteRequest.Quiet {
			response.Deleted = append(response.Deleted, DeletedObject(obj))
		}
	}

	writeXML(w, response, http.StatusOK)
}

// initiateMultipartUpload initiates a multipart upload
func (h *Handler) initiateMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key string) {
	uploadID, err := h.storage.InitiateMultipartUploadWithMetadata(bucket, key,
		r.Header.Get("Content-Type"), userMetadataFromHeader(r.Header))
	if err != nil {
		writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		return
	}

	response := InitiateMultipartUploadResult{
		Bucket:   bucket,
		Key:      key,
		UploadID: uploadID,
	}

	writeXML(w, response, http.StatusOK)
}

// uploadPart uploads a part of a multipart upload
func (h *Handler) uploadPart(w http.ResponseWriter, r *http.Request, bucket, key, uploadID, partNumberStr string) {
	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil || partNumber < 1 || partNumber > 10000 {
		writeError(w, "InvalidArgument", "Invalid part number", http.StatusBadRequest)
		return
	}

	contentLength := r.ContentLength
	if contentLength < 0 {
		writeError(w, "MissingContentLength", "Content-Length header is required", http.StatusLengthRequired)
		return
	}

	etag, err := h.storage.UploadPart(uploadID, partNumber, r.Body, contentLength)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchUpload", "The specified upload does not exist", http.StatusNotFound)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
}

// completeMultipartUpload completes a multipart upload
func (h *Handler) completeMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	// Parse the request body
	var complete CompleteMultipartUpload
	if err := xml.NewDecoder(r.Body).Decode(&complete); err != nil {
		writeError(w, "MalformedXML", "Invalid XML", http.StatusBadRequest)
		return
	}

	// Convert parts to storage.CompletePart slice
	parts := make([]storage.CompletePart, len(complete.Parts))
	for i, part := range complete.Parts {
		parts[i] = storage.CompletePart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		}
	}

	etag, err := h.storage.CompleteMultipartUpload(uploadID, parts)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchUpload", "The specified upload does not exist", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "part") {
			writeError(w, "InvalidPart", err.Error(), http.StatusBadRequest)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	response := CompleteMultipartUploadResult{
		Location: fmt.Sprintf("/%s/%s", bucket, key),
		Bucket:   bucket,
		Key:      key,
		ETag:     etag,
	}

	writeXML(w, response, http.StatusOK)
}

// abortMultipartUpload aborts a multipart upload
func (h *Handler) abortMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	if err := h.storage.AbortMultipartUpload(uploadID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchUpload", "The specified upload does not exist", http.StatusNotFound)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// listParts lists the parts of a multipart upload
func (h *Handler) listParts(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	parts, err := h.storage.ListMultipartUploadParts(uploadID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "NoSuchUpload", "The specified upload does not exist", http.StatusNotFound)
		} else {
			writeError(w, "InternalError", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	var partsList []Part
	for _, p := range parts {
		partsList = append(partsList, Part{
			PartNumber:   p.PartNumber,
			LastModified: p.LastModified.UTC().Format(time.RFC3339),
			ETag:         p.ETag,
			Size:         p.Size,
		})
	}

	response := ListPartsResult{
		Bucket:   bucket,
		Key:      key,
		UploadID: uploadID,
		Initiator: Initiator{
			ID:          "s3dir",
			DisplayName: "s3dir",
		},
		Owner: Owner{
			ID:          "s3dir",
			DisplayName: "s3dir",
		},
		StorageClass:         "STANDARD",
		PartNumberMarker:     0,
		NextPartNumberMarker: 0,
		MaxParts:             1000,
		IsTruncated:          false,
		Parts:                partsList,
	}

	writeXML(w, response, http.StatusOK)
}

// listMultipartUploads lists multipart uploads
func (h *Handler) listMultipartUploads(w http.ResponseWriter, r *http.Request, bucket string) {
	uploads := h.storage.ListMultipartUploads(bucket)

	var uploadsList []Upload
	for _, u := range uploads {
		uploadsList = append(uploadsList, Upload{
			Key:      u.Key,
			UploadID: u.UploadID,
			Initiator: Initiator{
				ID:          "s3dir",
				DisplayName: "s3dir",
			},
			Owner: Owner{
				ID:          "s3dir",
				DisplayName: "s3dir",
			},
			StorageClass: "STANDARD",
			Initiated:    u.Initiated.UTC().Format(time.RFC3339),
		})
	}

	response := ListMultipartUploadsResult{
		Bucket:             bucket,
		KeyMarker:          "",
		UploadIDMarker:     "",
		NextKeyMarker:      "",
		NextUploadIDMarker: "",
		MaxUploads:         1000,
		IsTruncated:        false,
		Uploads:            uploadsList,
	}

	writeXML(w, response, http.StatusOK)
}
