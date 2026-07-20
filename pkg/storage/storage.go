package storage

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ObjectInfo contains metadata about an object
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	ContentType  string
	UserMetadata map[string]string
}

// Storage provides filesystem-based storage for S3 objects
type Storage struct {
	baseDir   string
	multipart *MultipartManager
}

// New creates a new Storage instance
func New(baseDir string) (*Storage, error) {
	absPath, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &Storage{
		baseDir:   absPath,
		multipart: NewMultipartManager(absPath),
	}, nil
}

// PutObject stores an object
func (s *Storage) PutObject(bucket, key string, reader io.Reader, size int64) error {
	_, err := s.PutObjectWithMetadata(bucket, key, reader, size, "", nil)
	return err
}

// PutObjectWithMetadata stores an object along with its content type and user
// metadata, returning the quoted MD5 ETag of the content
func (s *Storage) PutObjectWithMetadata(bucket, key string, reader io.Reader, size int64, contentType string, userMetadata map[string]string) (string, error) {
	objectPath := s.objectPath(bucket, key)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(objectPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp(filepath.Dir(objectPath), ".s3dir-tmp-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Copy data to temporary file using a fixed-size buffer to limit memory usage,
	// calculating the content MD5 in the same pass
	// This ensures we stream data in 32KB chunks rather than allocating large buffers
	hash := md5.New()
	buffer := make([]byte, 32*1024) // 32KB buffer
	_, err = io.CopyBuffer(io.MultiWriter(tmpFile, hash), reader, buffer)
	closeErr := tmpFile.Close()
	if err != nil {
		return "", fmt.Errorf("failed to write object: %w", err)
	}
	if closeErr != nil {
		return "", fmt.Errorf("failed to close temporary file: %w", closeErr)
	}

	// Move temporary file to final location
	if err := os.Rename(tmpPath, objectPath); err != nil {
		return "", fmt.Errorf("failed to move object: %w", err)
	}

	etag := hex.EncodeToString(hash.Sum(nil))
	meta := &objectMetadata{
		ETag:         etag,
		ContentType:  contentType,
		UserMetadata: userMetadata,
	}
	if err := writeObjectMetadataFile(s.baseDir, bucket, key, meta); err != nil {
		return "", fmt.Errorf("failed to write metadata: %w", err)
	}

	return fmt.Sprintf("\"%s\"", etag), nil
}

// GetObject retrieves an object
func (s *Storage) GetObject(bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	objectPath := s.objectPath(bucket, key)

	stat, err := os.Stat(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("object not found")
		}
		return nil, nil, fmt.Errorf("failed to stat object: %w", err)
	}

	if stat.IsDir() {
		return nil, nil, fmt.Errorf("cannot get directory as object")
	}

	file, err := os.Open(objectPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open object: %w", err)
	}

	return file, s.objectInfo(bucket, key, stat), nil
}

// GetObjectRange retrieves a byte range of an object. start is the first byte
// offset and length the number of bytes to read
func (s *Storage) GetObjectRange(bucket, key string, start, length int64) (io.ReadCloser, *ObjectInfo, error) {
	objectPath := s.objectPath(bucket, key)

	stat, err := os.Stat(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("object not found")
		}
		return nil, nil, fmt.Errorf("failed to stat object: %w", err)
	}

	if stat.IsDir() {
		return nil, nil, fmt.Errorf("cannot get directory as object")
	}

	file, err := os.Open(objectPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open object: %w", err)
	}

	if _, err := file.Seek(start, io.SeekStart); err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("failed to seek object: %w", err)
	}

	reader := &rangeReadCloser{
		Reader: io.LimitReader(file, length),
		file:   file,
	}

	return reader, s.objectInfo(bucket, key, stat), nil
}

// rangeReadCloser wraps a limited reader over an open file so the file is
// closed when the caller finishes reading the range
type rangeReadCloser struct {
	io.Reader
	file *os.File
}

func (r *rangeReadCloser) Close() error {
	return r.file.Close()
}

// objectInfo builds an ObjectInfo from a file stat plus the metadata sidecar,
// falling back to a modification-time ETag for objects without a sidecar
func (s *Storage) objectInfo(bucket, key string, stat os.FileInfo) *ObjectInfo {
	info := &ObjectInfo{
		Key:          key,
		Size:         stat.Size(),
		LastModified: stat.ModTime(),
	}

	if meta := readObjectMetadataFile(s.baseDir, bucket, key); meta != nil {
		if meta.ETag != "" {
			info.ETag = fmt.Sprintf("\"%s\"", meta.ETag)
		}
		info.ContentType = meta.ContentType
		info.UserMetadata = meta.UserMetadata
	}

	if info.ETag == "" {
		info.ETag = fmt.Sprintf("\"%x\"", stat.ModTime().Unix())
	}

	return info
}

// CopyObject copies an object server-side, streaming the data through a
// fixed-size buffer while calculating the MD5 of the content. The source
// object's content type and user metadata are carried over
func (s *Storage) CopyObject(srcBucket, srcKey, dstBucket, dstKey string) (*ObjectInfo, error) {
	return s.CopyObjectWithMetadata(srcBucket, srcKey, dstBucket, dstKey, false, "", nil)
}

// CopyObjectWithMetadata copies an object server-side. When replaceMetadata is
// true the given content type and user metadata are stored on the destination
// instead of the source object's metadata
func (s *Storage) CopyObjectWithMetadata(srcBucket, srcKey, dstBucket, dstKey string, replaceMetadata bool, contentType string, userMetadata map[string]string) (*ObjectInfo, error) {
	reader, srcInfo, err := s.GetObject(srcBucket, srcKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	dstPath := s.objectPath(dstBucket, dstKey)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp(filepath.Dir(dstPath), ".s3dir-tmp-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Copy data while calculating MD5, using a fixed-size buffer to limit memory usage
	hash := md5.New()
	buffer := make([]byte, 32*1024) // 32KB buffer
	written, err := io.CopyBuffer(io.MultiWriter(tmpFile, hash), reader, buffer)
	closeErr := tmpFile.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to copy object: %w", err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("failed to close temporary file: %w", closeErr)
	}

	// Move temporary file to final location
	if err := os.Rename(tmpPath, dstPath); err != nil {
		return nil, fmt.Errorf("failed to move object: %w", err)
	}

	stat, err := os.Stat(dstPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat object: %w", err)
	}

	etag := hex.EncodeToString(hash.Sum(nil))
	meta := &objectMetadata{
		ETag:         etag,
		ContentType:  contentType,
		UserMetadata: userMetadata,
	}
	if !replaceMetadata {
		// Carry over the source object's metadata (S3 COPY directive)
		meta.ContentType = srcInfo.ContentType
		meta.UserMetadata = srcInfo.UserMetadata
	}
	if err := writeObjectMetadataFile(s.baseDir, dstBucket, dstKey, meta); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	return &ObjectInfo{
		Key:          dstKey,
		Size:         written,
		LastModified: stat.ModTime(),
		ETag:         fmt.Sprintf("\"%s\"", etag),
		ContentType:  meta.ContentType,
		UserMetadata: meta.UserMetadata,
	}, nil
}

// DeleteObject deletes an object
func (s *Storage) DeleteObject(bucket, key string) error {
	objectPath := s.objectPath(bucket, key)

	err := os.Remove(objectPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	removeObjectMetadataFile(s.baseDir, bucket, key)

	// Clean up empty parent directories
	s.cleanupEmptyDirs(filepath.Dir(objectPath), s.bucketPath(bucket))
	metadataBucketDir := filepath.Join(s.baseDir, metadataDirName, bucket)
	s.cleanupEmptyDirs(filepath.Dir(objectMetadataPath(s.baseDir, bucket, key)), metadataBucketDir)

	return nil
}

// HeadObject retrieves object metadata
func (s *Storage) HeadObject(bucket, key string) (*ObjectInfo, error) {
	objectPath := s.objectPath(bucket, key)

	stat, err := os.Stat(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object not found")
		}
		return nil, fmt.Errorf("failed to stat object: %w", err)
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("cannot head directory as object")
	}

	return s.objectInfo(bucket, key, stat), nil
}

// ListObjects lists objects in a bucket with optional prefix and delimiter
func (s *Storage) ListObjects(bucket, prefix, delimiter string, maxKeys int) ([]ObjectInfo, []string, error) {
	objects, commonPrefixes, _, _, err := s.ListObjectsPage(bucket, prefix, delimiter, "", maxKeys)
	return objects, commonPrefixes, err
}

// listEntry is a single result of a listing: either an object or a rolled-up
// common prefix
type listEntry struct {
	name     string
	isPrefix bool
	stat     os.FileInfo
}

// ListObjectsPage lists objects in a bucket in lexicographic key order,
// returning entries strictly after marker, up to maxKeys objects and common
// prefixes combined (maxKeys <= 0 means unlimited). It reports whether the
// listing was truncated and the marker to resume from
func (s *Storage) ListObjectsPage(bucket, prefix, delimiter, marker string, maxKeys int) ([]ObjectInfo, []string, bool, string, error) {
	bucketPath := s.bucketPath(bucket)

	if _, err := os.Stat(bucketPath); os.IsNotExist(err) {
		return nil, nil, false, "", fmt.Errorf("bucket not found")
	}

	// Collect all keys matching the prefix
	var keys []listEntry
	err := filepath.Walk(bucketPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if path == bucketPath {
			return nil
		}

		// Get relative path from bucket
		relPath, err := filepath.Rel(bucketPath, path)
		if err != nil {
			return nil
		}

		// Convert to S3-style key (forward slashes)
		key := filepath.ToSlash(relPath)

		if info.IsDir() {
			// Skip subtrees that cannot contain keys with the prefix
			if prefix != "" && !strings.HasPrefix(key+"/", prefix) && !strings.HasPrefix(prefix, key+"/") {
				return filepath.SkipDir
			}
			return nil
		}

		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}

		keys = append(keys, listEntry{name: key, stat: info})
		return nil
	})
	if err != nil {
		return nil, nil, false, "", fmt.Errorf("failed to list objects: %w", err)
	}

	// S3 listings are in lexicographic key order; filesystem walk order is not
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].name < keys[j].name
	})

	// Roll up keys containing the delimiter into common prefixes. Keys sharing
	// a common prefix are contiguous in sorted order, so deduplicating against
	// the previous entry is sufficient
	var entries []listEntry
	for _, k := range keys {
		if delimiter != "" {
			remainder := strings.TrimPrefix(k.name, prefix)
			if idx := strings.Index(remainder, delimiter); idx != -1 {
				commonPrefix := prefix + remainder[:idx+len(delimiter)]
				if len(entries) > 0 && entries[len(entries)-1].isPrefix && entries[len(entries)-1].name == commonPrefix {
					continue
				}
				entries = append(entries, listEntry{name: commonPrefix, isPrefix: true})
				continue
			}
		}
		entries = append(entries, k)
	}

	// Resume strictly after the marker
	if marker != "" {
		start := sort.Search(len(entries), func(i int) bool {
			return entries[i].name > marker
		})
		entries = entries[start:]
	}

	truncated := maxKeys > 0 && len(entries) > maxKeys
	if truncated {
		entries = entries[:maxKeys]
	}

	nextMarker := ""
	if truncated {
		nextMarker = entries[len(entries)-1].name
	}

	// Build results, reading metadata sidecars only for the returned page
	var objects []ObjectInfo
	var commonPrefixes []string
	for _, e := range entries {
		if e.isPrefix {
			commonPrefixes = append(commonPrefixes, e.name)
		} else {
			objects = append(objects, *s.objectInfo(bucket, e.name, e.stat))
		}
	}

	return objects, commonPrefixes, truncated, nextMarker, nil
}

// CreateBucket creates a new bucket (directory)
func (s *Storage) CreateBucket(bucket string) error {
	bucketPath := s.bucketPath(bucket)

	if _, err := os.Stat(bucketPath); err == nil {
		return fmt.Errorf("bucket already exists")
	}

	if err := os.MkdirAll(bucketPath, 0755); err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	return nil
}

// DeleteBucket deletes a bucket (directory)
func (s *Storage) DeleteBucket(bucket string) error {
	bucketPath := s.bucketPath(bucket)

	entries, err := os.ReadDir(bucketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("bucket not found")
		}
		return fmt.Errorf("failed to read bucket: %w", err)
	}

	if len(entries) > 0 {
		return fmt.Errorf("bucket not empty")
	}

	if err := os.Remove(bucketPath); err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	// Remove any leftover metadata sidecars for the bucket
	os.RemoveAll(filepath.Join(s.baseDir, metadataDirName, bucket))

	return nil
}

// HeadBucket checks if a bucket exists
func (s *Storage) HeadBucket(bucket string) error {
	bucketPath := s.bucketPath(bucket)

	stat, err := os.Stat(bucketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("bucket not found")
		}
		return fmt.Errorf("failed to stat bucket: %w", err)
	}

	if !stat.IsDir() {
		return fmt.Errorf("bucket is not a directory")
	}

	return nil
}

// ListBuckets lists all buckets (top-level directories)
func (s *Storage) ListBuckets() ([]string, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read base directory: %w", err)
	}

	var buckets []string
	for _, entry := range entries {
		// Skip internal directories (.multipart, .metadata)
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			buckets = append(buckets, entry.Name())
		}
	}

	return buckets, nil
}

// bucketPath returns the filesystem path for a bucket
func (s *Storage) bucketPath(bucket string) string {
	return filepath.Join(s.baseDir, bucket)
}

// objectPath returns the filesystem path for an object
func (s *Storage) objectPath(bucket, key string) string {
	return filepath.Join(s.baseDir, bucket, filepath.FromSlash(key))
}

// cleanupEmptyDirs removes empty parent directories up to the stop path
func (s *Storage) cleanupEmptyDirs(path, stopPath string) {
	for path != stopPath && strings.HasPrefix(path, stopPath) {
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) > 0 {
			return
		}
		os.Remove(path)
		path = filepath.Dir(path)
	}
}

// Multipart upload methods

// InitiateMultipartUpload starts a new multipart upload
func (s *Storage) InitiateMultipartUpload(bucket, key string) (string, error) {
	return s.InitiateMultipartUploadWithMetadata(bucket, key, "", nil)
}

// InitiateMultipartUploadWithMetadata starts a new multipart upload, recording
// the content type and user metadata to store on the completed object
func (s *Storage) InitiateMultipartUploadWithMetadata(bucket, key, contentType string, userMetadata map[string]string) (string, error) {
	// Verify bucket exists
	if err := s.HeadBucket(bucket); err != nil {
		return "", err
	}
	return s.multipart.InitiateUpload(bucket, key, contentType, userMetadata)
}

// UploadPart uploads a part of a multipart upload
func (s *Storage) UploadPart(uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	return s.multipart.UploadPart(uploadID, partNumber, reader, size)
}

// UploadPartCopy copies data from an existing object into a part of a
// multipart upload. rangeStart/rangeEnd are inclusive byte offsets; pass
// rangeStart = -1 to copy the whole source object
func (s *Storage) UploadPartCopy(uploadID string, partNumber int, srcBucket, srcKey string, rangeStart, rangeEnd int64) (string, error) {
	srcPath := s.objectPath(srcBucket, srcKey)

	stat, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("object not found")
		}
		return "", fmt.Errorf("failed to stat object: %w", err)
	}
	if stat.IsDir() {
		return "", fmt.Errorf("object not found")
	}

	file, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to open object: %w", err)
	}
	defer file.Close()

	var reader io.Reader = file
	size := stat.Size()
	if rangeStart >= 0 {
		if rangeStart > rangeEnd || rangeEnd >= stat.Size() {
			return "", fmt.Errorf("invalid range")
		}
		if _, err := file.Seek(rangeStart, io.SeekStart); err != nil {
			return "", fmt.Errorf("failed to seek object: %w", err)
		}
		size = rangeEnd - rangeStart + 1
		reader = io.LimitReader(file, size)
	}

	return s.multipart.UploadPart(uploadID, partNumber, reader, size)
}

// CompleteMultipartUpload completes a multipart upload
func (s *Storage) CompleteMultipartUpload(uploadID string, parts []CompletePart) (string, error) {
	return s.multipart.CompleteUpload(uploadID, parts)
}

// AbortMultipartUpload aborts a multipart upload
func (s *Storage) AbortMultipartUpload(uploadID string) error {
	return s.multipart.AbortUpload(uploadID)
}

// ListMultipartUploadParts lists parts of a multipart upload
func (s *Storage) ListMultipartUploadParts(uploadID string) ([]*UploadPart, error) {
	return s.multipart.ListParts(uploadID)
}

// ListMultipartUploads lists in-progress multipart uploads
func (s *Storage) ListMultipartUploads(bucket string) []*MultipartUpload {
	return s.multipart.ListUploads(bucket)
}
