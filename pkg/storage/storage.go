package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
}

// Storage provides filesystem-based storage for S3 objects
type Storage struct {
	baseDir string
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
		baseDir: absPath,
	}, nil
}

// PutObject stores an object
func (s *Storage) PutObject(bucket, key string, reader io.Reader, size int64) error {
	objectPath := s.objectPath(bucket, key)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(objectPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp(filepath.Dir(objectPath), ".s3dir-tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Copy data to temporary file
	_, err = io.Copy(tmpFile, reader)
	closeErr := tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to write object: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close temporary file: %w", closeErr)
	}

	// Move temporary file to final location
	if err := os.Rename(tmpPath, objectPath); err != nil {
		return fmt.Errorf("failed to move object: %w", err)
	}

	return nil
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

	info := &ObjectInfo{
		Key:          key,
		Size:         stat.Size(),
		LastModified: stat.ModTime(),
		ETag:         fmt.Sprintf("\"%x\"", stat.ModTime().Unix()),
	}

	return file, info, nil
}

// DeleteObject deletes an object
func (s *Storage) DeleteObject(bucket, key string) error {
	objectPath := s.objectPath(bucket, key)

	err := os.Remove(objectPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	// Clean up empty parent directories
	s.cleanupEmptyDirs(filepath.Dir(objectPath), s.bucketPath(bucket))

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

	info := &ObjectInfo{
		Key:          key,
		Size:         stat.Size(),
		LastModified: stat.ModTime(),
		ETag:         fmt.Sprintf("\"%x\"", stat.ModTime().Unix()),
	}

	return info, nil
}

// ListObjects lists objects in a bucket with optional prefix and delimiter
func (s *Storage) ListObjects(bucket, prefix, delimiter string, maxKeys int) ([]ObjectInfo, []string, error) {
	bucketPath := s.bucketPath(bucket)

	if _, err := os.Stat(bucketPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("bucket not found")
	}

	var objects []ObjectInfo
	var commonPrefixes []string
	prefixMap := make(map[string]bool)

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

		// Apply prefix filter
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			if !strings.HasPrefix(prefix, key+"/") {
				return filepath.SkipDir
			}
			return nil
		}

		// Handle delimiter
		if delimiter != "" {
			remainder := strings.TrimPrefix(key, prefix)
			if idx := strings.Index(remainder, delimiter); idx != -1 {
				commonPrefix := prefix + remainder[:idx+len(delimiter)]
				if !prefixMap[commonPrefix] {
					prefixMap[commonPrefix] = true
					commonPrefixes = append(commonPrefixes, commonPrefix)
				}
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Add object
		objects = append(objects, ObjectInfo{
			Key:          key,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			ETag:         fmt.Sprintf("\"%x\"", info.ModTime().Unix()),
		})

		// Check max keys limit
		if maxKeys > 0 && len(objects) >= maxKeys {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return nil, nil, fmt.Errorf("failed to list objects: %w", err)
	}

	return objects, commonPrefixes, nil
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
		if entry.IsDir() {
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
