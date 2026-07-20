package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// metadataDirName is the directory under baseDir holding object metadata
// sidecar files, mirroring the bucket/key layout
const metadataDirName = ".metadata"

// objectMetadata is persisted as a JSON sidecar for each object. The ETag is
// stored without surrounding quotes
type objectMetadata struct {
	ETag         string            `json:"etag,omitempty"`
	ContentType  string            `json:"contentType,omitempty"`
	UserMetadata map[string]string `json:"userMetadata,omitempty"`
}

func objectMetadataPath(baseDir, bucket, key string) string {
	return filepath.Join(baseDir, metadataDirName, bucket, filepath.FromSlash(key)+".json")
}

// writeObjectMetadataFile persists the metadata sidecar for an object
func writeObjectMetadataFile(baseDir, bucket, key string, meta *objectMetadata) error {
	path := objectMetadataPath(baseDir, bucket, key)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// readObjectMetadataFile loads the metadata sidecar for an object, returning
// nil if no sidecar exists (e.g. objects created before metadata support)
func readObjectMetadataFile(baseDir, bucket, key string) *objectMetadata {
	data, err := os.ReadFile(objectMetadataPath(baseDir, bucket, key))
	if err != nil {
		return nil
	}

	var meta objectMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}

	return &meta
}

// removeObjectMetadataFile deletes the metadata sidecar for an object, if any
func removeObjectMetadataFile(baseDir, bucket, key string) {
	os.Remove(objectMetadataPath(baseDir, bucket, key))
}
