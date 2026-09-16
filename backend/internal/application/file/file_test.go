package file

import (
	"errors"
	"testing"
)

func TestValidateContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantErr     error
	}{
		{name: "plain text with charset", contentType: "text/plain; charset=utf-8"},
		{name: "case and whitespace", contentType: " IMAGE/PNG "},
		{name: "unsupported type", contentType: "application/octet-stream", wantErr: ErrUnsupportedFileType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContentType(tt.contentType)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validateContentType(%q) error = %v, want %v", tt.contentType, err, tt.wantErr)
			}
		})
	}
}
