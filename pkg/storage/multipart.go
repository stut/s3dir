package storage

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MultipartUpload represents an in-progress multipart upload
type MultipartUpload struct {
	UploadID     string
	Bucket       string
	Key          string
	Initiated    time.Time
	LastActivity time.Time
	Parts        map[int]*UploadPart
	mu           sync.RWMutex
}

// UploadPart represents a single part of a multipart upload
type UploadPart struct {
	PartNumber   int
	Size         int64
	ETag         string
	Path         string
	LastModified time.Time
}

// MultipartManager manages multipart uploads
type MultipartManager struct {
	uploads       map[string]*MultipartUpload
	baseDir       string
	mu            sync.RWMutex
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// NewMultipartManager creates a new multipart upload manager
func NewMultipartManager(baseDir string) *MultipartManager {
	m := &MultipartManager{
		uploads:     make(map[string]*MultipartUpload),
		baseDir:     baseDir,
		stopCleanup: make(chan struct{}),
	}

	// Clean up orphaned uploads from previous runs on startup
	m.cleanupOrphanedUploads()

	// Start background cleanup goroutine (runs every hour)
	m.cleanupTicker = time.NewTicker(1 * time.Hour)
	go m.backgroundCleanup()

	return m
}

// InitiateUpload starts a new multipart upload
func (m *MultipartManager) InitiateUpload(bucket, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate unique upload ID
	uploadID := generateUploadID()

	now := time.Now()
	upload := &MultipartUpload{
		UploadID:     uploadID,
		Bucket:       bucket,
		Key:          key,
		Initiated:    now,
		LastActivity: now,
		Parts:        make(map[int]*UploadPart),
	}

	m.uploads[uploadID] = upload

	// Create directory for parts
	partsDir := m.getPartsDir(uploadID)
	if err := os.MkdirAll(partsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create parts directory: %w", err)
	}

	// Save upload metadata
	if err := m.saveUploadMetadata(upload); err != nil {
		return "", fmt.Errorf("failed to save metadata: %w", err)
	}

	return uploadID, nil
}

// UploadPart uploads a single part
func (m *MultipartManager) UploadPart(uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	m.mu.RLock()
	upload, exists := m.uploads[uploadID]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("upload not found")
	}

	// Create part file
	partPath := m.getPartPath(uploadID, partNumber)
	partFile, err := os.Create(partPath)
	if err != nil {
		return "", fmt.Errorf("failed to create part file: %w", err)
	}
	defer partFile.Close()

	// Calculate MD5 while writing using a fixed-size buffer to limit memory usage
	hash := md5.New()
	writer := io.MultiWriter(partFile, hash)

	buffer := make([]byte, 32*1024) // 32KB buffer for streaming
	written, err := io.CopyBuffer(writer, reader, buffer)
	if err != nil {
		os.Remove(partPath)
		return "", fmt.Errorf("failed to write part: %w", err)
	}

	etag := fmt.Sprintf("\"%s\"", hex.EncodeToString(hash.Sum(nil)))

	// Store part info
	part := &UploadPart{
		PartNumber:   partNumber,
		Size:         written,
		ETag:         etag,
		Path:         partPath,
		LastModified: time.Now(),
	}

	upload.mu.Lock()
	upload.Parts[partNumber] = part
	upload.LastActivity = time.Now()
	upload.mu.Unlock()

	// Update metadata
	if err := m.saveUploadMetadata(upload); err != nil {
		return "", fmt.Errorf("failed to update metadata: %w", err)
	}

	return etag, nil
}

// CompleteUpload assembles all parts into final object
func (m *MultipartManager) CompleteUpload(uploadID string, parts []CompletePart) (string, error) {
	m.mu.RLock()
	upload, exists := m.uploads[uploadID]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("upload not found")
	}

	// Validate all parts are present
	upload.mu.RLock()
	for _, cp := range parts {
		part, ok := upload.Parts[cp.PartNumber]
		if !ok {
			upload.mu.RUnlock()
			return "", fmt.Errorf("part %d not found", cp.PartNumber)
		}
		if part.ETag != cp.ETag {
			upload.mu.RUnlock()
			return "", fmt.Errorf("part %d etag mismatch", cp.PartNumber)
		}
	}
	upload.mu.RUnlock()

	// Sort parts by part number
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	// Create final object path
	objectPath := filepath.Join(m.baseDir, upload.Bucket, filepath.FromSlash(upload.Key))
	if err := os.MkdirAll(filepath.Dir(objectPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create object directory: %w", err)
	}

	// Create temporary file for assembly
	tmpFile, err := os.CreateTemp(filepath.Dir(objectPath), ".s3dir-multipart-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Assemble parts - use large buffer for faster assembly of large files
	// Use 1MB buffer instead of 32KB to speed up assembly of multi-GB files
	buffer := make([]byte, 1024*1024)

	// Calculate ETag as MD5 of concatenated part ETags (S3 multipart ETag format)
	// This is much faster than hashing the entire assembled file
	hash := md5.New()
	for _, cp := range parts {
		upload.mu.RLock()
		part := upload.Parts[cp.PartNumber]
		upload.mu.RUnlock()

		// Write part's MD5 to hash (for multipart ETag calculation)
		// Extract hex MD5 from ETag (remove quotes)
		partMD5 := strings.Trim(part.ETag, "\"")
		partMD5Bytes, _ := hex.DecodeString(partMD5)
		hash.Write(partMD5Bytes)

		// Copy part data to final file
		partFile, err := os.Open(part.Path)
		if err != nil {
			tmpFile.Close()
			return "", fmt.Errorf("failed to open part %d: %w", cp.PartNumber, err)
		}

		if _, err := io.CopyBuffer(tmpFile, partFile, buffer); err != nil {
			partFile.Close()
			tmpFile.Close()
			return "", fmt.Errorf("failed to copy part %d: %w", cp.PartNumber, err)
		}

		partFile.Close()
	}

	tmpFile.Close()

	// Move to final location
	if err := os.Rename(tmpPath, objectPath); err != nil {
		return "", fmt.Errorf("failed to move object: %w", err)
	}

	// Generate ETag in S3 multipart format: MD5-of-MD5s + part count
	etag := fmt.Sprintf("\"%s-%d\"", hex.EncodeToString(hash.Sum(nil)), len(parts))

	// Cleanup - remove from uploads map and delete parts directory
	m.mu.Lock()
	delete(m.uploads, uploadID)
	m.mu.Unlock()

	// Remove parts directory
	partsDir := m.getPartsDir(uploadID)
	os.RemoveAll(partsDir)

	return etag, nil
}

// AbortUpload cancels a multipart upload and cleans up
func (m *MultipartManager) AbortUpload(uploadID string) error {
	m.mu.Lock()
	_, exists := m.uploads[uploadID]
	if exists {
		delete(m.uploads, uploadID)
	}
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("upload not found")
	}

	// Remove parts directory
	partsDir := m.getPartsDir(uploadID)
	if err := os.RemoveAll(partsDir); err != nil {
		return fmt.Errorf("failed to remove parts: %w", err)
	}

	return nil
}

// ListParts lists uploaded parts
func (m *MultipartManager) ListParts(uploadID string) ([]*UploadPart, error) {
	m.mu.RLock()
	upload, exists := m.uploads[uploadID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("upload not found")
	}

	upload.mu.RLock()
	defer upload.mu.RUnlock()

	parts := make([]*UploadPart, 0, len(upload.Parts))
	for _, part := range upload.Parts {
		parts = append(parts, part)
	}

	// Sort by part number
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	return parts, nil
}

// ListUploads lists in-progress uploads for a bucket
func (m *MultipartManager) ListUploads(bucket string) []*MultipartUpload {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uploads := make([]*MultipartUpload, 0)
	for _, upload := range m.uploads {
		if upload.Bucket == bucket {
			uploads = append(uploads, upload)
		}
	}

	return uploads
}

// CompletePart represents a part in the complete multipart upload request
type CompletePart struct {
	PartNumber int
	ETag       string
}

// Helper functions

func (m *MultipartManager) getPartsDir(uploadID string) string {
	return filepath.Join(m.baseDir, ".multipart", uploadID)
}

func (m *MultipartManager) getPartPath(uploadID string, partNumber int) string {
	return filepath.Join(m.getPartsDir(uploadID), fmt.Sprintf("part-%d", partNumber))
}

func (m *MultipartManager) getMetadataPath(uploadID string) string {
	return filepath.Join(m.getPartsDir(uploadID), "metadata.json")
}

func (m *MultipartManager) saveUploadMetadata(upload *MultipartUpload) error {
	metadataPath := m.getMetadataPath(upload.UploadID)

	// Create a serializable version without the mutex
	// Make a deep copy of the parts map to avoid concurrent access during JSON marshaling
	upload.mu.RLock()
	partsCopy := make(map[int]*UploadPart, len(upload.Parts))
	for k, v := range upload.Parts {
		partsCopy[k] = v
	}
	metadata := struct {
		UploadID     string
		Bucket       string
		Key          string
		Initiated    time.Time
		LastActivity time.Time
		Parts        map[int]*UploadPart
	}{
		UploadID:     upload.UploadID,
		Bucket:       upload.Bucket,
		Key:          upload.Key,
		Initiated:    upload.Initiated,
		LastActivity: upload.LastActivity,
		Parts:        partsCopy,
	}
	upload.mu.RUnlock()

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0644)
}

func generateUploadID() string {
	// Simple upload ID generation
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(16))
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

// backgroundCleanup runs periodically to clean up stale uploads
func (m *MultipartManager) backgroundCleanup() {
	for {
		select {
		case <-m.cleanupTicker.C:
			m.cleanupStaleUploads()
		case <-m.stopCleanup:
			m.cleanupTicker.Stop()
			return
		}
	}
}

// cleanupStaleUploads removes uploads with no activity for more than 24 hours
func (m *MultipartManager) cleanupStaleUploads() {
	const staleThreshold = 24 * time.Hour
	now := time.Now()

	m.mu.Lock()
	var staleUploads []string
	for uploadID, upload := range m.uploads {
		upload.mu.RLock()
		lastActivity := upload.LastActivity
		upload.mu.RUnlock()

		if now.Sub(lastActivity) > staleThreshold {
			staleUploads = append(staleUploads, uploadID)
		}
	}

	// Remove stale uploads from map
	for _, uploadID := range staleUploads {
		delete(m.uploads, uploadID)
	}
	m.mu.Unlock()

	// Clean up filesystem (outside of lock)
	for _, uploadID := range staleUploads {
		partsDir := m.getPartsDir(uploadID)
		os.RemoveAll(partsDir)
	}
}

// cleanupOrphanedUploads removes leftover .multipart directories from previous runs
func (m *MultipartManager) cleanupOrphanedUploads() {
	multipartDir := filepath.Join(m.baseDir, ".multipart")

	// Check if .multipart directory exists
	if _, err := os.Stat(multipartDir); os.IsNotExist(err) {
		return
	}

	// Read all upload directories
	entries, err := os.ReadDir(multipartDir)
	if err != nil {
		return
	}

	// Remove all orphaned upload directories
	// Since this runs at startup, all .multipart directories are orphaned
	for _, entry := range entries {
		if entry.IsDir() {
			uploadDir := filepath.Join(multipartDir, entry.Name())
			os.RemoveAll(uploadDir)
		}
	}
}

// Stop stops the background cleanup goroutine
func (m *MultipartManager) Stop() {
	close(m.stopCleanup)
}
