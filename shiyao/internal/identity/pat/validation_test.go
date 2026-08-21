package pat

import (
	"testing"
	"time"
)

func TestValidateCreateRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateRequest
		wantErr bool
	}{
		{
			name:    "valid request",
			req:     CreateRequest{Name: "CLI token"},
		},
		{
			name:    "missing name",
			req:     CreateRequest{},
			wantErr: true,
		},
		{
			name:    "expired token",
			req: CreateRequest{
				Name:      "expired",
				ExpiresAt: timePtr(time.Now().Add(-time.Minute)),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateCreateRequest(tt.req); (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCreateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
