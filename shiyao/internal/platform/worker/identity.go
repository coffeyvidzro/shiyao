package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

var ErrInvalidCredential = errors.New("invalid worker credential")

// CredentialHash stores only a SHA-256 digest in durable state. The plaintext
// credential is generated/provisioned out of band and is never persisted.
func CredentialHash(credential string) string {
	sum := sha256.Sum256([]byte(credential))
	return hex.EncodeToString(sum[:])
}

func Authenticate(storedHash, credential string) error {
	if credential == "" || storedHash == "" || CredentialHash(credential) != storedHash {
		return ErrInvalidCredential
	}
	return nil
}
