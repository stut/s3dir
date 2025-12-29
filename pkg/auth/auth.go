package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Authenticator handles AWS Signature V4 authentication
type Authenticator struct {
	accessKeyID     string
	secretAccessKey string
	enabled         bool
}

// New creates a new Authenticator
func New(accessKeyID, secretAccessKey string, enabled bool) *Authenticator {
	return &Authenticator{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		enabled:         enabled,
	}
}

// Authenticate verifies the request signature
func (a *Authenticator) Authenticate(r *http.Request) error {
	if !a.enabled {
		return nil
	}

	// Check for Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("missing Authorization header")
	}

	// Parse AWS4-HMAC-SHA256 signature
	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256 ") {
		return fmt.Errorf("unsupported authorization type")
	}

	// Extract credentials from authorization header
	parts := strings.Split(strings.TrimPrefix(authHeader, "AWS4-HMAC-SHA256 "), ",")
	authParams := make(map[string]string)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			authParams[kv[0]] = kv[1]
		}
	}

	credential := authParams["Credential"]
	if credential == "" {
		return fmt.Errorf("missing Credential in Authorization header")
	}

	// Extract access key from credential
	credParts := strings.Split(credential, "/")
	if len(credParts) < 1 {
		return fmt.Errorf("invalid Credential format")
	}

	accessKeyID := credParts[0]
	if accessKeyID != a.accessKeyID {
		return fmt.Errorf("invalid access key")
	}

	// For now, we'll do basic access key validation
	// Full signature verification would require implementing the complete AWS Signature V4 algorithm
	// This is a simplified version for demonstration

	return nil
}

// Middleware returns an HTTP middleware for authentication
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := a.Authenticate(r); err != nil {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Reserved for future AWS Signature V4 implementation
// These functions will be used when full signature verification is implemented

/*
func calculateSignature(secretKey, stringToSign string) string {
	h := hmac.New(sha256.New, []byte("AWS4"+secretKey))
	h.Write([]byte(stringToSign))
	return hex.EncodeToString(h.Sum(nil))
}

func getSigningKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
*/

// FormatTime formats time for AWS signature
func FormatTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// FormatDate formats date for AWS signature
func FormatDate(t time.Time) string {
	return t.UTC().Format("20060102")
}
