package hatena

import (
	"crypto/sha1" //nolint:gosec
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestGenerateWSSEHeader_Format(t *testing.T) {
	header, err := GenerateWSSEHeader("user", "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{`Username="user"`, `PasswordDigest="`, `Nonce="`, `Created="`} {
		if !strings.Contains(header, want) {
			t.Errorf("header missing %q: %s", want, header)
		}
	}
}

func TestGenerateWSSE_KnownInput(t *testing.T) {
	nonce := []byte("fixed_nonce_1234")
	created := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	createdStr := created.Format(time.RFC3339)
	apiKey := "myapikey"

	// compute expected digest manually
	h := sha1.New() //nolint:gosec
	h.Write(nonce)
	h.Write([]byte(createdStr))
	h.Write([]byte(apiKey))
	expectedDigest := base64.StdEncoding.EncodeToString(h.Sum(nil))
	expectedNonce := base64.StdEncoding.EncodeToString(nonce)

	header, err := generateWSSE("user", apiKey, nonce, created)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(header, `PasswordDigest="`+expectedDigest+`"`) {
		t.Errorf("wrong digest in header: %s", header)
	}
	if !strings.Contains(header, `Nonce="`+expectedNonce+`"`) {
		t.Errorf("wrong nonce in header: %s", header)
	}
	if !strings.Contains(header, `Created="`+createdStr+`"`) {
		t.Errorf("wrong created in header: %s", header)
	}
}
