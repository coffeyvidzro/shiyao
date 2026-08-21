package authn

import (
	"errors"
	"testing"
)

func TestCredentialInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   CredentialInput
		wantErr bool
	}{
		{
			name:  "session",
			input: CredentialInput{Type: CredentialSession, Value: "session-token"},
		},
		{
			name:  "pat",
			input: CredentialInput{Type: CredentialPAT, Value: "pat-token"},
		},
		{
			name:    "missing type",
			input:   CredentialInput{Value: "token"},
			wantErr: true,
		},
		{
			name:    "missing value",
			input:   CredentialInput{Type: CredentialPAT},
			wantErr: true,
		},
		{
			name:    "unsupported type",
			input:   CredentialInput{Type: CredentialType("unknown"), Value: "token"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidCredential) {
					t.Fatalf("Validate() error = %v, want %v", err, ErrInvalidCredential)
				}
			return
			}

			if err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}
