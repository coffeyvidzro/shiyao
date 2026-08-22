package teamtoken

import (
	"strings"
	"testing"
	"time"
)

func TestValidateCreateRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateRequest
		wantErr bool
	}{
		{name: "valid request", req: CreateRequest{Name: "CI token"}},
		{name: "missing name", req: CreateRequest{}, wantErr: true},
		{name: "expired token", req: CreateRequest{Name: "expired", ExpiresAt: timePtr(time.Now().Add(-time.Minute))}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateCreateRequest(tt.req); (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCreateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
func TestGenerateToken(t *testing.T) {
	token, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken() error = %v", err)
	}
	if len(token) != len(tokenPrefix)+64 {
		t.Fatalf("token length = %d, want %d", len(token), len(tokenPrefix)+64)
	}
	prefix := tokenPrefixValue(token)
	if !strings.HasPrefix(prefix, tokenPrefix) || len(prefix) != prefixLen {
		t.Fatalf("token prefix = %q", prefix)
	}
}
func timePtr(value time.Time) *time.Time { return &value }
