// Package hatena provides a client for the Hatena Blog AtomPub API.
package hatena

import (
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // WSSE spec requires SHA-1; not used for security
	"encoding/base64"
	"fmt"
	"time"
)

// GenerateWSSEHeader returns a WSSE UsernameToken header value.
// The nonce is randomly generated for each call.
func GenerateWSSEHeader(username, apiKey string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	return generateWSSE(username, apiKey, nonce, time.Now().UTC())
}

// generateWSSE is the testable core: accepts explicit nonce and created time.
func generateWSSE(username, apiKey string, nonce []byte, created time.Time) (string, error) {
	createdStr := created.Format(time.RFC3339)
	nonceB64 := base64.StdEncoding.EncodeToString(nonce)

	h := sha1.New() //nolint:gosec
	h.Write(nonce)
	h.Write([]byte(createdStr))
	h.Write([]byte(apiKey))
	digest := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return fmt.Sprintf(
		`UsernameToken Username="%s", PasswordDigest="%s", Nonce="%s", Created="%s"`,
		username, digest, nonceB64, createdStr,
	), nil
}
