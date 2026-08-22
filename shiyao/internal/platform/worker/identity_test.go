package worker

import "testing"

func TestAuthenticateWorkerCredential(t *testing.T) {
	credential := "test-worker-secret"
	hash := CredentialHash(credential)
	if err := Authenticate(hash, credential); err != nil {
		t.Fatal(err)
	}
	if err := Authenticate(hash, "wrong-secret"); err != ErrInvalidCredential {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}
}
